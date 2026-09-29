"""
Genera un informe PDF con los resultados del pipeline (selección de
acciones por Markowitz + proyección de tendencia a 6 meses), usando el
formato de marca del usuario (ver reporting.theme):

  - Página Letter, fondo blanco en todo el documento (nunca fondos oscuros)
  - Fuente Calibri (Carlito como sustituto métricamente compatible)
  - Texto principal en negro, subtítulos/etiquetas en azul oscuro #1B3A6B
  - Paleta: ámbar #9A6E00 (acento), verde #1A7A55, rojo #B83232,
    azul #1B5EA5, tarjetas #F7F8FA, bordes #D8DCE3
  - Encabezado con título + fecha, secciones en mayúsculas color acento,
    pie de página con disclaimer

Ejecutar (después de markowitz y trend_forecast, vía wrapper de
compatibilidad en la raíz del proyecto):
    python step06_generate_report.py

Incluye una sección completa (resumen, portafolio, tendencia, sectores,
momentum) por cada análisis que `modeling.markowitz` haya generado (ver
config.ANALYSES): la principal (Asia + EEUU) y cualquier complementario
(p. ej. "solo EEUU") que exista para esta corrida, cada uno en una
página nueva después del anterior.

Genera, dentro de la carpeta de la corrida actual (output/<YYYYMMDD>_<NN>,
ver tendencia.config.get_run_dir):
    informe_portafolio_<YYYYMMDD>.pdf
"""

import os
from datetime import datetime

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd
from reportlab.lib import colors
from reportlab.lib.pagesizes import LETTER
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import (
    Image,
    KeepTogether,
    PageBreak,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
    Table,
    TableStyle,
)

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.reporting import theme
from fog_radar.modules.tendencia.reporting.run_data import compute_sector_summary, load_analysis

_MESES_ES = {
    1: "enero",
    2: "febrero",
    3: "marzo",
    4: "abril",
    5: "mayo",
    6: "junio",
    7: "julio",
    8: "agosto",
    9: "septiembre",
    10: "octubre",
    11: "noviembre",
    12: "diciembre",
}


def _today_strings():
    now = datetime.now()
    today_str = f"{now.day} de {_MESES_ES[now.month]} de {now.year}"
    today_file = now.strftime("%Y%m%d")
    return today_str, today_file


# ---------------------------------------------------------------------------
# Gráficos (matplotlib, fondo blanco, paleta print-friendly)
# ---------------------------------------------------------------------------
def _bar_chart(labels, values, colors_, xlabel, label_offset, out_path, legend_handles=None):
    """Barras horizontales con el estilo del informe (fondo blanco, sin
    bordes superior/derecho, grilla suave) y el valor al final de cada barra."""
    fig, ax = plt.subplots(figsize=(6.8, 3.6), dpi=200)
    fig.patch.set_facecolor("white")
    ax.set_facecolor("white")

    ax.barh(labels, values, color=colors_, height=0.6)
    ax.set_xlabel(xlabel, fontsize=10, color="#000000")
    ax.tick_params(axis="both", labelsize=8.5, colors="#000000")
    for spine in ["top", "right"]:
        ax.spines[spine].set_visible(False)
    for spine in ["left", "bottom"]:
        ax.spines[spine].set_color(theme.MPL_HEX_BORDER)
    ax.grid(axis="x", color=theme.MPL_HEX_BORDER, linewidth=0.6, alpha=0.7)
    ax.set_axisbelow(True)

    for i, v in enumerate(values):
        ax.text(v + label_offset, i, f"{v:.1f}%", va="center", fontsize=8, color=theme.MPL_HEX_LABEL)
    if legend_handles:
        ax.legend(handles=legend_handles, loc="lower right", fontsize=7.5, frameon=False)

    fig.tight_layout()
    fig.savefig(out_path, facecolor="white")
    plt.close(fig)


