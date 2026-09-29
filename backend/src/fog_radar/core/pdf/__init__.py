"""Estilo común de los informes PDF (ReportLab) del sitio.

Formato fijado por el usuario para todos los PDF del proyecto: página Letter,
fondo blanco (nunca fondos oscuros), fuente Calibri (o Carlito, sustituto
métrico-compatible, si Calibri no está instalada; Helvetica como último
recurso), texto principal en negro, subtítulos y etiquetas en azul oscuro,
títulos de sección en mayúsculas color ámbar, verde/rojo/azul solo para
resaltar datos clave, y pie de página con disclaimer + "Página X de Y".

Extraído de ``ETF_Choice/.../informe_pdf.py`` (Fase 5); el PDF de Tendencia lo
adopta en la Fase 7.
"""

from __future__ import annotations

import os
from collections.abc import Callable, Sequence
from typing import Any

import pandas as pd
from reportlab.lib import colors
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas as canvas_mod
from reportlab.platypus import Paragraph, Table, TableStyle

# --- Paleta print-friendly -----------------------------------------------------
BLANCO = colors.HexColor("#FFFFFF")
NEGRO = colors.HexColor("#000000")
NAVY = colors.HexColor("#1B3A6B")
AMBAR = colors.HexColor("#9A6E00")
ROJO = colors.HexColor("#B83232")
VERDE = colors.HexColor("#1A7A55")
AZUL = colors.HexColor("#1B5EA5")
BORDE = colors.HexColor("#D8DCE3")
CARD_BG = colors.HexColor("#F7F8FA")
AMBAR_PALE = colors.HexColor("#FDFAF0")
ROJO_PALE = colors.HexColor("#FEF5F5")
VERDE_PALE = colors.HexColor("#F3FBF7")
AZUL_PALE = colors.HexColor("#F2F6FD")

HEX_VERDE = "#1A7A55"
HEX_ROJO = "#B83232"
HEX_NAVY = "#1B3A6B"
HEX_AMBAR = "#9A6E00"

DISCLAIMER = "Análisis cuantitativo generado automáticamente. No constituye asesoramiento financiero."

# Carlito (SIL OFL) viaja con el código: el PDF se ve igual en un servidor sin fuentes.
FONTS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fonts")

_FUENTES_CANDIDATAS = (
    ("Calibri", r"C:\Windows\Fonts\calibri.ttf", r"C:\Windows\Fonts\calibrib.ttf"),
    (
        "Carlito",
        "/usr/share/fonts/truetype/crosextra/Carlito-Regular.ttf",
        "/usr/share/fonts/truetype/crosextra/Carlito-Bold.ttf",
    ),
    ("Calibri", "/Library/Fonts/Calibri.ttf", "/Library/Fonts/Calibri Bold.ttf"),
    ("Carlito", os.path.join(FONTS_DIR, "Carlito-Regular.ttf"), os.path.join(FONTS_DIR, "Carlito-Bold.ttf")),
)


def registrar_fuente() -> tuple[str, str]:
    """Registra Calibri/Carlito si están instaladas y devuelve (normal, negrita)."""
    for nombre, ruta_reg, ruta_bold in _FUENTES_CANDIDATAS:
        if not os.path.exists(ruta_reg):
            continue
        try:
            pdfmetrics.registerFont(TTFont(nombre, ruta_reg))
            if os.path.exists(ruta_bold):
                pdfmetrics.registerFont(TTFont(nombre + "-Bold", ruta_bold))
                return nombre, nombre + "-Bold"
            return nombre, nombre
        except Exception:
            continue
    return "Helvetica", "Helvetica-Bold"


FUENTE, FUENTE_BOLD = registrar_fuente()

_CURSIVAS_CANDIDATAS = (
    r"C:\Windows\Fonts\calibrii.ttf",
    "/usr/share/fonts/truetype/crosextra/Carlito-Italic.ttf",
    "/Library/Fonts/Calibri Italic.ttf",
    os.path.join(FONTS_DIR, "Carlito-Italic.ttf"),
)


def _registrar_cursiva() -> str:
    """Cursiva de la familia registrada (o la oblicua de Helvetica como respaldo)."""
    if FUENTE == "Helvetica":
        return "Helvetica-Oblique"
    nombre = FUENTE + "-Italic"
    for ruta in _CURSIVAS_CANDIDATAS:
        if os.path.exists(ruta):
            try:
                pdfmetrics.registerFont(TTFont(nombre, ruta))
                return nombre
            except Exception:
                continue
    return FUENTE


