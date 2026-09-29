"""PDF ejecutivo del módulo ETF (sección "Descargar").

Port de ``ETF_Choice/WEB/backend/analysis/informe_pdf.py``: mismas secciones y
columnas (Top 15, Excluidos por distorsión, Rotación, Diversificación, Ranking
por categoría, Top 15 Momentum, Top 15 Perf. + Beta y Top 15 Cap.), a partir de
la última corrida instalada en ``data/etf/current``. El estilo (paleta, fuente,
tablas, encabezado y pie) sale del módulo común :mod:`fog_radar.core.pdf`.

Orientación horizontal (Letter apaisado): las tablas de ETFs tienen 8-10
columnas y no entran con un tamaño legible (piso de 10 pt) en vertical.
"""

from __future__ import annotations

import io
from pathlib import Path
from typing import Any

import pandas as pd
from reportlab.lib.pagesizes import landscape, letter
from reportlab.platypus import PageBreak, Paragraph, SimpleDocTemplate, Spacer

from fog_radar.core import pdf as P
from fog_radar.modules.etf.analysis import rotacion as rotacion_mod

PAGE_SIZE = landscape(letter)
PAGE_W, PAGE_H = PAGE_SIZE
MARGEN_L = 30
MARGEN_R = 30
MARGEN_TOP = 74
MARGEN_BOTTOM = 44
FRAME_W = PAGE_W - MARGEN_L - MARGEN_R

Estilos = dict[str, Any]


def _pcell(texto: Any, estilo: Any) -> Paragraph:
    return Paragraph(str(texto), estilo)


def _nombre_cat(nombre: Any, categoria: Any, est: Estilos) -> Paragraph:
    nombre = "" if P.es_vacio(nombre) else str(nombre)
    categoria = "" if P.es_vacio(categoria) else str(categoria)
    return Paragraph(
        f"<b>{nombre}</b><br/><font color='{P.HEX_NAVY}' size=9>{categoria}</font>", est["celda"]
    )


def _tabla(headers: list[str], aligns: list[str], fracs: list[float], filas: list[list[Any]], est: Estilos):
    return P.tabla(headers, aligns, fracs, filas, est, FRAME_W)


def _tabla_top15_o_momentum(df: pd.DataFrame, est: Estilos, col_score: str, dec_score: int = 1):
    headers = ["#", "Ticker", "Nombre / Categoría", "5D", "1M", "6M", "1Y", "Sharpe", "Beta", "Score"]
    aligns = ["r", "l", "l", "r", "r", "r", "r", "r", "r", "r"]
    fracs = [0.03, 0.07, 0.32, 0.08, 0.08, 0.08, 0.08, 0.08, 0.07, 0.11]
    filas = []
    for i, r in enumerate(df.to_dict("records"), start=1):
        filas.append(
            [
                _pcell(i, est["celda_r"]),
                _pcell(r.get("Ticker", ""), est["celda_ticker"]),
                _nombre_cat(r.get("Nombre"), r.get("Category"), est),
                _pcell(P.fmt_pct(r.get("Performance 5D (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 1M (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 6M (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 1Y (%)")), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Sharpe Ratio"), 2), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Beta"), 2), est["celda_r"]),
                P.celda_acento(P.fmt_num(r.get(col_score), dec_score), est),
            ]
        )
    return _tabla(headers, aligns, fracs, filas, est)


