"""
Paleta de colores, tipografía y estilos de párrafo compartidos por el
informe PDF (reporting.pdf_report). Separado de pdf_report.py para que el
"look and feel" (que sigue las preferencias de formato guardadas del
usuario: Calibri, fondo blanco, paleta print-friendly) se pueda ajustar en
un solo lugar sin tocar la lógica de construcción del documento.
"""

import os

import matplotlib.font_manager as fm
import matplotlib.pyplot as plt
from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont

from fog_radar.core import pdf as _pdf_common

# ---------------------------------------------------------------------------
# Paleta (según preferencias de formato del usuario)
# ---------------------------------------------------------------------------
COLOR_TEXT = colors.HexColor("#000000")
COLOR_LABEL = colors.HexColor("#1B3A6B")
COLOR_ACCENT = colors.HexColor("#9A6E00")
COLOR_RED = colors.HexColor("#B83232")
COLOR_GREEN = colors.HexColor("#1A7A55")
COLOR_BLUE = colors.HexColor("#1B5EA5")
COLOR_BORDER = colors.HexColor("#D8DCE3")
COLOR_CARD_BG = colors.HexColor("#F7F8FA")
COLOR_ALERT_RED_BG = colors.HexColor("#FEF5F5")
COLOR_ALERT_GREEN_BG = colors.HexColor("#F3FBF7")
COLOR_ALERT_AMBER_BG = colors.HexColor("#FDFAF0")
COLOR_ALERT_BLUE_BG = colors.HexColor("#F2F6FD")
COLOR_WHITE = colors.HexColor("#FFFFFF")

MPL_HEX_LABEL = "#1B3A6B"
MPL_HEX_ACCENT = "#9A6E00"
MPL_HEX_RED = "#B83232"
MPL_HEX_GREEN = "#1A7A55"
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
# Tipografía: Calibri (preferencia del usuario) vía Carlito, un sustituto
# métricamente compatible y de licencia libre (SIL OFL) — Calibri en sí no
# es una fuente libre y no viene instalada por defecto en Linux/macOS.
#
# Para que esto funcione IGUAL en Windows, macOS y Linux sin depender de qué
# fuentes tenga instalado el sistema, los .ttf de Carlito se incluyen
# dentro del propio proyecto en fonts/. Si por lo que sea esos archivos no
# están o no se pueden leer, se cae a las fuentes estándar de ReportLab
# (Helvetica), que están garantizadas en cualquier sistema — así el informe
# nunca falla por un tema de fuentes, en el peor caso se ve con una
# tipografía ligeramente distinta a Calibri.
# ---------------------------------------------------------------------------
# Fog Radar: los .ttf de Carlito viven en core/pdf/fonts (compartidos por todos los PDF).
FONTS_DIR = _pdf_common.FONTS_DIR


def register_fonts():
    """Devuelve (font_regular, font_bold, font_italic) ya registrados en
    ReportLab, y configura matplotlib a juego. Con fallback a Helvetica."""
    paths = {
        "regular": os.path.join(FONTS_DIR, "Carlito-Regular.ttf"),
        "bold": os.path.join(FONTS_DIR, "Carlito-Bold.ttf"),
        "italic": os.path.join(FONTS_DIR, "Carlito-Italic.ttf"),
    }
    try:
        pdfmetrics.registerFont(TTFont("Calibri", paths["regular"]))
        pdfmetrics.registerFont(TTFont("Calibri-Bold", paths["bold"]))
        pdfmetrics.registerFont(TTFont("Calibri-Italic", paths["italic"]))
        fm.fontManager.addfont(paths["regular"])
        fm.fontManager.addfont(paths["bold"])
        plt.rcParams["font.family"] = "Carlito"
        return "Calibri", "Calibri-Bold", "Calibri-Italic"
    except Exception as e:
        print(
            f"[reporting.theme] No se pudo cargar la fuente Calibri/Carlito desde {FONTS_DIR} ({e}); "
            "se usa Helvetica (fuente estándar, disponible en todos los sistemas)."
        )

    # Fallback: fuentes base14 de ReportLab, no requieren ningún archivo y
    # funcionan igual en Windows/macOS/Linux.
    plt.rcParams["font.family"] = "sans-serif"
    return "Helvetica", "Helvetica-Bold", "Helvetica-Oblique"


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
            textColor=COLOR_WHITE,
        )
    )
    return styles
