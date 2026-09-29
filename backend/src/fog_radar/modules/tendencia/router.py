"""Endpoints del módulo Tendencia: ``/api/tendencia/*``.

- ``/latest`` y ``/runs/{run_dir}/dashboard``: una corrida completa (reemplaza el
  ``dashboard_data.json`` estático que el original copiaba a ``frontend/public``).
- ``/runs/{run_dir}/files[/{nombre}]``: PDF y CSV de una corrida (reemplaza el
  servidor estático en :8000 sobre ``backend/output``).
- ``/analyses``, ``/runs``, ``/runs/{run_dir}``, ``/history/*``: los endpoints de
  ``api/app.py`` del original sobre ``history.db`` (solo lectura).
"""

from __future__ import annotations

from typing import Annotated, Any

from fastapi import Depends, Query
from fastapi import Path as PathParam
from fastapi.responses import FileResponse

from fog_radar.modules._base import make_module_router
from fog_radar.modules.tendencia import run_files as storage
from fog_radar.modules.tendencia import service
from fog_radar.modules.tendencia.schemas import (
    TdAnalisisDisponible,
    TdArchivo,
    TdDashboard,
    TdExcluido,
    TdMomentumLider,
    TdRunDetalle,
    TdRunPerformance,
    TdSectorHistoria,
    TdTickerHistoria,
    TdTickerNombre,
)
from fog_radar.settings import Settings, get_settings

router = make_module_router("tendencia", "Tendencia")

SettingsDep = Annotated[Settings, Depends(get_settings)]
RunDir = Annotated[str, PathParam(pattern=storage.RUN_DIR_PATTERN, description="YYYYMMDD_NN")]
Analysis = Annotated[
    str,
    Query(pattern=r"^(_[a-z]+)?$", description="'' = Asia + EEUU (principal), '_us' = solo EEUU"),
]
DateParam = Annotated[str | None, Query(pattern=r"^\d{4}-\d{2}-\d{2}$")]


@router.get("/latest", response_model=TdDashboard, summary="Última corrida completa")
def latest(settings: SettingsDep) -> TdDashboard:
    return service.dashboard(settings)


@router.get("/runs/{run_dir}/dashboard", response_model=TdDashboard, summary="Una corrida completa")
def run_dashboard(run_dir: RunDir, settings: SettingsDep) -> TdDashboard:
    return service.dashboard(settings, run_dir)


@router.get("/runs/{run_dir}/files", response_model=list[TdArchivo], summary="Archivos de una corrida")
def run_files(run_dir: RunDir, settings: SettingsDep) -> Any:
    return storage.archivos(storage.run_dir(settings, run_dir))


@router.get(
    "/runs/{run_dir}/files/{nombre}",
    summary="Descarga un archivo de una corrida (PDF, CSV, JSON, TXT)",
    response_class=FileResponse,
    responses={200: {"content": {"application/pdf": {}, "text/csv": {}}}},
)
def run_file(run_dir: RunDir, nombre: str, settings: SettingsDep) -> FileResponse:
    path = storage.archivo(storage.run_dir(settings, run_dir), nombre)
    tipo = storage.TIPOS_DESCARGABLES[path.suffix.lower()]
    return FileResponse(path, media_type=tipo, filename=path.name)


@router.get(
    "/excluidos",
    response_model=list[TdExcluido],
    summary="Tickers excluidos en el último armado del dataset (con motivo)",
)
def excluidos(settings: SettingsDep) -> Any:
    return storage.excluidos(settings)


@router.get("/analyses", response_model=list[TdAnalisisDisponible], summary="Análisis vistos en el historial")
def list_analyses(settings: SettingsDep) -> Any:
    return service.analyses(settings)


@router.get("/runs", response_model=list[TdRunPerformance], summary="Performance por corrida (ascendente)")
def list_runs(
    settings: SettingsDep,
    analysis: Analysis = "",
    date_from: Annotated[DateParam, Query(alias="from")] = None,
    date_to: Annotated[DateParam, Query(alias="to")] = None,
    limit: Annotated[int, Query(ge=1, le=5000)] = 500,
) -> Any:
    return service.runs(settings, analysis, date_from, date_to, limit)


@router.get("/runs/{run_dir}", response_model=TdRunDetalle, summary="Corrida reconstruida desde history.db")
def get_run(run_dir: RunDir, settings: SettingsDep) -> Any:
    return service.run_detalle(settings, run_dir)


@router.get("/history/performance", response_model=list[TdRunPerformance], summary="Serie de performance")
def history_performance(
    settings: SettingsDep,
    analysis: Analysis = "",
    date_from: Annotated[DateParam, Query(alias="from")] = None,
    date_to: Annotated[DateParam, Query(alias="to")] = None,
) -> Any:
    return service.runs(settings, analysis, date_from, date_to, 5000)


@router.get(
    "/history/tickers", response_model=list[TdTickerNombre], summary="Tickers seleccionados alguna vez"
)
def history_tickers(settings: SettingsDep, analysis: Analysis = "") -> Any:
    return service.tickers(settings, analysis)


@router.get(
    "/history/ticker/{ticker}", response_model=list[TdTickerHistoria], summary="Trayectoria de un ticker"
)
def history_ticker(
    ticker: Annotated[str, PathParam(pattern=r"^[A-Za-z0-9.\-^=]{1,15}$")],
    settings: SettingsDep,
    analysis: Analysis = "",
) -> Any:
    return service.ticker_history(settings, ticker, analysis)


@router.get("/history/sectors", response_model=list[TdSectorHistoria], summary="Peso por sector en el tiempo")
def history_sectors(settings: SettingsDep, analysis: Analysis = "") -> Any:
    return service.sector_history(settings, analysis)


@router.get(
    "/history/momentum-leaders",
    response_model=list[TdMomentumLider],
    summary="Líderes de momentum por corrida",
)
def history_momentum_leaders(
    settings: SettingsDep, analysis: Analysis = "", top: Annotated[int, Query(ge=1, le=20)] = 5
) -> Any:
    return service.momentum_leaders(settings, analysis, top)