def _tabla_top15_simple(df: pd.DataFrame, est: Estilos):
    headers = ["#", "Ticker", "Nombre / Categoría", "5D", "1M", "6M", "1Y", "Beta", "Score"]
    aligns = ["r", "l", "l", "r", "r", "r", "r", "r", "r"]
    fracs = [0.03, 0.08, 0.35, 0.09, 0.09, 0.09, 0.09, 0.08, 0.10]
    filas = []
    for i, r in enumerate(df.to_dict("records"), start=1):
        filas.append(
            [
                _pcell(i, est["celda_r"]),
                _pcell(r.get("Ticker", ""), est["celda_ticker"]),
                _nombre_cat(r.get("Nombre"), r.get("Category"), est),
                _pcell(P.fmt_pct(r.get("Performance 5D (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 1M (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 6M (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 1Y (%)")), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Beta"), 2), est["celda_r"]),
                P.celda_acento(P.fmt_num(r.get("Score_Simple"), 1), est),
            ]
        )
    return _tabla(headers, aligns, fracs, filas, est)


def _tabla_cap_resumen(df: pd.DataFrame, est: Estilos):
    headers = ["Bucket", "Seleccionados / Universo", "AUM representado (informativo)"]
    filas = [
        [
            _pcell(r.get("Cap_Bucket", ""), est["celda"]),
            _pcell(
                f"{P.fmt_num(r.get('N_Seleccionados'), 0)} de {P.fmt_num(r.get('N_Universo'), 0)}",
                est["celda_r"],
            ),
            _pcell(f"{P.fmt_num(r.get('Pct_AUM_Cubierto'), 1)}%", est["celda_r"]),
        ]
        for r in df.to_dict("records")
    ]
    return _tabla(headers, ["l", "r", "r"], [0.5, 0.25, 0.25], filas, est)


def _tabla_cap(df: pd.DataFrame, est: Estilos):
    headers = [
        "#",
        "Ticker",
        "Nombre / Categoría",
        "Bucket",
        "6M",
        "1Y",
        "Beta",
        "Alpha aprox.",
        "Score",
        "Peso",
    ]
    aligns = ["r", "l", "l", "l", "r", "r", "r", "r", "r", "r"]
    fracs = [0.03, 0.07, 0.27, 0.09, 0.08, 0.08, 0.07, 0.09, 0.10, 0.12]
    filas = []
    for i, r in enumerate(df.to_dict("records"), start=1):
        filas.append(
            [
                _pcell(i, est["celda_r"]),
                _pcell(r.get("Ticker", ""), est["celda_ticker"]),
                _nombre_cat(r.get("Nombre"), r.get("Category"), est),
                _pcell(r.get("Cap_Bucket", ""), est["celda"]),
                _pcell(P.fmt_pct(r.get("Performance 6M (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 1Y (%)")), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Beta"), 2), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Alpha_CAPM_aprox")), est["celda_r"]),
                P.celda_acento(P.fmt_num(r.get("Momentum_Score"), 1), est),
                _pcell(f"{P.fmt_num(r.get('Peso_Portfolio'), 2)}%", est["celda_r"]),
            ]
        )
    return _tabla(headers, aligns, fracs, filas, est)


def _tabla_excluidos(df: pd.DataFrame, est: Estilos):
    headers = [
        "Ticker",
        "Nombre / Categoría",
        "5D",
        "1M",
        "6M",
        "1Y",
        "Sharpe",
        "Beta",
        "Ventana atípica",
        "Z-score",
    ]
    aligns = ["l", "l", "r", "r", "r", "r", "r", "r", "l", "r"]
    fracs = [0.07, 0.28, 0.07, 0.07, 0.07, 0.07, 0.07, 0.06, 0.12, 0.12]
    filas = []
    for r in df.to_dict("records"):
        filas.append(
            [
                _pcell(r.get("Ticker", ""), est["celda_ticker"]),
                _nombre_cat(r.get("Nombre"), r.get("Category"), est),
                _pcell(P.fmt_pct(r.get("Performance 5D (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 1M (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 6M (%)")), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Performance 1Y (%)")), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Sharpe Ratio"), 2), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Beta"), 2), est["celda_r"]),
                _pcell(r.get("Ventana_Atipica", "—"), est["celda"]),
                P.celda_signo(r.get("Z_Score"), 2, est),
            ]
        )
    return _tabla(headers, aligns, fracs, filas, est)


def _tabla_diversificacion(df: pd.DataFrame, est: Estilos):
    filas = [
        [
            _pcell(r.get("Category", ""), est["celda"]),
            _pcell(P.fmt_num(r.get("Conteo"), 0), est["celda_r"]),
            Paragraph(f"<b>{P.fmt_num(r.get('Pct'), 1)}%</b>", est["celda_r"]),
        ]
        for r in df.to_dict("records")
    ]
    return _tabla(["Categoría", "N ETFs", "% del Top 15"], ["l", "r", "r"], [0.6, 0.2, 0.2], filas, est)


