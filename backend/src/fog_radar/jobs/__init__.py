"""Pipelines batch de cada módulo. Punto de entrada: ``python -m fog_radar.jobs``."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field


@dataclass(frozen=True)
class PipelineOption:
    """Opción del CLI de un pipeline.

    Sin ``tipo`` es una bandera (``--solo-scoring`` → kwarg ``solo_scoring=True``);
    con ``tipo`` lleva valor (``--top-n 20`` → kwarg ``top_n=20``), validado
    contra ``minimo``/``maximo`` (TD-CFG-1).
    """

    flag: str
    help: str
    tipo: type[int] | type[float] | None = None
    minimo: float | None = None
    maximo: float | None = None
    kwarg: str | None = None
    """Nombre del kwarg de ``run()`` si difiere del de la bandera."""

    @property
    def dest(self) -> str:
        return self.flag.lstrip("-").replace("-", "_")

    @property
    def destino(self) -> str:
        return self.kwarg or self.dest

    @property
    def con_valor(self) -> bool:
        return self.tipo is not None

    def convertir(self, texto: str) -> int | float:
        """Tipo para argparse: convierte y valida el rango (``ArgumentTypeError`` si no)."""
        import argparse

        assert self.tipo is not None
        try:
            valor = self.tipo(texto)
        except ValueError as exc:
            raise argparse.ArgumentTypeError(f"{self.flag}: '{texto}' no es un {self.tipo.__name__}") from exc
        if (self.minimo is not None and valor < self.minimo) or (
            self.maximo is not None and valor > self.maximo
        ):
            raise argparse.ArgumentTypeError(
                f"{self.flag}: {valor} fuera de rango [{self.minimo}, {self.maximo}]"
            )
        return valor


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


# Todos los módulos: reconstruye history.db desde los archivos de corridas (core.histdb).
_BACKFILL = PipelineOption("--backfill", "Solo carga en history.db las corridas ya guardadas en disco.")

# modulo -> pipeline. Se completa al portar cada módulo (Fases 3-6).
PIPELINES: dict[str, Pipeline] = {
    "cl_stocks": Pipeline(
        _cl_stocks,
        (
            PipelineOption(
                "--solo-scoring", "Re-puntúa el último CSV de indicadores sin descargar de Yahoo."
            ),
            _BACKFILL,
        ),
    ),
    "us_stocks": Pipeline(_us_stocks, (_BACKFILL,)),
    "etf": Pipeline(
        _etf,
        (PipelineOption("--solo-analisis", "Re-analiza el CSV vigente sin descargar de Yahoo."), _BACKFILL),
    ),
    "tendencia": Pipeline(
        _tendencia,
        (
            PipelineOption("--skip-fetch", "Reutiliza los precios/FX ya descargados (sin Yahoo)."),
            PipelineOption("--force-refresh", "Ignora la caché de precios y vuelve a descargar todo."),
            PipelineOption("--force-discovery", "Ignora el snapshot de universo de hoy."),
            PipelineOption("--static-universe", "Usa la lista fija de respaldo en vez del screener."),
            PipelineOption("--skip-discovery", "Reutiliza el último snapshot de universo (sin screener)."),
            PipelineOption("--macro", "Descarga antes los datos macro de FRED (requiere FRED_API_KEY)."),
            _BACKFILL,
            PipelineOption(
                "--top-n",
                "Acciones a seleccionar en Asia + EEUU (por defecto 15).",
                tipo=int,
                minimo=2,
                maximo=60,
            ),
            PipelineOption(
                "--top-n-us",
                "Acciones a seleccionar en el análisis solo EEUU (por defecto 15).",
                tipo=int,
                minimo=2,
                maximo=60,
            ),
            PipelineOption(
                "--max-weight",
                "Tope de peso por activo, fracción en (0, 1] (por defecto 0.15).",
                tipo=float,
                minimo=0.01,
                maximo=1.0,
            ),
        ),
    ),
}
