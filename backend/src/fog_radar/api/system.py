"""Endpoints de sistema: salud (público), estado y jobs (protegidos)."""

from __future__ import annotations

from importlib.metadata import PackageNotFoundError, version
from typing import Annotated

from fastapi import APIRouter, Depends, Query

from fog_radar.core.errors import ApiError
from fog_radar.core.jobs import JobRecord, JobRegistry
from fog_radar.core.schemas import HealthResponse, SystemStatus
from fog_radar.core.status import build_module_status
from fog_radar.modules import MODULES, MODULES_BY_KEY
from fog_radar.settings import Settings, get_settings

try:
    APP_VERSION = version("fog-radar")
except PackageNotFoundError:  # pragma: no cover - ejecutando sin instalar
    APP_VERSION = "0.0.0"

SettingsDep = Annotated[Settings, Depends(get_settings)]

public_router = APIRouter(tags=["Sistema"])
protected_router = APIRouter(tags=["Sistema"])


@public_router.get("/health", response_model=HealthResponse, summary="Salud del servicio (público)")
def health(settings: SettingsDep) -> HealthResponse:
    return HealthResponse(app=settings.app_name, version=APP_VERSION, env=settings.env)


@protected_router.get("/status", response_model=SystemStatus, summary="Estado de los 4 módulos")
def system_status(settings: SettingsDep) -> SystemStatus:
    return SystemStatus(
        app=settings.app_name,
        version=APP_VERSION,
        env=settings.env,
        modules=[build_module_status(m, settings) for m in MODULES],
    )


@protected_router.get(
    "/jobs", response_model=dict[str, JobRecord | None], summary="Último job de cada módulo"
)
def jobs_latest(settings: SettingsDep) -> dict[str, JobRecord | None]:
    registry = JobRegistry(settings.jobs_dir)
    return {m.key: registry.latest(m.key) for m in MODULES}


@protected_router.get(
    "/jobs/{module}", response_model=list[JobRecord], summary="Historial de jobs de un módulo"
)
def jobs_history(
    module: str,
    settings: SettingsDep,
    limit: Annotated[int, Query(ge=1, le=200)] = 20,
) -> list[JobRecord]:
    if module not in MODULES_BY_KEY:
        raise ApiError(404, "unknown_module", f"Módulo desconocido: {module}")
    return JobRegistry(settings.jobs_dir).history(module, limit=limit)