def _tabla_categorias(df: pd.DataFrame, est: Estilos):
    headers = [
        "#",
        "Categoría",
        "N",
        "Sharpe Prom.",
        "Treynor Med.",
        "Beta Prom.",
        "Perf. 1Y Prom.",
        "Perf. 6M Prom.",
        "Score Prom.",
        "% Pos. 1Y",
    ]
    aligns = ["r", "l", "r", "r", "r", "r", "r", "r", "r", "r"]
    fracs = [0.03, 0.27, 0.06, 0.09, 0.09, 0.08, 0.09, 0.09, 0.09, 0.11]
    filas = []
    for i, r in enumerate(df.to_dict("records"), start=1):
        filas.append(
            [
                _pcell(i, est["celda_r"]),
                _pcell(r.get("Category", ""), est["celda"]),
                _pcell(P.fmt_num(r.get("N"), 0), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Sharpe_Promedio"), 2), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Treynor_Mediana"), 2), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Beta_Promedio"), 2), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Perf_1Y_Promedio"), 1), est["celda_r"]),
                _pcell(P.fmt_pct(r.get("Perf_6M_Promedio"), 1), est["celda_r"]),
                P.celda_acento(P.fmt_num(r.get("Score_Promedio"), 1), est),
                _pcell(f"{P.fmt_num(r.get('Pct_Positivos_1Y'), 0)}%", est["celda_r"]),
            ]
        )
    return _tabla(headers, aligns, fracs, filas, est)


def _tabla_mantienen(df: pd.DataFrame, est: Estilos):
    headers = ["Ticker", "Nombre / Categoría", "# Act.", "# Prev.", "Racha", "Score", "ΔScore", "ΔRank"]
    aligns = ["l", "l", "r", "r", "r", "r", "r", "r"]
    fracs = [0.08, 0.34, 0.09, 0.09, 0.09, 0.10, 0.10, 0.11]
    filas = []
    for r in df.to_dict("records"):
        filas.append(
            [
                _pcell(r.get("Ticker", ""), est["celda_ticker"]),
                _nombre_cat(r.get("Nombre"), r.get("Category"), est),
                _pcell(P.fmt_num(r.get("Rank_actual"), 0), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Rank_previo"), 0), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Racha"), 0), est["celda_r"]),
                _pcell(P.fmt_num(r.get("Score"), 1), est["celda_r"]),
                P.celda_signo(r.get("Delta_Score"), 1, est),
                P.celda_signo(r.get("Delta_Rank"), 0, est),
            ]
        )
    return _tabla(headers, aligns, fracs, filas, est)


def _tabla_nuevos_o_salidas(df: pd.DataFrame, est: Estilos, col_rank: str, col_score: str, label_rank: str):
    headers = ["Ticker", "Nombre / Categoría", label_rank, "Score", "Detalle"]
    aligns = ["l", "l", "r", "r", "l"]
    fracs = [0.08, 0.24, 0.08, 0.09, 0.51]
    filas = []
    for r in df.to_dict("records"):
        detalle = r.get("Procedencia") if "Procedencia" in r else r.get("Destino", "")
        filas.append(
            [
                _pcell(r.get("Ticker", ""), est["celda_ticker"]),
                _nombre_cat(r.get("Nombre"), r.get("Category"), est),
                _pcell(P.fmt_num(r.get(col_rank), 0), est["celda_r"]),
                _pcell(P.fmt_num(r.get(col_score), 1), est["celda_r"]),
                _pcell(detalle, est["celda"]),
            ]
        )
    return _tabla(headers, aligns, fracs, filas, est)


def _vacio(df: pd.DataFrame | None) -> bool:
    return df is None or len(df) == 0


