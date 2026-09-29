"""
Lectura de los archivos que deja una corrida en output/<YYYYMMDD>_<NN>/
(los que escriben modeling.markowitz, trend_forecast y momentum),
combinados por análisis (ver settings.ANALYSES).

Es la única capa que conoce los nombres de esos archivos del lado de
lectura: la usan tanto reporting.pdf_report como reporting.dashboard_data
(y, a través de este último, backfill_history.py). Vive aparte de
pdf_report para que exportar el JSON del dashboard no dependa de
reportlab/matplotlib. Agregar un artefacto nuevo a una corrida es:
escribirlo en modeling/*, leerlo acá (un campo más en AnalysisData) y
exponerlo en dashboard_data/pdf_report.
"""

import json
import os
from dataclasses import dataclass, field

import pandas as pd

from fog_radar.modules.tendencia import config


@dataclass
class AnalysisData:
    """Todo lo de un análisis (p. ej. "Asia + EEUU" o "solo EEUU") en una corrida."""

    perf: dict  # portfolio_performance<key>.json
    merged: pd.DataFrame  # pesos finales + proyección, indexado por ticker
    notes: list  # notas de régimen de mercado (macro, compartidas)
    momentum: pd.DataFrame  # top momentum mensual (puede estar vacío)
    momentum_weekly: pd.DataFrame = field(default_factory=pd.DataFrame)
    # [{"n", "performance", "holdings": DataFrame}] — ver load_reduced_portfolios
    reduced_portfolios: list = field(default_factory=list)


def _read_csv_or_empty(path, **kwargs):
    """CSV -> DataFrame, o DataFrame vacío si no existe o no tiene datos
    (p. ej. un ranking de momentum sin tickers con historia suficiente)."""
    if not os.path.exists(path) or os.path.getsize(path) == 0:
        return pd.DataFrame()
    try:
        return pd.read_csv(path, **kwargs)
    except pd.errors.EmptyDataError:
        return pd.DataFrame()


def _read_notes(run_dir):
    # Las notas de régimen de mercado son macro, no dependen del universo
    # (Asia+EEUU vs. solo EEUU): siempre se leen del archivo sin sufijo.
    path = os.path.join(run_dir, "market_regime_notes.txt")
    if not os.path.exists(path):
        return []
    with open(path, encoding="utf-8") as f:
        return [line.strip() for line in f if line.strip()]


def _with_sector(df):
    # El sector se toma siempre de config.SECTOR_MAP (no de los CSV) para
    # no depender de que cada paso se haya vuelto a correr con la columna
    # "sector" ya incluida.
    df["sector"] = [config.SECTOR_MAP.get(t, "Otro") for t in df.index]
    return df.sort_values("peso_final", ascending=False)


def load_reduced_portfolios(run_dir, suffix, merged):
    """
    Portafolios reducidos (config.PORTFOLIO_SIZE_OPTIONS, ver
    modeling.markowitz.run_reduced_portfolios). Las columnas de tendencia
    (retorno esperado, señales) se toman de `merged` (el portafolio de
    top_n, que ya las tiene para todas las seleccionadas). Omite los N que
    no tengan archivo (corridas anteriores a esta funcionalidad).
    """
    options = []
    trend_cols = merged.drop(columns=["peso_final", "nombre", "pais", "sector"], errors="ignore")
    for n in config.PORTFOLIO_SIZE_OPTIONS:
        weights_path = os.path.join(run_dir, f"portfolio_final_weights_top{n}{suffix}.csv")
        perf_path = os.path.join(run_dir, f"portfolio_performance_top{n}{suffix}.json")
        if not (os.path.exists(weights_path) and os.path.exists(perf_path)):
            continue
        weights = pd.read_csv(weights_path, index_col=0).join(trend_cols, how="left")
        with open(perf_path, encoding="utf-8") as f:
            perf = json.load(f)
        options.append({"n": n, "performance": perf, "holdings": _with_sector(weights)})
    return options


def load_analysis(run_dir=None, suffix=""):
    """
    Carga los archivos de un análisis (universo combinado si suffix="",
    o un complementario como "solo EEUU" si suffix="_us" — ver
    modeling.markowitz.run_selection). Con suffix no vacío, si
    portfolio_performance<suffix>.json no existe (el análisis se omitió
    por falta de tickers) devuelve None en vez de fallar, para que el
    llamador pueda omitir esa sección.
    """
    run_dir = run_dir or config.get_run_dir()

    perf_path = os.path.join(run_dir, f"portfolio_performance{suffix}.json")
    if suffix and not os.path.exists(perf_path):
        return None
    with open(perf_path, encoding="utf-8") as f:
        perf = json.load(f)

    weights = pd.read_csv(os.path.join(run_dir, f"portfolio_final_weights{suffix}.csv"), index_col=0)
    forecast = pd.read_csv(os.path.join(run_dir, f"trend_forecast_6m{suffix}.csv"), index_col="ticker")
    forecast = forecast.drop(columns=["nombre", "pais", "sector"], errors="ignore")
    merged = _with_sector(weights.join(forecast, how="left"))

    return AnalysisData(
        perf=perf,
        merged=merged,
        notes=_read_notes(run_dir),
        momentum=_read_csv_or_empty(os.path.join(run_dir, f"top_momentum_mensual{suffix}.csv")),
        momentum_weekly=_read_csv_or_empty(os.path.join(run_dir, f"top_momentum_semanal{suffix}.csv")),
        reduced_portfolios=load_reduced_portfolios(run_dir, suffix, merged),
    )


def compute_sector_summary(merged):
    grp = merged.groupby("sector").agg(
        n_acciones=("peso_final", "count"),
        peso_total=("peso_final", "sum"),
        retorno_6m_prom=("retorno_esperado_6m_pct", "mean"),
    )
    signal_mode = merged.groupby("sector")["senal_tecnica"].agg(lambda s: s.value_counts().idxmax())
    grp["senal_predominante"] = signal_mode
    # Se arma como Series (ticker = índice y valor) y se agrupa por la
    # columna "sector" en vez de usar DataFrameGroupBy.apply sobre "merged"
    # directamente: así se evita el FutureWarning de pandas sobre operar
    # con la propia columna de agrupación (y no depende de "include_groups",
    # un parámetro que no existe en versiones de pandas anteriores a 2.2).
    ticker_series = pd.Series(merged.index, index=merged.index)
    tickers_por_sector = ticker_series.groupby(merged["sector"]).agg(lambda s: ", ".join(s))
    grp["tickers"] = tickers_por_sector
    grp = grp.sort_values("retorno_6m_prom", ascending=False)
    return grp
