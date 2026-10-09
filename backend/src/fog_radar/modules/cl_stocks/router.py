"""Endpoints del módulo Chile (IPSA): ``/api/cl/*``.

Solo lectura: sirven el último ranking / indicadores generados por el
pipeline (``python -m fog_radar.jobs cl_stocks``) y el historial de
corridas (``/history/*``, sobre ``history.db``). El recálculo con pesos
personalizados ocurre en el navegador (``$lib/modules/cl/scoring.ts``).
"""

from __future__ import annotations

from typing import Annotated, Any

from fastapi import Depends, Query
from fastapi import Path as PathParam

from fog_radar.core.errors import ApiError
from fog_radar.core.schemas import HistDiff
from fog_radar.modules._base import make_module_router
from fog_radar.modules.cl_stocks import history, service
from fog_radar.modules.cl_stocks.schemas import (
    ClHistCorrida,
    ClHistIndicadorPunto,
    ClHistTickerPunto,
    IndicadoresResponse,
    RankingResponse,
)
from fog_radar.settings import Settings, get_settings

router = make_module_router("cl_stocks", "Chile (IPSA)")

SettingsDep = Annotated[Settings, Depends(get_settings)]


@router.get("/ranking", response_model=RankingResponse, summary="Último ranking multi-factor del IPSA")
def get_ranking(settings: SettingsDep) -> RankingResponse:
    return service.ranking(settings)


@router.get(
    "/indicadores",
    response_model=IndicadoresResponse,
    summary="Indicadores financieros crudos (antes del scoring)",
)
def get_indicadores(settings: SettingsDep) -> IndicadoresResponse:
    return service.indicadores(settings)


# --- Historial ---------------------------------------------------------------------

Ticker = Annotated[str, PathParam(pattern=r"^[A-Za-z0-9.\-]{1,15}$")]
Ts = Annotated[str | None, Query(pattern=r"^\d{8}_\d{6}$", description="YYYYMMDD_HHMMSS")]


@router.get("/history/runs", response_model=list[ClHistCorrida], summary="Corridas guardadas en el historial")
def history_runs(settings: SettingsDep) -> Any:
    return history.corridas(settings)


@router.get(
    "/history/ticker/{ticker}", response_model=list[ClHistTickerPunto], summary="Evolución de una acción"
)
def history_ticker(ticker: Ticker, settings: SettingsDep) -> Any:
    return service.historia_ticker(settings, ticker.upper())


@router.get(
    "/history/ticker/{ticker}/indicador/{metrica}",
    response_model=list[ClHistIndicadorPunto],
    summary="Evolución de un indicador crudo de una acción",
)
def history_indicador(ticker: Ticker, metrica: str, settings: SettingsDep) -> Any:
    if metrica not in history.METRICAS_NUMERICAS:
        raise ApiError(404, "not_found", f"Métrica desconocida: '{metrica}'.")
    return history.historia_indicador(settings, ticker.upper(), metrica)


@router.get("/history/diff", response_model=HistDiff, summary="Cambios de posición entre dos corridas")
def history_diff(settings: SettingsDep, desde: Ts = None, hasta: Ts = None) -> Any:
    return history.diff(settings, desde, hasta)
