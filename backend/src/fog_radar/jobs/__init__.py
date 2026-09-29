"""Pipelines batch de cada módulo. Punto de entrada: ``python -m fog_radar.jobs``."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field


@dataclass(frozen=True)
class PipelineOption:
    """Bandera booleana del CLI (``--solo-scoring`` → kwarg ``solo_scoring=True``)."""

    flag: str
    help: str

    @property
    def dest(self) -> str:
        return self.flag.lstrip("-").replace("-", "_")


@dataclass(frozen=True)
class Pipeline:
    run: Callable[..., str | None]
    """``run(settings, **opciones)``; devuelve un resumen para el registro de jobs."""
    options: tuple[PipelineOption, ...] = field(default_factory=tuple)


def _cl_stocks(settings, **opts) -> str | None:  # noqa: ANN001 - import diferido
    from fog_radar.modules.cl_stocks.pipeline import run

    return run(settings, **opts)


def _us_stocks(settings, **opts) -> str | None:  # noqa: ANN001 - import diferido
    from fog_radar.modules.us_stocks.pipeline import run

    return run(settings, **opts)


def _etf(settings, **opts) -> str | None:  # noqa: ANN001 - import diferido
    from fog_radar.modules.etf.pipeline import run

    return run(settings, **opts)


def _tendencia(settings, **opts) -> str | None:  # noqa: ANN001 - import diferido
    from fog_radar.modules.tendencia.pipeline import run

    return run(settings, **opts)


# modulo -> pipeline. Se completa al portar cada módulo (Fases 3-6).
PIPELINES: dict[str, Pipeline] = {
    "cl_stocks": Pipeline(
        _cl_stocks,
        (PipelineOption("--solo-scoring", "Re-puntúa el último CSV de indicadores sin descargar de Yahoo."),),
    ),
    "us_stocks": Pipeline(_us_stocks),
    "etf": Pipeline(
        _etf,
        (PipelineOption("--solo-analisis", "Re-analiza el CSV vigente sin descargar de Yahoo."),),
    ),
    "tendencia": Pipeline(
        _tendencia,
        (
            PipelineOption("--skip-fetch", "Reutiliza los precios/FX ya descargados (sin Yahoo)."),
            PipelineOption("--force-refresh", "Ignora la caché de precios y vuelve a descargar todo."),
            PipelineOption("--force-discovery", "Ignora el snapshot de universo de hoy."),
            PipelineOption("--static-universe", "Usa la lista fija de respaldo en vez del screener."),
            PipelineOption("--macro", "Descarga antes los datos macro de FRED (requiere FRED_API_KEY)."),
            PipelineOption("--backfill", "Solo carga en history.db las corridas de output/ que falten."),
        ),
    ),
}