def _construir_story(datos: dict[str, Any]) -> list[Any]:
    est = P.estilos()
    story: list[Any] = []

    story.append(Paragraph("TOP 15 RECOMENDADOS", est["seccion"]))
    story.append(_tabla_top15_o_momentum(datos["top15"], est, col_score="Score"))
    story.append(Spacer(1, 14))

    story.append(Paragraph("EXCLUIDOS POR DISTORSIÓN ESTADÍSTICA", est["seccion"]))
    excl = datos["excluidos"]
    if not _vacio(excl):
        story.append(
            Paragraph(
                "La exclusión responde a una distorsión estadística frente a un universo diversificado "
                "(|z-score MAD| &gt; 6) y no implica necesariamente mala calidad del fondo.",
                est["nota"],
            )
        )
        story.append(Spacer(1, 4))
        story.append(_tabla_excluidos(excl, est))
    else:
        story.append(Paragraph("Ningún ETF fue excluido por distorsión en esta corrida.", est["nota"]))

    story.append(PageBreak())

    story.append(Paragraph("ROTACIÓN VS. CORRIDA ANTERIOR", est["seccion"]))
    rot = datos["rotacion"]
    if rot is None:
        story.append(
            Paragraph(
                "No hay todavía una corrida archivada para comparar. Esta sección se completa "
                "automáticamente en la próxima corrida.",
                est["nota"],
            )
        )
    else:
        mant, nuevos, salidas = rot["mantienen"], rot["nuevos"], rot["salidas"]
        story.append(Paragraph(f"Se mantienen en el Top 15 ({len(mant)})", est["badge_verde"]))
        if len(mant):
            story.append(_tabla_mantienen(mant, est))
        else:
            story.append(Paragraph("Ningún ETF se mantuvo (rotación total).", est["nota"]))

        story.append(Paragraph(f"Nuevos ingresos ({len(nuevos)})", est["badge_azul"]))
        if len(nuevos):
            story.append(_tabla_nuevos_o_salidas(nuevos, est, "Rank_actual", "Score", "# Act."))
        else:
            story.append(Paragraph("No hubo nuevos ingresos en esta corrida.", est["nota"]))

        story.append(Paragraph(f"Salidas del Top 15 ({len(salidas)})", est["badge_rojo"]))
        if len(salidas):
            story.append(_tabla_nuevos_o_salidas(salidas, est, "Rank_previo", "Score_previo", "# Prev."))
        else:
            story.append(Paragraph("No hubo salidas en esta corrida.", est["nota"]))

    story.append(PageBreak())

    story.append(Paragraph("DIVERSIFICACIÓN DEL TOP 15", est["seccion"]))
    story.append(_tabla_diversificacion(datos["diversificacion"], est))
    story.append(Spacer(1, 14))

    story.append(Paragraph("RANKING POR CATEGORÍA", est["seccion"]))
    cats = datos["categorias"]
    if not _vacio(cats):
        story.append(
            Paragraph(
                "Rankea categorías completas (no ETFs individuales) dentro del universo ya depurado de distorsiones.",
                est["nota"],
            )
        )
        story.append(Spacer(1, 4))
        story.append(_tabla_categorias(cats, est))
    else:
        story.append(Paragraph("El análisis de Categoría no se corrió en esta corrida.", est["nota"]))

    story.append(PageBreak())

    story.append(Paragraph("TOP 15 MOMENTUM / CRECIMIENTO", est["seccion"]))
    mom = datos["momentum"]
    if not _vacio(mom):
        story.append(
            Paragraph(
                "Análisis complementario al ranking principal: en vez de minimizar riesgo y descartar "
                "distorsiones, busca momentum de rendimiento.",
                est["nota"],
            )
        )
        story.append(Spacer(1, 4))
        story.append(_tabla_top15_o_momentum(mom, est, col_score="Score_comp"))
    else:
        story.append(
            Paragraph("El análisis de Momentum/Crecimiento no se corrió en esta corrida.", est["nota"])
        )

    story.append(PageBreak())

    story.append(Paragraph("TOP 15 (PERFORMANCE + BETA)", est["seccion"]))
    simple = datos["top15_simple"]
    if not _vacio(simple):
        story.append(
            Paragraph(
                "Ranking considerando únicamente el rendimiento por período (5D, 1M, 6M, 1Y) y el Beta.",
                est["nota"],
            )
        )
        story.append(Spacer(1, 4))
        story.append(_tabla_top15_simple(simple, est))
    else:
        story.append(Paragraph("Este ranking alternativo no se corrió en esta corrida.", est["nota"]))

    story.append(PageBreak())

    story.append(Paragraph("TOP 15 CAP.", est["seccion"]))
    cap, cap_resumen = datos["russell3000"], datos["russell3000_resumen"]
    if not _vacio(cap):
        story.append(
            Paragraph(
                "Estrategia tipo Vanguard U.S. Momentum Factor ETF (VFMO), aplicada sobre ETFs en vez de "
                "acciones individuales del Russell 3000. Los 5 ETFs de mayor Score de Momentum de cada bucket "
                "de capitalización (Large/Mid/Small), 15 en total, ponderados proporcional al Score.",
                est["nota"],
            )
        )
        story.append(Spacer(1, 4))
        if not _vacio(cap_resumen):
            story.append(_tabla_cap_resumen(cap_resumen, est))
            story.append(Spacer(1, 10))
        story.append(_tabla_cap(cap, est))
    else:
        story.append(Paragraph("El análisis Top 15 Cap. no se corrió en esta corrida.", est["nota"]))

    return story


