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


def set_root(root: Path | str | None) -> None:
    global _root
    _root = Path(root) if root is not None else None


def root() -> Path:
    if _root is not None:
        return _root
    from fog_radar.settings import get_settings

    return get_settings().module_data_dir("tendencia")