def make_weights_chart(merged, out_path):
    data = merged.sort_values("peso_final", ascending=True)
    _bar_chart(
        data["nombre"],
        list(data["peso_final"] * 100),
        theme.MPL_HEX_BLUE,
        "Peso en el portafolio (%)",
        0.15,
        out_path,
    )


def make_forecast_chart(merged, out_path):
    from matplotlib.patches import Patch

    data = merged.sort_values("retorno_esperado_6m_pct", ascending=True)
    bar_colors = [theme.SIGNAL_COLOR.get(s, theme.MPL_HEX_BORDER) for s in data["senal_tecnica"]]
    handles = [
        Patch(color=theme.MPL_HEX_GREEN, label="Alcista"),
        Patch(color=theme.MPL_HEX_ACCENT, label="Lateral / mixta"),
        Patch(color=theme.MPL_HEX_RED, label="Bajista"),
    ]
    _bar_chart(
        data["nombre"],
        list(data["retorno_esperado_6m_pct"]),
        bar_colors,
        "Retorno proyectado a 6 meses (%, escenario central)",
        0.4,
        out_path,
        legend_handles=handles,
    )


# ---------------------------------------------------------------------------
# Construcción del documento
# ---------------------------------------------------------------------------
def _data_table_style(*extra):
    """Estilo común de las tablas de datos: encabezado azul oscuro, filas
    alternadas, grilla suave. `extra` agrega reglas propias de cada tabla
    (alineaciones, tamaño de fuente)."""
    return TableStyle(
        [
            ("BACKGROUND", (0, 0), (-1, 0), theme.COLOR_LABEL),
            ("ROWBACKGROUNDS", (0, 1), (-1, -1), [theme.COLOR_WHITE, theme.COLOR_CARD_BG]),
            ("GRID", (0, 0), (-1, -1), 0.5, theme.COLOR_BORDER),
            ("TOPPADDING", (0, 0), (-1, -1), 6),
            ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
            ("LEFTPADDING", (0, 0), (-1, -1), 6),
            ("RIGHTPADDING", (0, 0), (-1, -1), 6),
            ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
            *extra,
        ]
    )


def _pct_cell(styles, value):
    return Paragraph(f"{value:.1f}%" if pd.notna(value) else "—", styles["TableCell"])


def make_metric_card(styles, label, value, value_color=None, width=1.55 * inch):
    value_style = ParagraphStyle(
        "CardValueColored",
        parent=styles["CardValue"],
        textColor=value_color or theme.COLOR_TEXT,
    )
    content = [
        [Paragraph(label, styles["CardLabel"])],
        [Paragraph(value, value_style)],
    ]
    t = Table(content, colWidths=[width - 0.24 * inch])
    t.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, -1), theme.COLOR_CARD_BG),
                ("BOX", (0, 0), (-1, -1), 0.75, theme.COLOR_BORDER),
                ("LEFTPADDING", (0, 0), (-1, -1), 12),
                ("RIGHTPADDING", (0, 0), (-1, -1), 12),
                ("TOPPADDING", (0, 0), (-1, -1), 12),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 12),
            ]
        )
    )
    return t


def build_metric_cards(styles, perf):
    p_final = perf["portafolio_15_finales"]
    cards = Table(
        [
            [
                make_metric_card(
                    styles,
                    "Retorno esperado anual",
                    f"{p_final['retorno_esperado_anual']:.1%}",
                    theme.COLOR_GREEN,
                ),
                make_metric_card(
                    styles, "Volatilidad anual", f"{p_final['volatilidad_anual']:.1%}", theme.COLOR_BLUE
                ),
                make_metric_card(
                    styles, "Sharpe Ratio", f"{p_final['sharpe_ratio']:.2f}", theme.COLOR_ACCENT
                ),
                make_metric_card(
                    styles, "Tasa libre de riesgo", f"{perf['tasa_libre_riesgo_anual']:.2%}", theme.COLOR_TEXT
                ),
            ]
        ],
        colWidths=[1.6 * inch] * 4,
    )
    cards.setStyle(
        TableStyle(
            [
                ("LEFTPADDING", (0, 0), (-1, -1), 3),
                ("RIGHTPADDING", (0, 0), (-1, -1), 3),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
            ]
        )
    )
    return cards


