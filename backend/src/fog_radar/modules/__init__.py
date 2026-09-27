"""Registro de módulos del sitio. Cada módulo = un proyecto original migrado."""

from __future__ import annotations

from dataclasses import dataclass

from fog_radar.settings import ModuleKey


@dataclass(frozen=True)
class ModuleInfo:
    key: ModuleKey
    prefix: str  # prefijo bajo /api
    title: str
    source_project: str
    migration_phase: int


MODULES: tuple[ModuleInfo, ...] = (
    ModuleInfo("cl_stocks", "/cl", "Acciones Chile (IPSA)", "Stock_Choice_Nacional", 3),
    ModuleInfo("us_stocks", "/us", "Acciones EEUU", "Stock_Choice_EEUU", 4),
    ModuleInfo("etf", "/etf", "ETFs", "ETF_Choice", 5),
    ModuleInfo("tendencia", "/tendencia", "Tendencia Asia + EEUU", "Stock_Tendencia", 6),
)

MODULES_BY_KEY: dict[str, ModuleInfo] = {m.key: m for m in MODULES}
