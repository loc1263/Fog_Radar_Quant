"""Endpoints del módulo ETFs: ``/api/etf/*``.

Mismos endpoints que ``ETF_Choice/WEB/backend/main.py`` bajo el prefijo del
módulo. Único cambio de nombre: el ``GET /api/status`` del original es
``GET /api/etf/resumen`` (``/api/etf/status`` es el estado común de módulo).

Todas las lecturas toman el lock de ``data/etf/current`` (503 ``busy`` si el
pipeline lo tiene más de 15 s). ``POST /run`` re-analiza el CSV vigente (sin
Yahoo, segundos) y queda en el registro de jobs.
"""

from __future__ import annotations

from typing import Annotated

from fastapi import Depends
from fastapi.responses import Response

from fog_radar.core.errors import ApiError, NoDataError
from fog_radar.core.jobs import JobRegistry
from fog_radar.core.logging import get_logger
from fog_radar.core.ratelimit import limite_concurrente, rate_limit
from fog_radar.modules._base import make_module_router
from fog_radar.modules.etf import informe_pdf, pipeline, service, storage
from fog_radar.modules.etf.schemas import (
    EtfCap,
    EtfCategorias,
    EtfDiversificacion,
    EtfHistorial,
    EtfResumen,
    EtfRotacion,
    EtfRunResult,
    EtfTabla,
)
from fog_radar.settings import Settings, get_settings

log = get_logger("etf.api")

router = make_module_router("etf", "ETFs")

SettingsDep = Annotated[Settings, Depends(get_settings)]


@router.get(
    "/resumen", response_model=EtfResumen, summary="Resumen de la corrida vigente + parámetros del modelo"
)
def resumen(settings: SettingsDep) -> EtfResumen:
    return service.resumen(settings)


@router.get("/top15", response_model=EtfTabla, summary="Top 15 recomendados (ranking principal)")
def top15(settings: SettingsDep) -> EtfTabla:
    return service.tabla(settings, storage.TOP15)


@router.get("/top15-simple", response_model=EtfTabla, summary="Top 15 por rendimiento y Beta (20 % c/u)")
def top15_simple(settings: SettingsDep) -> EtfTabla:
    return service.tabla(settings, storage.TOP15_SIMPLE)


@router.get("/top15-vanguard", response_model=EtfTabla, summary="Top 15 del ranking principal, solo Vanguard")
def top15_vanguard(settings: SettingsDep) -> EtfTabla:
    return service.top15_vanguard(settings)


@router.get("/ranking-completo", response_model=EtfTabla, summary="Universo depurado completo")
def ranking_completo(settings: SettingsDep) -> EtfTabla:
    return service.tabla(settings, storage.RANKING)


@router.get("/excluidos", response_model=EtfTabla, summary="Excluidos por distorsión (|z MAD| > umbral)")
def excluidos(settings: SettingsDep) -> EtfTabla:
    return service.tabla(settings, storage.EXCLUIDOS)


@router.get("/diversificacion", response_model=EtfDiversificacion, summary="Categorías dentro del Top 15")
def diversificacion(settings: SettingsDep) -> EtfDiversificacion:
    return service.diversificacion(settings)


@router.get("/rotacion", response_model=EtfRotacion, summary="Rotación del Top 15 vs. la corrida anterior")
def rotacion(settings: SettingsDep) -> EtfRotacion:
    return service.rotacion_vs_anterior(settings)


@router.get("/momentum/top15", response_model=EtfTabla, summary="Top 15 Momentum / crecimiento")
def momentum_top15(settings: SettingsDep) -> EtfTabla:
    return service.tabla(
        settings, storage.MOMENTUM_TOP15, "El análisis de Momentum no se corrió en esta corrida."
    )


@router.get("/categorias/ranking", response_model=EtfCategorias, summary="Ranking por categoría")
def categorias_ranking(settings: SettingsDep) -> EtfCategorias:
    return service.categorias(settings)


@router.get("/russell3000", response_model=EtfCap, summary="Top 15 Cap. (momentum factor tipo VFMO)")
def russell3000(settings: SettingsDep) -> EtfCap:
    return service.top15_cap(settings)


@router.get("/historial", response_model=EtfHistorial, summary="Historial de corridas")
def historial(settings: SettingsDep) -> EtfHistorial:
    return service.historial(settings)


@router.get(
    "/pdf",
    summary="Informe ejecutivo en PDF de la corrida vigente",
    response_class=Response,
    responses={200: {"content": {"application/pdf": {}}, "description": "PDF"}},
    dependencies=[
        Depends(rate_limit("10/minute", scope="etf-pdf")),
        Depends(limite_concurrente(2, scope="etf-pdf")),
    ],
)
def descargar_pdf(settings: SettingsDep) -> Response:
    try:
        with storage.bloqueo_datos(settings):
            clave = _firma_datos(settings)
            hit = _pdf_cache.get("pdf")
            if hit and hit[0] == clave:
                pdf_bytes, fecha = hit[1], hit[2]
            else:
                pdf_bytes, fecha = informe_pdf.generar_pdf(
                    storage.current_dir(settings), storage.hist_dir(settings)
                )
                _pdf_cache["pdf"] = (clave, pdf_bytes, fecha)
    except FileNotFoundError as exc:
        raise NoDataError(str(exc)) from exc
    nombre = f"etf_informe_{fecha.replace('/', '')}.pdf"
    return Response(
        content=pdf_bytes,
        media_type="application/pdf",
        headers={"Content-Disposition": f'attachment; filename="{nombre}"'},
    )


# El PDF solo cambia cuando cambian los CSV: se cachea por la firma (nombre, tamaño,
# mtime) de los archivos de current/ y hist/.
_pdf_cache: dict[str, tuple[tuple[tuple[str, int, int], ...], bytes, str]] = {}


def _firma_datos(settings: Settings) -> tuple[tuple[str, int, int], ...]:
    from datetime import date

    firma = [(f"hoy/{date.today().isoformat()}", 0, 0)]  # el PDF lleva la fecha de emisión
    for carpeta in (storage.current_dir(settings), storage.hist_dir(settings)):
        if carpeta.is_dir():
            for f in sorted(carpeta.iterdir()):
                if f.is_file():
                    st = f.stat()
                    firma.append((f"{carpeta.name}/{f.name}", st.st_size, st.st_mtime_ns))
    return tuple(firma)


@router.post(
    "/run",
    response_model=EtfRunResult,
    summary="Re-analiza el CSV vigente (no descarga de Yahoo)",
    dependencies=[Depends(rate_limit("3/minute", scope="etf-run"))],
)
def run(settings: SettingsDep) -> EtfRunResult:
    from fog_radar.jobs.runner import OcupadoError, lock_pipelines

    registry = JobRegistry(settings.jobs_dir)
    if registry.is_running("etf"):
        raise ApiError(409, "run_in_progress", "Ya hay una corrida de ETFs en curso.")
    try:
        # Mismo lock global que las corridas batch: un re-análisis no puede pisar
        # (ni ser pisado por) una corrida del cron o de la consola.
        with lock_pipelines(settings, esperar_s=0), registry.track("etf", trigger="api") as job:
            job.message = pipeline.run(settings, solo_analisis=True)
    except OcupadoError as exc:
        raise ApiError(409, "run_in_progress", "Ya hay una corrida de pipelines en curso.") from exc
    except ApiError:
        raise
    except Exception as exc:
        log.exception("Error re-analizando ETFs vía POST /run")
        raise ApiError(
            500, "pipeline_error", "Error procesando el análisis; revisa el log del servidor."
        ) from exc
    return EtfRunResult(job=job)
