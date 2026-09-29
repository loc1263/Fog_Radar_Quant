"""
Analisis Momentum/Crecimiento (version WEB) - adaptado de
etf_momentum_analysis.py de la carpeta raiz del proyecto ETF_Choice.

Misma metodologia y pesos que la version raiz (ver CLAUDE.md seccion 2ter).
Diferencias para uso desde la API:
  - Expone run(ruta_csv) -> dict con los DataFrames resultantes.
  - Sin generacion de grafico PNG (no requiere matplotlib).
  - La "corrida historica" para el componente de Consistencia se busca en
    WEB/backend/data_hist (equivalente a old/ en la version raiz), no en
    una carpeta "old" anidada dentro de data/.
"""

import glob
import os
import re

import numpy as np
import pandas as pd

from fog_radar.core.logging import get_logger

from .config import COL_1M, COL_1Y, COL_5D, COL_6M, COL_BETA, COL_SHARPE, COL_TREYNOR, TOP_N

log = get_logger("etf.analysis")

REQUIRED_COLS = [COL_5D, COL_1M, COL_6M, COL_1Y, COL_SHARPE, COL_TREYNOR, COL_BETA]

PESO_M_5D = 0.10
PESO_M_1M = 0.20
PESO_M_6M = 0.30
PESO_M_1Y = 0.40

CATEGORIAS_EXCLUIR = [
    "Ultrashort Bond",
    "Securitized Bond - Focused",
    "Short-Term Bond",
    "Short Government",
    "Short-Term Inflation-Protected Bond",
    "Muni National Short",
    "Muni Single State Short",
    "Money Market-Taxable",
    "Prime Money Market",
    "Target Maturity",
    "Muni Target Maturity",
    "Corporate Target Maturity",
    "Target-Date 2030",
    "Target-Date 2035",
    "Target-Date 2040",
    "Target-Date 2045",
    "Target-Date 2050",
    "Target-Date 2055",
    "Target-Date 2060",
    "Target-Date 2065",
    "Target-Date 2070+",
    "Target-Date Retirement",
]


def encontrar_csv_mas_reciente(carpeta):
    candidatos = sorted(glob.glob(os.path.join(carpeta, "etf_perf_data_*.csv")))
    if not candidatos:
        raise FileNotFoundError(f"No se encontro ningun archivo etf_perf_data_*.csv en {carpeta}")
    return candidatos[-1]


def minmax_0_100(series: pd.Series) -> pd.Series:
    clean = series.copy().replace([np.inf, -np.inf], np.nan)
    min_value, max_value = clean.min(), clean.max()
    if pd.isna(min_value) or pd.isna(max_value) or max_value == min_value:
        return pd.Series(50.0, index=clean.index)
    return 100 * (clean - min_value) / (max_value - min_value)


def cargar_perf_6m_historico(data_hist_dir):
    """Version WEB de cargar_perf_6m_historico: busca en backend/data_hist
    (en vez de <carpeta>/old) los etf_perf_data_*.csv archivados."""
    candidatos = sorted(glob.glob(os.path.join(data_hist_dir, "etf_perf_data_*.csv")))
    if not candidatos:
        return None, None

    m = re.search(r"(\d{8})", os.path.basename(candidatos[0]))
    fecha_mas_antigua = m.group(1) if m else None
    ruta_mas_antigua = candidatos[0]

    df_hist = pd.read_csv(ruta_mas_antigua, encoding="utf-8-sig")
    df_hist = df_hist[df_hist["TipoInstrumento"] == "ETF"].copy()
    df_hist[COL_6M] = pd.to_numeric(df_hist[COL_6M], errors="coerce")
    serie = df_hist.set_index("Ticker")[COL_6M]
    serie = serie[~serie.index.duplicated(keep="first")]
    return fecha_mas_antigua, serie