FUENTE_ITALIC = _registrar_cursiva()


def configurar_matplotlib() -> str:
    """Usa Carlito (métricamente igual a Calibri, incluida en ``fonts/``) en los gráficos.

    Devuelve la familia configurada (``sans-serif`` si no se pudo cargar).
    """
    import matplotlib.font_manager as fm
    import matplotlib.pyplot as plt

    try:
        for archivo in ("Carlito-Regular.ttf", "Carlito-Bold.ttf", "Carlito-Italic.ttf"):
            fm.fontManager.addfont(os.path.join(FONTS_DIR, archivo))
        plt.rcParams["font.family"] = "Carlito"
        return "Carlito"
    except Exception:
        plt.rcParams["font.family"] = "sans-serif"
        return "sans-serif"


def estilos() -> dict[str, ParagraphStyle]:
    """Estilos de párrafo comunes (piso de 9-10 pt, nunca texto gris)."""
    return {
        "seccion": ParagraphStyle(
            "seccion",
            fontName=FUENTE_BOLD,
            fontSize=11,
            textColor=AMBAR,
            leading=14,
            spaceBefore=4,
            spaceAfter=6,
        ),
        "nota": ParagraphStyle("nota", fontName=FUENTE, fontSize=11, textColor=NEGRO, leading=14),
        "badge_verde": ParagraphStyle(
            "badge_verde", fontName=FUENTE_BOLD, fontSize=11, textColor=VERDE, spaceBefore=6, spaceAfter=4
        ),
        "badge_azul": ParagraphStyle(
            "badge_azul", fontName=FUENTE_BOLD, fontSize=11, textColor=AZUL, spaceBefore=6, spaceAfter=4
        ),
        "badge_rojo": ParagraphStyle(
            "badge_rojo", fontName=FUENTE_BOLD, fontSize=11, textColor=ROJO, spaceBefore=6, spaceAfter=4
        ),
        "th": ParagraphStyle("th", fontName=FUENTE_BOLD, fontSize=10, textColor=NAVY, leading=12),
        "th_r": ParagraphStyle(
            "th_r", fontName=FUENTE_BOLD, fontSize=10, textColor=NAVY, leading=12, alignment=2
        ),
        "celda": ParagraphStyle("celda", fontName=FUENTE, fontSize=10, textColor=NEGRO, leading=12),
        "celda_r": ParagraphStyle(
            "celda_r", fontName=FUENTE, fontSize=10, textColor=NEGRO, leading=12, alignment=2
        ),
        "celda_ticker": ParagraphStyle(
            "celda_ticker", fontName=FUENTE_BOLD, fontSize=10, textColor=NAVY, leading=12
        ),
        "celda_cat": ParagraphStyle("celda_cat", fontName=FUENTE, fontSize=10, textColor=NAVY, leading=12),
    }


# --- Formato -------------------------------------------------------------------
def es_vacio(v: Any) -> bool:
    if v is None:
        return True
    try:
        return bool(pd.isna(v))
    except (TypeError, ValueError):
        return v == ""


def fmt_num(v: Any, dec: int = 2) -> str:
    if es_vacio(v):
        return "\u2014"
    try:
        n = float(v)
    except (TypeError, ValueError):
        return str(v)
    return f"{n:,.{dec}f}"


def fmt_pct(v: Any, dec: int = 2) -> str:
    """Porcentaje ya expresado en puntos (12.3 → "+12.30%")."""
    if es_vacio(v):
        return "\u2014"
    try:
        n = float(v)
    except (TypeError, ValueError):
        return str(v)
    signo = "+" if n > 0 else ""
    return f"{signo}{fmt_num(n, dec)}%"


def fmt_fecha_yyyymmdd(v: Any) -> str:
    """``20260924`` → ``24/09/2026`` (otros formatos se devuelven tal cual)."""
    s = "" if v is None else str(v).split(".")[0]
    if len(s) == 8 and s.isdigit():
        return f"{s[6:8]}/{s[4:6]}/{s[0:4]}"
    return s or "\u2014"


def celda_signo(v: Any, dec: int, est: dict[str, ParagraphStyle]) -> Paragraph:
    """Celda numérica verde/rojo según signo (solo para datos clave: deltas, z-scores)."""
    if es_vacio(v):
        return Paragraph("\u2014", est["celda_r"])
    try:
        n = float(v)
    except (TypeError, ValueError):
        return Paragraph(str(v), est["celda_r"])
    color = HEX_VERDE if n > 0 else (HEX_ROJO if n < 0 else "#000000")
    signo = "+" if n > 0 else ""
    return Paragraph(f"<font color='{color}'><b>{signo}{fmt_num(n, dec)}</b></font>", est["celda_r"])


