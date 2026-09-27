"""Estado de un módulo (datos presentes + último job), usado por /status."""

from __future__ import annotations

from fog_radar.core.jobs import JobRegistry
from fog_radar.core.schemas import ModuleStatus
from fog_radar.jobs import PIPELINES
from fog_radar.modules import ModuleInfo
from fog_radar.settings import Settings


def build_module_status(info: ModuleInfo, settings: Settings) -> ModuleStatus:
    data_dir = settings.module_data_dir(info.key)
    has_data = data_dir.is_dir() and any(p for p in data_dir.iterdir() if not p.name.startswith("."))
    registry = JobRegistry(settings.jobs_dir)
    return ModuleStatus(
        key=info.key,
        title=info.title,
        api_prefix=f"/api{info.prefix}",
        source_project=info.source_project,
        migrated=info.key in PIPELINES,
        migration_phase=info.migration_phase,
        data_available=has_data,
        latest_job=registry.latest(info.key),
        last_success=registry.last_success(info.key),
    )