def build_portfolio_table(styles, merged):
    header_cells = [
        Paragraph(h, styles["TableHeader"])
        for h in ["Ticker", "Empresa", "País", "Sector", "Peso", "Retorno anual", "Señal 6m"]
    ]
    table_data = [header_cells]
    for ticker, row in merged.iterrows():
        signal = row["senal_tecnica"]
        signal_style = ParagraphStyle(
            "SignalCell",
            parent=styles["TableCell"],
            fontName=theme.FONT_BOLD,
            textColor=theme.SIGNAL_COLOR_PDF.get(signal, theme.COLOR_TEXT),
        )
        table_data.append(
            [
                Paragraph(ticker, styles["TableCell"]),
                Paragraph(str(row.get("nombre", "")), styles["TableCell"]),
                Paragraph(str(row.get("pais", "")), styles["TableCell"]),
                Paragraph(str(row.get("sector", "")), styles["TableCell"]),
                Paragraph(f"{row['peso_final']:.1%}", styles["TableCell"]),
                Paragraph(f"{row['retorno_esperado_anual_pct']:.1f}%", styles["TableCell"]),
                Paragraph(signal, signal_style),
            ]
        )

    portfolio_table = Table(
        table_data,
        colWidths=[0.78 * inch, 1.35 * inch, 0.55 * inch, 1.55 * inch, 0.45 * inch, 0.9 * inch, 0.9 * inch],
        repeatRows=1,
    )
    portfolio_table.setStyle(_data_table_style(("ALIGN", (4, 0), (5, -1), "RIGHT")))
    return portfolio_table


def build_forecast_table(styles, merged):
    fc_header = [
        Paragraph(h, styles["TableHeader"])
        for h in ["Ticker", "Precio actual", "Proyectado 6m", "Rango 90%", "Retorno 6m"]
    ]
    fc_data = [fc_header]
    for ticker, row in merged.iterrows():
        fc_data.append(
            [
                Paragraph(ticker, styles["TableCell"]),
                Paragraph(f"${row['precio_actual_usd']:.2f}", styles["TableCell"]),
                Paragraph(f"${row['precio_proyectado_6m_usd']:.2f}", styles["TableCell"]),
                Paragraph(
                    f"${row['precio_min_90pct_6m_usd']:.2f} – ${row['precio_max_90pct_6m_usd']:.2f}",
                    styles["TableCell"],
                ),
                Paragraph(f"{row['retorno_esperado_6m_pct']:.1f}%", styles["TableCell"]),
            ]
        )
    fc_table = Table(
        fc_data, colWidths=[0.8 * inch, 1.15 * inch, 1.15 * inch, 2.2 * inch, 1.1 * inch], repeatRows=1
    )
    fc_table.setStyle(
        _data_table_style(
            ("ALIGN", (1, 0), (2, -1), "RIGHT"),
            ("ALIGN", (4, 0), (4, -1), "RIGHT"),
            ("FONTSIZE", (0, 0), (-1, -1), 10),
        )
    )
    return fc_table


