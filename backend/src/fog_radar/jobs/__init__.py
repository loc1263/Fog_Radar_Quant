"""Pipelines batch de cada módulo. Punto de entrada: ``python -m fog_radar.jobs``."""

from __future__ import annotations

from collections.abc import Callable

# modulo -> función que corre el pipeline completo. Se completa al portar
# cada módulo (Fases 3-6).
PIPELINES: dict[str, Callable[..., None]] = {}
