"""Modelos de respuesta compartidos."""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict

from fog_radar.core.jobs import JobRecord


class Out(BaseModel):
    """Base de las respuestas de los módulos: los campos con valor por defecto se
    publican como requeridos en el OpenAPI (siempre vienen en la respuesta), así los
    tipos TS generados no los marcan como opcionales (ADR 0002)."""

    model_config = ConfigDict(json_schema_serialization_defaults_required=True)


class HealthResponse(BaseModel):
    status: Literal["ok"] = "ok"
    app: str
    version: str
    env: str


class ModuleStatus(BaseModel):
    key: str
    title: str
    api_prefix: str
    source_project: str
    migrated: bool
    """True cuando el pipeline del módulo ya está portado a Fog Radar."""
    migration_phase: int
    data_available: bool
    latest_job: JobRecord | None = None
    last_success: JobRecord | None = None


class SystemStatus(BaseModel):
    app: str
    version: str
    env: str
    modules: list[ModuleStatus]