def build_sector_narrative(sector_summary, n_seleccionadas):
    top_sector = sector_summary.index[0]
    top_row = sector_summary.iloc[0]
    top_by_weight = sector_summary.sort_values("peso_total", ascending=False).index[0]

    if top_sector == top_by_weight:
        return (
            f"<b>{top_sector}</b> es el sector con mejor proyección dentro de la selección: agrupa "
            f"{int(top_row['n_acciones'])} de las {n_seleccionadas} acciones ({top_row['peso_total']:.1%} del "
            f"portafolio), con un retorno proyectado a 6 meses promedio de {top_row['retorno_6m_prom']:.1f}% "
            f"y señal técnica predominantemente <b>{top_row['senal_predominante']}</b> "
            f"({top_row['tickers']}). Además es el sector con mayor peso asignado por el modelo, "
            "así que la concentración de capital y la mejor proyección coinciden."
        )
    weight_row = sector_summary.loc[top_by_weight]
    return (
        f"<b>{top_sector}</b> es el sector con mejor proyección a 6 meses dentro de la selección "
        f"(retorno promedio {top_row['retorno_6m_prom']:.1f}%, señal predominante "
        f"<b>{top_row['senal_predominante']}</b>; {top_row['tickers']}). En peso asignado por el "
        f"modelo, sin embargo, el sector dominante es <b>{top_by_weight}</b> "
        f"({weight_row['peso_total']:.1%} del portafolio) — es decir, el modelo concentró más "
        "capital ahí por su menor volatilidad/correlación, aunque no sea el de mayor retorno "
        "proyectado."
    )


def build_sector_table(styles, sector_summary):
    sector_header = [
        Paragraph(h, styles["TableHeader"])
        for h in ["Sector", "N° acciones", "Peso total", "Retorno 6m prom.", "Señal predominante"]
    ]
    sector_data = [sector_header]
    for sector, row in sector_summary.iterrows():
        signal_style = ParagraphStyle(
            "SectorSignalCell",
            parent=styles["TableCell"],
            fontName=theme.FONT_BOLD,
            textColor=theme.SIGNAL_COLOR_PDF.get(row["senal_predominante"], theme.COLOR_TEXT),
        )
        sector_data.append(
            [
                Paragraph(sector, styles["TableCell"]),
                Paragraph(str(int(row["n_acciones"])), styles["TableCell"]),
                Paragraph(f"{row['peso_total']:.1%}", styles["TableCell"]),
                Paragraph(f"{row['retorno_6m_prom']:.1f}%", styles["TableCell"]),
                Paragraph(row["senal_predominante"], signal_style),
            ]
        )
    sector_table = Table(
        sector_data, colWidths=[2.3 * inch, 0.85 * inch, 0.95 * inch, 1.2 * inch, 1.5 * inch], repeatRows=1
    )
    sector_table.setStyle(_data_table_style(("ALIGN", (1, 0), (3, -1), "RIGHT")))
    return sector_table


def build_momentum_table(styles, momentum):
    mom_header = [
        Paragraph(h, styles["TableHeader"])
        for h in ["#", "Ticker", "Empresa", "1m", "3m", "6m", "12m", "Score"]
    ]
    mom_data = [mom_header]
    for _, row in momentum.iterrows():
        score = row.get("momentum_score_pct")
        score_style = ParagraphStyle(
            "MomentumScoreCell",
            parent=styles["TableCell"],
            fontName=theme.FONT_BOLD,
            textColor=theme.COLOR_GREEN if pd.notna(score) and score >= 0 else theme.COLOR_RED,
        )
        mom_data.append(
            [
                Paragraph(str(int(row["rank"])), styles["TableCell"]),
                Paragraph(str(row["ticker"]), styles["TableCell"]),
                Paragraph(str(row.get("nombre", "")), styles["TableCell"]),
                *(_pct_cell(styles, row.get(f"momentum_{w}m_pct")) for w in (1, 3, 6, 12)),
                Paragraph(f"{score:.1f}%" if pd.notna(score) else "—", score_style),
            ]
        )
    mom_table = Table(
        mom_data,
        colWidths=[
            0.35 * inch,
            0.75 * inch,
            1.9 * inch,
            0.72 * inch,
            0.72 * inch,
            0.72 * inch,
            0.72 * inch,
            0.72 * inch,
        ],
        repeatRows=1,
    )
    mom_table.setStyle(
        _data_table_style(
            ("ALIGN", (0, 0), (0, -1), "CENTER"),
            ("ALIGN", (3, 0), (7, -1), "RIGHT"),
            ("FONTSIZE", (0, 0), (-1, -1), 9.5),
        )
    )
    return mom_table


