"""Endpoints transversales: hub con datos reales y buscador global de ticker."""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends, Query

from fog_radar.core.ratelimit import rate_limit
from fog_radar.core.schemas import BusquedaResponse, HubResponse
from fog_radar.settings import Settings, get_settings
from fog_radar.transversal import servicio

SettingsDep = Annotated[Settings, Depends(get_settings)]

router = APIRouter(tags=["Transversal"])


@router.get("/hub", response_model=HubResponse, summary="Estado, frescura y top 3 de cada módulo")
def get_hub(settings: SettingsDep) -> HubResponse:
    return servicio.hub(settings)


@router.get(
    "/buscar",
    response_model=BusquedaResponse,
    summary="Buscador global: en qué módulos aparece un ticker y con qué puntaje",
    dependencies=[Depends(rate_limit("120/minute", scope="buscar"))],
)
def get_buscar(
    settings: SettingsDep,
    q: Annotated[str, Query(min_length=1, max_length=40, description="Ticker o parte del nombre")],
    limite: Annotated[int, Query(ge=1, le=servicio.MAX_RESULTADOS)] = 20,
) -> BusquedaResponse:
    return servicio.buscar(settings, q, limite)
