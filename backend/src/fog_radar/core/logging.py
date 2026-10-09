"""Configuración de logging común (API y pipelines)."""

from __future__ import annotations

import logging
import sys

_FORMAT = "%(asctime)s | %(levelname)-7s | %(name)s | %(message)s"


def configure_logging(level: str = "INFO") -> None:
    root = logging.getLogger()
    # Idempotente: no duplicar handlers si se llama más de una vez (tests, reload).
    if not any(getattr(h, "_fog_radar", False) for h in root.handlers):
        # En Windows, stdout redirigido a un pipe usa cp1252: '≥', '→', emojis, etc.
        # lanzarían UnicodeEncodeError ("--- Logging error ---"). Se reemplazan en vez de fallar.
        reconfigure = getattr(sys.stdout, "reconfigure", None)
        if reconfigure is not None and (sys.stdout.encoding or "").lower().replace("-", "") != "utf8":
            try:
                reconfigure(errors="backslashreplace")
            except (ValueError, OSError):  # pragma: no cover - stream ya en uso / no reconfigurable
                pass
        handler = logging.StreamHandler(sys.stdout)
        handler.setFormatter(logging.Formatter(_FORMAT))
        handler._fog_radar = True  # type: ignore[attr-defined]
        root.addHandler(handler)
    root.setLevel(level.upper())


def get_logger(name: str) -> logging.Logger:
    return logging.getLogger(f"fog_radar.{name}")
