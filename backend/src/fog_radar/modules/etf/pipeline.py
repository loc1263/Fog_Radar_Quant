"""Pipeline batch del módulo ETF (port de ``actualizar_datos_semanal.py``).

    uv run python -m fog_radar.jobs etf                   # screener Yahoo + análisis (~30 min)
    uv run python -m fog_radar.jobs etf --solo-analisis   # re-analiza el CSV vigente (segundos)
    uv run python -m fog_radar.jobs etf --backfill        # solo reconstruye history.db

Flujo completo (igual que el original):

1. Screener → ``data/etf/screener/etf_perf_data_<ts>.csv`` (Yahoo Finance).
2. Análisis (ranking, momentum, categoría, top 15 cap.) en una carpeta de
   *staging*; ``data/etf/current`` no se toca todavía.
3. Solo si el análisis terminó sin errores: archiva la corrida vigente en
   ``data/etf/hist`` e instala la nueva (bajo el lock que respetan los GET).
4. Guarda la corrida en ``data/etf/history.db`` (índice histórico; si falla, solo
   avisa: se recupera con ``--backfill``).
5. Mueve a ``screener/data_old/`` las salidas crudas del screener de más de
   30 días (no borra nada).

``--solo-analisis`` equivale al ``POST /api/run`` del original: recalcula los
outputs sobre el CSV vigente sin archivar ni llamar a Yahoo. También lo usa
``POST /api/etf/run``.
"""

from __future__ import annotations

import re
import shutil
from datetime import datetime
from pathlib import Path

from fog_radar.core import histdb
from fog_radar.core.errors import NoDataError
from fog_radar.core.logging import get_logger
from fog_radar.modules.etf import history, storage
from fog_radar.settings import Settings

log = get_logger("etf.pipeline")

ARCHIVO_DIAS_RESPALDO = 30
PATRONES_RESPALDO_SCREENER = (
    "etf_perf_data_*.csv",
    "etf_perf_data_*.xlsx",
    "etf_choice_*.csv",
    "etf_choice_*.xlsx",
    "List_All_ETF_valid_*.csv",
    "incidencias_*.csv",
)
_RE_TIMESTAMP_CORRIDA = re.compile(r"(\d{8}_\d{6})")


def correr_screener(settings: Settings) -> Path:
    """Corre el screener (in-process) y devuelve el ``etf_perf_data_*.csv`` generado."""
    from fog_radar.modules.etf.screener.main import procesar_etfs  # import diferido: yfinance

    log.info("Corriendo el screener de ETFs (puede tardar ~30 min por el rate limit de Yahoo)...")
    csv = procesar_etfs(storage.screener_dir(settings))
    if csv is None or not csv.is_file():
        raise RuntimeError("El screener terminó sin generar un etf_perf_data_*.csv (ningún ETF validado).")
    return csv


def fecha_tag_de(origen_csv: Path, settings: Settings) -> str:
    m = re.search(r"(\d{8})", origen_csv.name)
    return m.group(1) if m else datetime.now(settings.tzinfo).strftime("%Y%m%d")


def respaldar_outputs_viejos_screener(directory: Path, dias: int = ARCHIVO_DIAS_RESPALDO) -> list[Path]:
    """Mueve (no borra) a ``data_old/`` las salidas crudas del screener cuyo timestamp
    ``AAAAMMDD_HHMMSS`` en el nombre tenga más de ``dias`` días. Una falla puntual se
    registra y no hace fallar la corrida."""
    if dias <= 0 or not directory.is_dir():
        return []
    data_old = directory / "data_old"
    data_old.mkdir(parents=True, exist_ok=True)
    ahora = datetime.now()
    movidos: list[Path] = []
    for patron in PATRONES_RESPALDO_SCREENER:
        for ruta in directory.glob(patron):
            m = _RE_TIMESTAMP_CORRIDA.search(ruta.name)
            if not m:
                continue
            try:
                fecha = datetime.strptime(m.group(1), "%Y%m%d_%H%M%S")
            except ValueError:
                continue
            if (ahora - fecha).days <= dias:
                continue
            try:
                destino = data_old / ruta.name
                shutil.move(ruta, destino)
                movidos.append(destino)
            except OSError as exc:
                log.warning("No se pudo respaldar %s: %s", ruta.name, exc)
    return movidos


def _analizar_e_instalar(settings: Settings, origen_csv: Path, *, archivar: bool) -> str:
    fecha_tag = fecha_tag_de(origen_csv, settings)
    staging = storage.preparar_staging(settings, origen_csv, fecha_tag)
    try:
        log.info("Corriendo el análisis en staging (%s)...", staging.name)
        resultado = storage.correr_analisis(staging, storage.hist_dir(settings))
        instalados = storage.instalar_desde_staging(settings, staging, archivar=archivar)
        log.info("Instalados en current/: %s", ", ".join(instalados))
    finally:
        shutil.rmtree(staging, ignore_errors=True)
    histdb.guardar_sin_fallar("etf", lambda: history.guardar_vigente(settings))

    top3 = resultado["top15"]["Ticker"].head(3).tolist()
    partes = [
        f"corrida {fecha_tag}",
        f"{resultado['n_universo_depurado']}/{resultado['n_universo_bruto']} ETFs depurados",
        f"{resultado['n_outliers']} outliers",
        f"top 3: {', '.join(top3)}",
    ]
    if resultado["avisos"]:
        partes.append(f"sin {', '.join(resultado['avisos'])}")
    return " · ".join(partes)


def run(settings: Settings, *, solo_analisis: bool = False, backfill: bool = False) -> str:
    """Ejecuta el pipeline y devuelve un resumen para el registro de jobs."""
    if backfill:
        return history.backfill(settings)
    if solo_analisis:
        with storage.bloqueo_datos(settings, timeout=60):
            vigente = storage.csv_actual(storage.current_dir(settings))
        if vigente is None:
            raise NoDataError(storage.SIN_CORRIDA)
        return "solo análisis · " + _analizar_e_instalar(settings, vigente, archivar=False)

    csv_generado = correr_screener(settings)
    log.info("Screener generó: %s", csv_generado.name)
    resumen = _analizar_e_instalar(settings, csv_generado, archivar=True)
    movidos = respaldar_outputs_viejos_screener(storage.screener_dir(settings))
    if movidos:
        log.info("Respaldados %d archivos viejos del screener en data_old/", len(movidos))
    return resumen
