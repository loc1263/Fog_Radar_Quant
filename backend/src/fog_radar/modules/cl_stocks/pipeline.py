"""Pipeline batch del módulo Chile: extractor (Yahoo) → scoring → CSV/XLSX.

    uv run python -m fog_radar.jobs cl_stocks                 # completo (~10 min)
    uv run python -m fog_radar.jobs cl_stocks --solo-scoring  # re-puntúa el último CSV

Nunca se ejecuta dentro de un request web.
"""

from __future__ import annotations

from datetime import datetime

from fog_radar.core.logging import get_logger
from fog_radar.modules.cl_stocks import storage
from fog_radar.modules.cl_stocks.extractor import cargar_tickers, extraer_indicadores
from fog_radar.modules.cl_stocks.scoring import construir_ranking, grupos_duplicados
from fog_radar.settings import Settings

log = get_logger("cl.pipeline")


def run(settings: Settings, *, solo_scoring: bool = False) -> str:
    """Ejecuta el pipeline y devuelve un resumen para el registro de jobs."""
    directory = storage.data_dir(settings)
    timestamp = datetime.now(settings.tzinfo).strftime("%Y%m%d_%H%M%S")
    resumen: list[str] = []

    if solo_scoring:
        fuente = storage.cargar_indicadores(settings)
        indicadores = fuente.df
        # El ranking conserva el timestamp de sus datos: re-puntuar no los hace "más nuevos".
        timestamp = fuente.path.stem.removeprefix("indicadores_acciones_")
        resumen.append(f"solo scoring de {fuente.path.name}")
    else:
        tickers = cargar_tickers()
        indicadores, errores = extraer_indicadores(tickers)
        if indicadores.empty:
            raise RuntimeError("Yahoo Finance no devolvió datos para ningún ticker.")
        ruta = storage.guardar(indicadores, directory, "indicadores_acciones", timestamp)
        log.info("Indicadores: %s", ruta.name)
        resumen.append(f"{len(indicadores)}/{len(tickers)} tickers")
        if errores:
            resumen.append(f"sin datos: {', '.join(errores)}")

    ranking = construir_ranking(indicadores)
    ruta = storage.guardar(ranking, directory, "ranking_acciones", timestamp)
    log.info("Ranking: %s", ruta.name)

    top = ranking.head(3)["Símbolo"].tolist()
    resumen.append(f"top 3: {', '.join(top)}")
    for grupo, simbolos in grupos_duplicados(ranking).items():
        log.warning("Grupo financiero duplicado %d: %s (riesgo de doble conteo)", grupo, ", ".join(simbolos))
    return " · ".join(resumen)
