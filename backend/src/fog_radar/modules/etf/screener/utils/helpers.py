"""Funciones auxiliares comunes."""

from datetime import datetime

from fog_radar.core.logging import get_logger

log = get_logger("etf.screener")


def get_timestamp():
    """Obtiene timestamp en formato YYYYMMDD_HHMMSS."""
    return datetime.now().strftime("%Y%m%d_%H%M%S")


def print_section_header(title):
    """Imprime un encabezado de sección formateado."""
    from ..config import SEPARATOR_LINE

    log.info(SEPARATOR_LINE)
    log.info(title)
    log.info(SEPARATOR_LINE)
