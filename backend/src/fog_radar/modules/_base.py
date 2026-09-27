"""Fábrica del router base de cada módulo.

Cada módulo arranca con ``GET /api/<prefijo>/status``; sus endpoints propios
se agregan al portar el proyecto original (Fases 3-6).
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends

from fog_radar.core.schemas import ModuleStatus
from fog_radar.core.status import build_module_status
from fog_radar.modules import MODULES_BY_KEY
from fog_radar.settings import ModuleKey, Settings, get_settings


def make_module_router(key: ModuleKey, tag: str) -> APIRouter:
    info = MODULES_BY_KEY[key]
    router = APIRouter(prefix=info.prefix, tags=[tag])

    @router.get("/status", response_model=ModuleStatus, summary=f"Estado del módulo {info.title}")
    def module_status(settings: Annotated[Settings, Depends(get_settings)]) -> ModuleStatus:
        return build_module_status(info, settings)

    return router
