"""
Ranking por Categoria (version WEB) - adaptado de etf_categoria_analysis.py
de la carpeta raiz del proyecto ETF_Choice. Misma metodologia (ver CLAUDE.md
seccion 2quater): agrupa etf_ranking_completo.csv (universo depurado, salida
de etf_ranking_analysis.py) por Category, descarta categorias con N<5 (sin
generar una tabla aparte para ellas) y rankea el resto por Sharpe Ratio
promedio (Treynor por mediana, por el mismo motivo documentado en la
version raiz).

Diferencia para uso desde la API: expone run(ranking_path) -> dict con los
DataFrames resultantes, sin generar grafico PNG.
"""

import os
import re

import pandas as pd

MIN_N_CATEGORIA = 5

# Categorias excluidas del ranking por decision explicita del usuario
# (2026-07-18, ampliado 2026-07-21, ampliado 2026-07-24) -- mismo criterio
# y motivo que la version raiz (ver CLAUDE.md seccion 2quater): Ultrashort
# Bond y Securitized Bond - Focused lideran por Sharpe promedio pero son el
# artefacto de Beta~=0 (CLOA/CLOI/CMBS con Beta 0.01-0.03): Sharpe alto
# porque el denominador (volatilidad) es minusculo, no porque el retorno
# absoluto sea bueno (1Y entre 3% y 5.25%, muy por debajo de las
# categorias que si entran al Top 15 individual). Target Maturity agregada
# 2026-07-24 a pedido explicito del usuario ("no me aportan nada"), mismo
# criterio de fondo. No es un problema de tamano de muestra, es una
# decision de curacion: no participan del ranking.
CATEGORIAS_EXCLUIR_RANKING = ["Ultrashort Bond", "Securitized Bond - Focused", "Target Maturity"]

# Nota 2026-07-24 (mejoras pendientes de WEB/, item 31): desde hoy
# etf_ranking_analysis.py tiene su propio CATEGORIAS_EXCLUIR con estas
# mismas 3 categorias, aplicado ANTES de escribir etf_ranking_completo.csv
# -- que es exactamente el archivo que este script lee como "ranking_path"
# en run(). En la practica eso vuelve al filtro de aca abajo un no-op: ya
# nunca va a encontrar ninguna de las 3 categorias para sacar, porque
# etf_ranking_analysis.py las saco rio arriba. Se deja el filtro a
# proposito como defensa en profundidad (si el dia de manana alguien toca
# CATEGORIAS_EXCLUIR de etf_ranking_analysis.py y deja de excluirlas ahi,
# este script las sigue excluyendo igual del ranking por categoria) -- no
# por descuido ni porque haga falta hoy.


def run(ranking_path):
    ranking = pd.read_csv(ranking_path)
    carpeta = os.path.dirname(os.path.abspath(ranking_path))

    if "Fecha" in ranking.columns and len(ranking):
        fecha_tag = str(ranking["Fecha"].iloc[0]).replace("-", "")
    else:
        csvs = sorted([f for f in os.listdir(carpeta) if re.match(r"etf_perf_data_\d{8}\.csv", f)])
        m = re.search(r"(\d{8})", csvs[-1]) if csvs else None
        fecha_tag = m.group(0) if m else "SINFECHA"

    ranking_cat = ranking[~ranking["Category"].isin(CATEGORIAS_EXCLUIR_RANKING)]

    conteos = ranking_cat["Category"].value_counts()
    categorias_validas = conteos[conteos >= MIN_N_CATEGORIA].index

    universo_valido = ranking_cat[ranking_cat["Category"].isin(categorias_validas)]
    agg = (
        universo_valido.groupby("Category")
        .agg(
            N=("Ticker", "count"),
            Sharpe_Promedio=("Sharpe Ratio", "mean"),
            Treynor_Mediana=("Treynor Ratio", "median"),
            Beta_Promedio=("Beta", "mean"),
            Perf_1Y_Promedio=("Performance 1Y (%)", "mean"),
            Perf_6M_Promedio=("Performance 6M (%)", "mean"),
            Score_Promedio=("Score", "mean"),
            Pct_Positivos_1Y=("Performance 1Y (%)", lambda s: (s > 0).mean() * 100),
            # Lista de tickers de la categoria, ordenados por Sharpe descendente,
            # para el popup de la columna N en el panel (hover sobre el numero).
            Tickers=(
                "Ticker",
                lambda s: ", ".join(
                    universo_valido.loc[s.index].sort_values("Sharpe Ratio", ascending=False)["Ticker"]
                ),
            ),
        )
        .reset_index()
    )

    for col in [
        "Sharpe_Promedio",
        "Treynor_Mediana",
        "Beta_Promedio",
        "Perf_1Y_Promedio",
        "Perf_6M_Promedio",
        "Score_Promedio",
        "Pct_Positivos_1Y",
    ]:
        agg[col] = agg[col].round(3)

    agg = agg.sort_values("Sharpe_Promedio", ascending=False).reset_index(drop=True)
    agg["Fecha"] = fecha_tag

    ranking_out = os.path.join(carpeta, "etf_categorias_ranking.csv")
    agg.to_csv(ranking_out, index=False)

    return {"fecha_tag": fecha_tag, "ranking": agg}