def build_universe_section(
    styles,
    charts_dir,
    chart_prefix,
    perf,
    merged,
    momentum,
    intro_text,
    momentum_intro_text,
    section_suffix="",
):
    """
    Arma la secuencia de flowables de un análisis completo (resumen
    ejecutivo, portafolio seleccionado + gráfico, proyección a 6 meses +
    gráfico, sector en tendencia, top momentum mensual) para un universo
    dado. `section_suffix` se agrega a los títulos (p. ej. " (EEUU)") para
    distinguir el análisis "solo EEUU" del combinado cuando ambos
    aparecen en el mismo documento. Extraída de main() para generar los
    dos análisis sin duplicar la construcción de tablas/gráficos.
    """
    story = []
    n_seleccionadas = len(merged)

    story.append(Paragraph(f"RESUMEN EJECUTIVO{section_suffix}", styles["SectionHeader"]))
    story.append(build_metric_cards(styles, perf))
    story.append(Spacer(1, 10))
    story.append(Paragraph(intro_text, styles["BodyText11"]))

    story.append(
        Paragraph(
            f"PORTAFOLIO SELECCIONADO{section_suffix} — {n_seleccionadas} ACCIONES", styles["SectionHeader"]
        )
    )
    story.append(build_portfolio_table(styles, merged))
    story.append(Spacer(1, 10))

    weights_chart_path = os.path.join(charts_dir, f"{chart_prefix}_weights_chart.png")
    make_weights_chart(merged, weights_chart_path)
    story.append(Image(weights_chart_path, width=6.8 * inch, height=3.6 * inch))

    story.append(Paragraph(f"PROYECCIÓN DE TENDENCIA A 6 MESES{section_suffix}", styles["SectionHeader"]))
    story.append(
        Paragraph(
            "Proyección estadística tipo movimiento geométrico browniano (GBM), a partir del retorno y "
            "volatilidad histórica anualizada de cada acción. El precio central es el escenario más "
            "probable; el rango min-máx corresponde a una banda de confianza aproximada del 90%. "
            "<b>No es una predicción garantizada</b>: es un rango de escenarios estadísticos basado en el "
            "comportamiento de los últimos 5 años.",
            styles["BodyText11"],
        )
    )
    story.append(build_forecast_table(styles, merged))
    story.append(Spacer(1, 10))

    forecast_chart_path = os.path.join(charts_dir, f"{chart_prefix}_forecast_chart.png")
    make_forecast_chart(merged, forecast_chart_path)
    story.append(Image(forecast_chart_path, width=6.8 * inch, height=3.6 * inch))

    story.append(Paragraph(f"SECTOR EN TENDENCIA{section_suffix}", styles["SectionHeader"]))
    sector_summary = compute_sector_summary(merged)
    story.append(Paragraph(build_sector_narrative(sector_summary, n_seleccionadas), styles["BodyText11"]))
    story.append(Spacer(1, 6))
    story.append(build_sector_table(styles, sector_summary))

    if not momentum.empty:
        story.append(Paragraph(f"TOP ACCIONES POR MOMENTUM MENSUAL{section_suffix}", styles["SectionHeader"]))
        story.append(Paragraph(momentum_intro_text, styles["BodyText11"]))
        story.append(build_momentum_table(styles, momentum))
        story.append(Spacer(1, 10))

    return story


