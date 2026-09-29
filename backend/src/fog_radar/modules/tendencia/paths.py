"""Carpeta raíz de datos del módulo Tendencia (``data/tendencia``).

Vive aparte de ``config`` a propósito: el pipeline hace
``importlib.reload(config)`` tras descubrir el universo (patrón del
original) y la raíz elegida no debe perderse con ese reload. Por defecto
es ``Settings.module_data_dir("tendencia")``; los tests y el pipeline la
fijan con :func:`set_root`.
"""

from __future__ import annotations

from pathlib import Path

_root: Path | None = None

# Fog Radar (TD-CFG-1): parámetros del modelo que se sobrescriben para UNA
# corrida desde el CLI de jobs (``--top-n``, ``--max-weight``…). Viven acá por
# el mismo motivo que la raíz: ``config`` los vuelve a aplicar en cada reload.
PARAMETROS_AJUSTABLES = ("TOP_N_SELECTION", "TOP_N_SELECTION_US", "MAX_WEIGHT_PER_ASSET")
_overrides: dict[str, int | float] = {}


def set_overrides(valores: dict[str, int | float | None] | None) -> None:
    """Fija (o limpia, con ``None``/``{}``) los parámetros sobrescritos."""
    global _overrides
    limpios = {k: v for k, v in (valores or {}).items() if v is not None}
    desconocidos = set(limpios) - set(PARAMETROS_AJUSTABLES)
    if desconocidos:
        raise ValueError(f"Parámetros no ajustables: {', '.join(sorted(desconocidos))}")
    _overrides = limpios


def overrides() -> dict[str, int | float]:
    return dict(_overrides)


def set_root(root: Path | str | None) -> None:
    global _root
    _root = Path(root) if root is not None else None


def root() -> Path:
    if _root is not None:
        return _root
    from fog_radar.settings import get_settings

    return get_settings().module_data_dir("tendencia")