def generar_pdf(data_dir: Path, data_hist_dir: Path) -> tuple[bytes, str]:
    """Arma el PDF a partir de la última corrida en ``data_dir``.

    Devuelve ``(bytes, fecha_corrida "dd/mm/aaaa")``. Lanza ``FileNotFoundError`` si
    todavía no hay corrida procesada.
    """

    def _leer(nombre: str) -> pd.DataFrame | None:
        ruta = data_dir / nombre
        if not ruta.exists():
            return None
        try:
            return pd.read_csv(ruta)
        except Exception:
            return None

    top15 = _leer("etf_top15_recomendados.csv")
    if top15 is None or top15.empty:
        raise FileNotFoundError("Todavía no hay ninguna corrida procesada.")

    excluidos = _leer("etf_excluidos_outliers.csv")
    ranking_completo = _leer("etf_ranking_completo.csv")
    categorias = _leer("etf_categorias_ranking.csv")

    rotacion = None
    if excluidos is not None and ranking_completo is not None:
        rotacion = rotacion_mod.generar_datos_rotacion(
            str(data_dir), str(data_hist_dir), top15, excluidos, ranking_completo
        )

    fecha_corrida = (
        P.fmt_fecha_yyyymmdd(top15["Fecha"].iloc[0]) if "Fecha" in top15.columns and len(top15) else "—"
    )

    datos = {
        "top15": top15,
        "excluidos": excluidos,
        "categorias": categorias.head(15) if categorias is not None else None,
        "momentum": _leer("etf_momentum_top15_score_comp.csv"),
        "diversificacion": rotacion_mod.diversificacion_top15(top15),
        "rotacion": rotacion,
        "top15_simple": _leer("etf_top15_simple.csv"),
        "russell3000": _leer("etf_russell3000_portfolio.csv"),
        "russell3000_resumen": _leer("etf_russell3000_resumen_buckets.csv"),
    }

    buffer = io.BytesIO()
    doc = SimpleDocTemplate(
        buffer,
        pagesize=PAGE_SIZE,
        leftMargin=MARGEN_L,
        rightMargin=MARGEN_R,
        topMargin=MARGEN_TOP,
        bottomMargin=MARGEN_BOTTOM,
        title="Fog Radar Quant - ETFs - Informe Ejecutivo",
        author="Fog Radar Quant",
    )
    canvasmaker = P.canvas_con_marco(
        "ETFs — Informe Ejecutivo",
        f"Corrida: {fecha_corrida}",
        PAGE_SIZE,
        margen_l=MARGEN_L,
        margen_r=MARGEN_R,
    )
    doc.build(_construir_story(datos), canvasmaker=canvasmaker)
    return buffer.getvalue(), fecha_corrida