def _header_footer(canvas, doc, today_str):
    canvas.saveState()
    width, height = LETTER

    # Fondo de encabezado (ámbar muy pálido)
    canvas.setFillColor(colors.HexColor("#FDFAF0"))
    canvas.rect(0, height - 0.95 * inch, width, 0.95 * inch, stroke=0, fill=1)
    canvas.setStrokeColor(theme.COLOR_BORDER)
    canvas.setLineWidth(0.75)
    canvas.line(0.6 * inch, height - 0.95 * inch, width - 0.6 * inch, height - 0.95 * inch)

    canvas.setFont(theme.FONT_BOLD, 13)
    canvas.setFillColor(theme.COLOR_TEXT)
    canvas.drawString(
        0.6 * inch, height - 0.55 * inch, "Informe de Portafolio — Modelo Markowitz Asia + EEUU"
    )
    canvas.setFont(theme.FONT_REGULAR, 11)
    canvas.setFillColor(theme.COLOR_LABEL)
    canvas.drawString(0.6 * inch, height - 0.75 * inch, today_str)

    # Pie de página con disclaimer
    canvas.setFont(theme.FONT_ITALIC, 11)
    canvas.setFillColor(theme.COLOR_LABEL)
    canvas.drawString(
        0.6 * inch,
        0.5 * inch,
        "Informe generado automáticamente con fines informativos. No constituye asesoría financiera.",
    )
    canvas.setFont(theme.FONT_REGULAR, 11)
    canvas.drawRightString(width - 0.6 * inch, 0.5 * inch, f"Página {doc.page}")
    canvas.restoreState()


