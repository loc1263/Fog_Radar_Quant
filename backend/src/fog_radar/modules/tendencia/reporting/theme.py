"""
Paleta de colores, tipografía y estilos de párrafo compartidos por el
informe PDF (reporting.pdf_report). Separado de pdf_report.py para que el
"look and feel" (que sigue las preferencias de formato guardadas del
usuario: Calibri, fondo blanco, paleta print-friendly) se pueda ajustar en
un solo lugar sin tocar la lógica de construcción del documento.
"""

from reportlab.lib.enums import TA_LEFT
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet

from fog_radar.core import pdf as _pdf_common

# ---------------------------------------------------------------------------
# Paleta: la común de ``fog_radar.core.pdf`` (preferencias de formato del
# usuario). Fog Radar (Fase 7): los nombres de Tendencia se mantienen como
# alias para no tocar pdf_report.py.
# ---------------------------------------------------------------------------
COLOR_TEXT = _pdf_common.NEGRO
COLOR_LABEL = _pdf_common.NAVY
COLOR_ACCENT = _pdf_common.AMBAR
COLOR_RED = _pdf_common.ROJO
COLOR_GREEN = _pdf_common.VERDE
COLOR_BLUE = _pdf_common.AZUL
COLOR_BORDER = _pdf_common.BORDE
COLOR_CARD_BG = _pdf_common.CARD_BG
COLOR_ALERT_RED_BG = _pdf_common.ROJO_PALE
COLOR_ALERT_GREEN_BG = _pdf_common.VERDE_PALE
COLOR_ALERT_AMBER_BG = _pdf_common.AMBAR_PALE
COLOR_ALERT_BLUE_BG = _pdf_common.AZUL_PALE
COLOR_WHITE = _pdf_common.BLANCO

MPL_HEX_LABEL = _pdf_common.HEX_NAVY
MPL_HEX_ACCENT = _pdf_common.HEX_AMBAR
MPL_HEX_RED = _pdf_common.HEX_ROJO
MPL_HEX_GREEN = _pdf_common.HEX_VERDE
MPL_HEX_BLUE = "#1B5EA5"
MPL_HEX_BORDER = "#D8DCE3"

SIGNAL_COLOR = {
    "Alcista": MPL_HEX_GREEN,
    "Bajista": MPL_HEX_RED,
    "Lateral / mixta": MPL_HEX_ACCENT,
    "Datos insuficientes": MPL_HEX_BORDER,
}
SIGNAL_COLOR_PDF = {
    "Alcista": COLOR_GREEN,
    "Bajista": COLOR_RED,
    "Lateral / mixta": COLOR_ACCENT,
    "Datos insuficientes": COLOR_BORDER,
}

# ---------------------------------------------------------------------------
# Tipografía: la de ``fog_radar.core.pdf`` (Calibri si está instalada; si no,
# Carlito incluida en core/pdf/fonts, métricamente compatible; Helvetica
# como último respaldo), con la misma fuente en los gráficos de matplotlib.
# ---------------------------------------------------------------------------
FONTS_DIR = _pdf_common.FONTS_DIR


def register_fonts():
    """Devuelve (font_regular, font_bold, font_italic) ya registrados en
    ReportLab, y configura matplotlib a juego."""
    _pdf_common.configurar_matplotlib()
    return _pdf_common.FUENTE, _pdf_common.FUENTE_BOLD, _pdf_common.FUENTE_ITALIC


FONT_REGULAR, FONT_BOLD, FONT_ITALIC = register_fonts()


def build_styles():
    styles = getSampleStyleSheet()
    styles.add(
        ParagraphStyle(
            name="ReportTitle",
            fontName=FONT_BOLD,
            fontSize=22,
            leading=26,
            textColor=COLOR_TEXT,
            spaceAfter=2,
        )
    )
    styles.add(
        ParagraphStyle(
            name="ReportSubtitle",
            fontName=FONT_REGULAR,
            fontSize=11,
            leading=14,
            textColor=COLOR_LABEL,
        )
    )
    styles.add(
        ParagraphStyle(
            name="SectionHeader",
            fontName=FONT_BOLD,
            fontSize=11,
            leading=14,
            textColor=COLOR_ACCENT,
            spaceBefore=14,
            spaceAfter=6,
        )
    )
    styles.add(
        ParagraphStyle(
            name="BodyText11",
            fontName=FONT_REGULAR,
            fontSize=11,
            leading=15,
            textColor=COLOR_TEXT,
            spaceAfter=6,
            alignment=TA_LEFT,
        )
    )
    styles.add(
        ParagraphStyle(
            name="NoteText",
            fontName=FONT_REGULAR,
            fontSize=11,
            leading=14,
            textColor=COLOR_TEXT,
        )
    )
    styles.add(
        ParagraphStyle(
            name="CardLabel",
            fontName=FONT_REGULAR,
            fontSize=11,
            leading=13,
            textColor=COLOR_LABEL,
        )
    )
    styles.add(
        ParagraphStyle(
            name="CardValue",
            fontName=FONT_BOLD,
            fontSize=18,
            leading=22,
            textColor=COLOR_TEXT,
        )
    )
    styles.add(
        ParagraphStyle(
            name="Disclaimer",
            fontName=FONT_ITALIC,
            fontSize=11,
            leading=13,
            textColor=COLOR_LABEL,
        )
    )
    styles.add(
        ParagraphStyle(
            name="TableCell",
            fontName=FONT_REGULAR,
            fontSize=10,
            leading=12,
            textColor=COLOR_TEXT,
        )
    )
    styles.add(
        ParagraphStyle(
            name="TableHeader",
            fontName=FONT_BOLD,
            fontSize=10,
            leading=12,
            textColor=COLOR_LABEL,
        )
    )
    return styles