def run(ruta_csv, data_hist_dir=None):
    log.info(f"Leyendo: {ruta_csv}")
    df = pd.read_csv(ruta_csv, encoding="utf-8-sig")
    carpeta_salida = os.path.dirname(os.path.abspath(ruta_csv))
    if data_hist_dir is None:
        data_hist_dir = os.path.join(os.path.dirname(carpeta_salida), "data_hist")

    universo = df[df["TipoInstrumento"] == "ETF"].copy()

    universo = universo[~universo["Category"].isin(CATEGORIAS_EXCLUIR)].copy()
    universo = universo.dropna(subset=REQUIRED_COLS).copy()

    for col in REQUIRED_COLS + ["Precio", "AUM", "52W High", "52W Low"]:
        if col in universo.columns:
            universo[col] = pd.to_numeric(universo[col], errors="coerce")

    universo["M_raw"] = (
        PESO_M_5D * universo[COL_5D]
        + PESO_M_1M * universo[COL_1M]
        + PESO_M_6M * universo[COL_6M]
        + PESO_M_1Y * universo[COL_1Y]
    )
    universo["M_norm"] = minmax_0_100(universo["M_raw"])

    treynor = universo[COL_TREYNOR].replace([np.inf, -np.inf], np.nan)
    q_low, q_high = treynor.quantile(0.01), treynor.quantile(0.99)
    universo["Treynor_cap"] = treynor.clip(q_low, q_high)
    universo["Q_raw"] = 0.5 * universo[COL_SHARPE] + 0.5 * universo["Treynor_cap"]
    universo["Q_norm"] = minmax_0_100(universo["Q_raw"])

    fecha_antigua, perf6m_antiguo = cargar_perf_6m_historico(data_hist_dir)
    if perf6m_antiguo is not None:
        consist_raw = universo.set_index("Ticker")[COL_6M] - perf6m_antiguo
        consist_raw = consist_raw.reindex(universo["Ticker"]).values
        universo["Consist_raw"] = consist_raw
        universo["Consist_norm"] = minmax_0_100(universo["Consist_raw"]).fillna(50.0)
    else:
        universo["Consist_norm"] = 50.0

    universo["Score_comp"] = (
        0.55 * universo["M_norm"] + 0.25 * universo["Q_norm"] + 0.20 * universo["Consist_norm"]
    )

    m_fecha = re.search(r"(\d{8})", os.path.basename(ruta_csv))
    fecha_tag = m_fecha.group(0) if m_fecha else "SINFECHA"
    universo["Fecha"] = fecha_tag

    cols_out = [
        "Ticker",
        "Nombre",
        "Category",
        COL_5D,
        COL_1M,
        COL_6M,
        COL_1Y,
        COL_SHARPE,
        COL_TREYNOR,
        COL_BETA,
        "M_norm",
        "Q_norm",
        "Consist_norm",
        "Score_comp",
        "Fecha",
        # Precio/52W High/52W Low agregados 2026-08-03 para que el
        # frontend pueda dibujar la barra de posicion dentro del rango de
        # 52 semanas (que tan cerca del maximo/minimo esta el ETF) en la
        # tabla Top 15 Momentum, igual que ya se hace en Top 15 principal
        # (esos 3 campos ya viajan ahi porque etf_ranking_analysis.py no
        # recorta columnas). A proposito NO se agregaron a REQUIRED_COLS:
        # son solo para mostrar, no deben filtrar el universo momentum.
        "Precio",
        "52W High",
        "52W Low",
    ]
    cols_out = [c for c in cols_out if c in universo.columns]

    ranking_comp = universo.sort_values("Score_comp", ascending=False).reset_index(drop=True)
    top15_comp = ranking_comp.head(TOP_N).copy()

    ranking_path = os.path.join(carpeta_salida, "etf_momentum_ranking_completo.csv")
    top15_comp_path = os.path.join(carpeta_salida, "etf_momentum_top15_score_comp.csv")

    ranking_comp[cols_out].to_csv(ranking_path, index=False)
    top15_comp[cols_out].to_csv(top15_comp_path, index=False)

    return {
        "fecha_tag": fecha_tag,
        "ranking": ranking_comp[cols_out],
        "top15": top15_comp[cols_out],
    }