def main():
    styles = theme.build_styles()
    run_dir = config.get_run_dir()
    today_str, today_file = _today_strings()

    charts_dir = os.path.join(run_dir, "_charts")
    os.makedirs(charts_dir, exist_ok=True)

    out_path = os.path.join(run_dir, f"informe_portafolio_{today_file}.pdf")
    doc = SimpleDocTemplate(
        out_path,
        pagesize=LETTER,
        topMargin=1.15 * inch,
        bottomMargin=0.8 * inch,
        leftMargin=0.6 * inch,
        rightMargin=0.6 * inch,
        title="Informe de Portafolio - Modelo Markowitz Asia + EEUU",
    )

    # Una sección completa por cada análisis disponible (ver config.ANALYSES
    # y modeling.markowitz): el principal siempre debe existir; los
    # complementarios se omiten en silencio si no se generaron ese día
    # (universo insuficiente). Agregar un análisis nuevo a config.ANALYSES
    # no requiere tocar este loop.
    story = []
    primary_sharpe = None
    is_first_section = True

    for spec in config.ANALYSES:
        data = load_analysis(run_dir, suffix=spec.key)
        if data is None:
            continue
        perf, merged, notes, momentum = data.perf, data.merged, data.notes, data.momentum
        n_sel = len(perf.get("tickers_seleccionados", merged.index))

        if is_first_section:
            # --- Contexto de mercado: es macro, no depende del universo,
            # se muestra una sola vez antes del primer análisis disponible ---
            if notes:
                note_rows = [[Paragraph(n, styles["NoteText"])] for n in notes]
                note_table = Table(note_rows, colWidths=[6.8 * inch])
                note_table.setStyle(
                    TableStyle(
                        [
                            ("BACKGROUND", (0, 0), (-1, -1), theme.COLOR_ALERT_BLUE_BG),
                            ("BOX", (0, 0), (-1, -1), 0.75, theme.COLOR_BORDER),
                            ("LEFTPADDING", (0, 0), (-1, -1), 12),
                            ("RIGHTPADDING", (0, 0), (-1, -1), 12),
                            ("TOPPADDING", (0, 0), (-1, -1), 10),
                            ("BOTTOMPADDING", (0, 0), (-1, -1), 10),
                        ]
                    )
                )
                story.append(Paragraph("CONTEXTO DE MERCADO", styles["SectionHeader"]))
                story.append(note_table)
                story.append(Spacer(1, 10))
            section_suffix = ""
        else:
            story.append(PageBreak())
            story.append(Paragraph(f"ANÁLISIS COMPLEMENTARIO — {spec.label.upper()}", styles["ReportTitle"]))
            story.append(
                Paragraph(
                    f"Mismo modelo y metodología del análisis anterior, aplicado únicamente al "
                    f"universo '{spec.label}', para comparar cómo se ve el portafolio bajo ese "
                    "universo más acotado.",
                    styles["ReportSubtitle"],
                )
            )
            story.append(Spacer(1, 10))
            section_suffix = f" ({spec.label.upper()})"

        intro_text = (
            f"El modelo de Markowitz/covarianza, aplicado sobre el universo '{spec.label}' "
            f"({len(spec.tickers)} acciones, universo {config.UNIVERSE_SOURCE}) con 5 años de "
            f"historial semanal de precios en USD, seleccionó las {n_sel} acciones que maximizan "
            "el Sharpe Ratio del portafolio."
        )
        momentum_intro_text = (
            f"Ranking independiente de la selección de Markowitz: evalúa el universo "
            f"'{spec.label}' ({len(spec.tickers)} acciones) por fuerza y persistencia de la "
            "tendencia mensual (retorno acumulado a 1, 3, 6 y 12 meses, ponderado por la "
            "proporción de meses positivos del último año). Puede incluir acciones que el "
            "modelo de covarianza descartó por volatilidad o correlación alta, pero que "
            "muestran el mejor impulso de corto plazo."
        )
        chart_prefix = spec.key.strip("_") or "global"
        story.extend(
            build_universe_section(
                styles,
                charts_dir,
                chart_prefix,
                perf,
                merged,
                momentum,
                intro_text,
                momentum_intro_text,
                section_suffix=section_suffix,
            )
        )

        if spec.key == "":
            primary_sharpe = perf["portafolio_15_finales"]["sharpe_ratio"]
        is_first_section = False

    if primary_sharpe is None:
        raise RuntimeError("No se generó ningún análisis (ver modeling.markowitz); no hay nada que reportar.")

    # --- Limitaciones (compartidas por todos los análisis incluidos arriba) ---
    limit_header = Paragraph("LIMITACIONES Y ADVERTENCIAS", styles["SectionHeader"])
    if config.latest_macro_file():
        rf_note = (
            "La tasa libre de riesgo y el contexto de mercado (curva de tasas, spreads de crédito) "
            "provienen de datos reales de FRED (fetch_macro_data.py)."
        )
    else:
        rf_note = (
            "La tasa libre de riesgo usada proviene de ^IRX (yfinance) como respaldo, no de FRED, porque "
            "fetch_macro_data.py no se había corrido con una API key de FRED al momento de este informe."
        )
    limitations = [
        "Este informe es un producto cuantitativo (MVP) y no constituye asesoría financiera ni "
        "recomendación de inversión. Consulta a un asesor financiero antes de tomar decisiones.",
        "El modelo de Markowitz usa retornos y covarianzas históricos de 5 años como estimador del "
        "comportamiento futuro; esto no garantiza que el mismo patrón se repita, y tiende a "
        "sobreponderar activos que tuvieron un desempeño excepcional recientemente.",
        f"El Sharpe Ratio del portafolio principal ({primary_sharpe:.2f}) refleja el período de mercado usado para "
        "estimarlo; no debe interpretarse como un retorno esperado garantizado hacia adelante. Lo "
        "mismo aplica a cualquier análisis complementario incluido en este informe.",
        rf_note,
    ]
    limit_rows = [[Paragraph(f"• {item}", styles["NoteText"])] for item in limitations]
    limit_table = Table(limit_rows, colWidths=[6.8 * inch])
    limit_table.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, -1), theme.COLOR_ALERT_AMBER_BG),
                ("BOX", (0, 0), (-1, -1), 0.75, theme.COLOR_BORDER),
                ("LEFTPADDING", (0, 0), (-1, -1), 12),
                ("RIGHTPADDING", (0, 0), (-1, -1), 12),
                ("TOPPADDING", (0, 0), (-1, -1), 10),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 10),
            ]
        )
    )
    story.append(KeepTogether([limit_header, limit_table]))

    def _on_page(canvas, doc_):
        _header_footer(canvas, doc_, today_str)

    doc.build(story, onFirstPage=_on_page, onLaterPages=_on_page)
    print(f"Informe generado: {out_path}")
    return out_path


if __name__ == "__main__":
    main()
