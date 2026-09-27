"""Endpoints del módulo Chile (IPSA): ``/api/cl/*``.

Solo lectura: sirven el último ranking / indicadores generados por el
pipeline (``python -m fog_radar.jobs cl_stocks``). El recálculo con pesos
personalizados ocurre en el navegador (``$lib/modules/cl/scoring.ts``).
"""

from __future__ import annotations

from typing import Annotated

from fastapi import Depends

from fog_radar.modules._base import make_module_router
from fog_radar.modules.cl_stocks import service
from fog_radar.modules.cl_stocks.schemas import IndicadoresResponse, RankingResponse
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