def celda_acento(texto: str, est: dict[str, ParagraphStyle]) -> Paragraph:
    """Celda destacada en ámbar (score principal de cada tabla)."""
    return Paragraph(f"<font color='{HEX_AMBAR}'><b>{texto}</b></font>", est["celda_r"])


# --- Tablas --------------------------------------------------------------------
def tabla(
    headers: Sequence[str],
    aligns: Sequence[str],
    fracs: Sequence[float],
    filas: list[list[Any]],
    est: dict[str, ParagraphStyle],
    frame_w: float,
    *,
    repeat_header: bool = True,
) -> Table:
    """Tabla con encabezado claro, grilla suave y filas alternadas blanco/gris muy claro.

    ``aligns``: ``"l"`` o ``"r"`` por columna (números siempre a la derecha).
    ``fracs``: fracción del ancho útil (``frame_w``) de cada columna.
    """
    col_widths = [frame_w * f for f in fracs]
    header_row = [
        Paragraph(h, est["th_r"] if a == "r" else est["th"]) for h, a in zip(headers, aligns, strict=True)
    ]
    data = [header_row, *filas]
    t = Table(data, colWidths=col_widths, repeatRows=1 if repeat_header else 0)
    cmds: list[tuple[Any, ...]] = [
        ("BACKGROUND", (0, 0), (-1, 0), CARD_BG),
        ("GRID", (0, 0), (-1, -1), 0.5, BORDE),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("TOPPADDING", (0, 0), (-1, -1), 4),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 4),
        ("LEFTPADDING", (0, 0), (-1, -1), 5),
        ("RIGHTPADDING", (0, 0), (-1, -1), 5),
    ]
    for i in range(1, len(data)):
        cmds.append(("BACKGROUND", (0, i), (-1, i), CARD_BG if i % 2 == 0 else BLANCO))
    t.setStyle(TableStyle(cmds))
    return t


# --- Encabezado y pie ----------------------------------------------------------
def canvas_con_marco(
    titulo: str,
    derecha: str,
    page_size: tuple[float, float],
    *,
    margen_l: float = 30,
    margen_r: float = 30,
    disclaimer: str = DISCLAIMER,
) -> Callable[..., canvas_mod.Canvas]:
    """``canvasmaker`` para ``doc.build`` con banda ámbar pálida (título 22 pt + texto a la
    derecha) y pie con disclaimer + "Página X de Y" en todas las páginas.

    Patrón estándar de ReportLab en dos pasadas: se guardan los estados de cada
    página y se dibujan al final, cuando ya se conoce el total.
    """

    class _Canvas(canvas_mod.Canvas):
        def __init__(self, *args: Any, **kwargs: Any) -> None:
            canvas_mod.Canvas.__init__(self, *args, **kwargs)
            self._saved_page_states: list[dict[str, Any]] = []

        def showPage(self) -> None:  # noqa: N802 - API de ReportLab
            self._saved_page_states.append(dict(self.__dict__))
            self._startPage()

        def save(self) -> None:
            total = len(self._saved_page_states)
            for estado in self._saved_page_states:
                self.__dict__.update(estado)
                self._marco(total)
                canvas_mod.Canvas.showPage(self)
            canvas_mod.Canvas.save(self)

        def _marco(self, total: int) -> None:
            w, h = page_size
            self.setFillColor(AMBAR_PALE)
            self.rect(0, h - 58, w, 58, stroke=0, fill=1)
            self.setFillColor(NAVY)
            self.setFont(FUENTE_BOLD, 22)
            self.drawString(margen_l, h - 34, titulo)
            self.setFont(FUENTE, 11)
            self.setFillColor(NEGRO)
            self.drawRightString(w - margen_r, h - 34, derecha)

            self.setStrokeColor(BORDE)
            self.setLineWidth(0.75)
            self.line(margen_l, 32, w - margen_r, 32)
            self.setFont(FUENTE, 10)
            self.setFillColor(NEGRO)
            self.drawString(margen_l, 18, disclaimer)
            self.drawRightString(w - margen_r, 18, f"Página {self.getPageNumber()} de {total}")

    return _Canvas
