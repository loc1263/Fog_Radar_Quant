"""
Selección de las 15 mejores acciones vía modelo de Markowitz
(media-varianza / covarianza).

Metodología:
  1. Con los retornos semanales de TODO el universo (Asia + EEUU) se estiman
     los retornos esperados anualizados (mu) y la matriz de covarianza
     anualizada (S).
  2. Se optimiza el portafolio de máximo Sharpe Ratio sobre el universo
     completo, con:
       - Restricción long-only (no ventas en corto): peso >= 0
       - Tope de concentración por activo (MAX_WEIGHT_PER_ASSET)
       - Regularización L2 (objective_functions.L2_reg) para evitar que la
         optimización colapse en 2-3 activos "de esquina" (comportamiento
         típico de Markowitz sin regularizar) y así reflejar mejor la
         diversificación real del universo.
  3. Se ordenan los activos por el peso que les asignó el optimizador y se
     seleccionan los 15 con mayor peso -> "las 15 mejores acciones" según
     el modelo de covarianza/rendimiento.
  4. Con esas 15 acciones se vuelve a correr la optimización de máximo
     Sharpe (sin el tope de concentración del universo amplio, para permitir
     una asignación más decidida) -> portafolio final recomendado.

Corre este modelo una vez por cada análisis configurado en
config.ANALYSES (ver settings.py): el principal, sobre el universo
combinado Asia + EEUU, y cualquier análisis complementario (p. ej. "solo
EEUU") — cada uno se re-optimiza desde cero sobre su propio universo
(nunca es un filtro sobre los resultados de otro análisis) y escribe los
mismos 4 archivos con el sufijo de archivo (`key`) de ese análisis. Un
análisis se omite si tiene menos tickers con datos disponibles que su
`min_tickers`. Agregar un análisis nuevo es agregar una entrada a
config.ANALYSES, no tocar este módulo.

Ejecutar (vía wrapper de compatibilidad en la raíz del proyecto):
    python step04_markowitz_selection.py

Genera, dentro de la carpeta de la corrida actual (output/<YYYYMMDD>_<NN>,
ver tendencia.config.get_run_dir):
    markowitz_universe_weights<key>.csv   (pesos de TODO el universo de ese análisis)
    top15_seleccion<key>.csv              (las top_n acciones seleccionadas)
    portfolio_final_weights<key>.csv      (pesos finales sobre esa selección)
    portfolio_performance<key>.json       (retorno/riesgo/Sharpe esperado)
    portfolio_final_weights_top<N><key>.csv / portfolio_performance_top<N><key>.json
        (portafolios reducidos a N = config.PORTFOLIO_SIZE_OPTIONS, re-optimizados)
donde <key> es "" para el análisis principal y, p. ej., "_us" para el
complementario "solo EEUU" (ver config.ANALYSES).
"""

import json
import os

import numpy as np
import pandas as pd

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.ingestion.build_dataset import load_prices_usd
from fog_radar.modules.tendencia.logging_utils import get_logger

logger = get_logger("step04", config.LOGS_DIR, log_filename="step04_markowitz.log")


def load_inputs():
    prices = load_prices_usd()
    rf_path = os.path.join(config.PROCESSED_DIR, "risk_free_rate.txt")
    with open(rf_path) as f:
        rf = float(f.read().strip())
    return prices, rf


def optimize_max_sharpe(prices, rf, weight_bounds, l2_gamma):
    from pypfopt import EfficientFrontier, expected_returns, objective_functions, risk_models

    mu = expected_returns.mean_historical_return(prices, frequency=config.WEEKS_PER_YEAR)
    S = risk_models.sample_cov(prices, frequency=config.WEEKS_PER_YEAR)
    # Shrinkage ligero implícito vía CovarianceShrinkage sería ideal con más
    # historia; con 5 años semanales (~260 obs) sample_cov ya es razonablemente
    # estable, se deja sample_cov para transparencia del cálculo.

    ef = EfficientFrontier(mu, S, weight_bounds=weight_bounds)
    if l2_gamma:
        ef.add_objective(objective_functions.L2_reg, gamma=l2_gamma)

    try:
        ef.max_sharpe(risk_free_rate=rf)
    except Exception as e:
        logger.warning(f"max_sharpe falló ({e}); se intenta min_volatility como respaldo.")
        ef = EfficientFrontier(mu, S, weight_bounds=weight_bounds)
        ef.min_volatility()

    weights = ef.clean_weights()
    perf = ef.portfolio_performance(risk_free_rate=rf, verbose=False)
    return weights, perf, mu, S


def select_top_n(weights_full_series, mu, S, rf, top_n):
    """
    Devuelve la lista de tickers seleccionados (top_n) a partir de los pesos
    óptimos del universo completo. Si el optimizador dejó menos de `top_n`
    activos con peso significativo, completa con los de mayor Sharpe
    individual entre los restantes. Extraída como función independiente
    (antes vivía inline en main()) para poder testearla sin depender de
    archivos en disco.
    """
    top_tickers = weights_full_series[weights_full_series > 1e-6].head(top_n).index.tolist()
    if len(top_tickers) < top_n:
        variances = pd.Series(np.diag(S.loc[mu.index, mu.index]), index=mu.index)
        individual_sharpe = (mu - rf) / variances.pow(0.5)
        remaining = [t for t in individual_sharpe.sort_values(ascending=False).index if t not in top_tickers]
        top_tickers += remaining[: top_n - len(top_tickers)]
    return top_tickers


def optimize_final_portfolio(prices_top, rf, universe_tickers):
    """
    Máx. Sharpe sobre un grupo chico ya seleccionado, sin el tope de
    concentración del universo amplio (para permitir una asignación más
    decidida). El tope por activo es 30%, o 1/N si con 30% no alcanza a
    sumar 100%. Devuelve (DataFrame peso_final/nombre/pais, performance).
    """
    max_weight = max(0.30, 1 / prices_top.shape[1])
    weights, perf, _mu, _S = optimize_max_sharpe(
        prices_top,
        rf,
        weight_bounds=(0, max_weight),
        l2_gamma=0.05,
    )
    weights_series = pd.Series(weights).sort_values(ascending=False)
    weights_df = weights_series.rename("peso_final").to_frame()
    weights_df["nombre"] = [universe_tickers[t][0] for t in weights_df.index]
    weights_df["pais"] = [universe_tickers[t][1] for t in weights_df.index]
    return weights_df, perf


def run_selection(prices, rf, run_dir, universe_tickers, top_n, out_suffix="", label="universo completo"):
    """
    Corre la selección de Markowitz completa (optimización sobre
    `universe_tickers` + portafolio final concentrado en las `top_n`
    mejores) y escribe los 4 archivos de salida de siempre, con
    `out_suffix` agregado al nombre (p. ej. "_us" para el análisis
    restringido a EEUU). Extraída de main() para poder correr el mismo
    análisis sobre distintos universos (combinado Asia+EEUU, o solo EEUU)
    sin duplicar la lógica de optimización.

    `universe_tickers` es un dict ticker -> (nombre, país, moneda) —
    típicamente config.ALL_TICKERS o config.US_TICKERS — usado tanto para
    filtrar las columnas de `prices` como para las columnas nombre/país de
    los CSV de salida.
    """
    universe_cols = [t for t in prices.columns if t in universe_tickers]
    prices_universe = prices[universe_cols]

    # --- optimización sobre el universo dado ---
    weights_full, perf_full, mu, S = optimize_max_sharpe(
        prices_universe,
        rf,
        weight_bounds=(0, config.MAX_WEIGHT_PER_ASSET),
        l2_gamma=config.L2_GAMMA,
    )

    weights_full_series = pd.Series(weights_full).sort_values(ascending=False)
    weights_full_df = weights_full_series.rename("peso_markowitz").to_frame()
    weights_full_df["nombre"] = [universe_tickers[t][0] for t in weights_full_df.index]
    weights_full_df["pais"] = [universe_tickers[t][1] for t in weights_full_df.index]
    weights_full_df["retorno_esperado_anual"] = mu.reindex(weights_full_df.index)
    weights_full_df.to_csv(os.path.join(run_dir, f"markowitz_universe_weights{out_suffix}.csv"))

    # --- selección de las mejores top_n por peso óptimo ---
    top_tickers = select_top_n(weights_full_series, mu, S, rf, top_n)

    top_df = weights_full_df.loc[weights_full_df.index.intersection(top_tickers)].copy()
    top_df = top_df.reindex(top_tickers)
    top_df.to_csv(os.path.join(run_dir, f"top15_seleccion{out_suffix}.csv"))
    logger.info(f"Top {top_n} seleccionadas ({label}): {top_tickers}")

    # --- portafolio final concentrado en las seleccionadas ---
    weights_final_df, perf_final = optimize_final_portfolio(
        prices_universe[top_tickers], rf, universe_tickers
    )
    weights_final_df.to_csv(os.path.join(run_dir, f"portfolio_final_weights{out_suffix}.csv"))

    performance = {
        "universo_completo": {
            "retorno_esperado_anual": perf_full[0],
            "volatilidad_anual": perf_full[1],
            "sharpe_ratio": perf_full[2],
        },
        "portafolio_15_finales": {
            "retorno_esperado_anual": perf_final[0],
            "volatilidad_anual": perf_final[1],
            "sharpe_ratio": perf_final[2],
        },
        "tasa_libre_riesgo_anual": rf,
        "tickers_seleccionados": top_tickers,
    }
    with open(os.path.join(run_dir, f"portfolio_performance{out_suffix}.json"), "w") as f:
        json.dump(performance, f, indent=2, ensure_ascii=False)

    logger.info(
        f"Performance esperada portafolio final ({label}): retorno={perf_final[0]:.2%} "
        f"vol={perf_final[1]:.2%} sharpe={perf_final[2]:.2f}"
    )

    run_reduced_portfolios(
        prices_universe, rf, run_dir, top_tickers, universe_tickers, out_suffix=out_suffix, label=label
    )

    return top_tickers, weights_final_df, performance


def run_reduced_portfolios(
    prices_universe, rf, run_dir, top_tickers, universe_tickers, out_suffix="", label="universo completo"
):
    """
    Portafolios más chicos para la sección "Portafolio" del dashboard (ver
    config.PORTFOLIO_SIZE_OPTIONS): para cada N, toma las N primeras de
    `top_tickers` (ya ordenadas por peso en la optimización del universo)
    y re-optimiza máx. Sharpe solo sobre ellas, así el 100% se reparte
    entre esas N. No toca los archivos del portafolio de top_n, que siguen
    alimentando el resto de las secciones.

    Escribe por cada N:
        portfolio_final_weights_top<N><key>.csv
        portfolio_performance_top<N><key>.json
    """
    for n in config.PORTFOLIO_SIZE_OPTIONS:
        if n >= len(top_tickers):
            continue
        tickers = top_tickers[:n]
        weights_df, perf = optimize_final_portfolio(prices_universe[tickers], rf, universe_tickers)
        weights_df.to_csv(os.path.join(run_dir, f"portfolio_final_weights_top{n}{out_suffix}.csv"))
        performance = {
            "n_acciones": n,
            "retorno_esperado_anual": perf[0],
            "volatilidad_anual": perf[1],
            "sharpe_ratio": perf[2],
            "tickers_seleccionados": tickers,
        }
        with open(os.path.join(run_dir, f"portfolio_performance_top{n}{out_suffix}.json"), "w") as f:
            json.dump(performance, f, indent=2, ensure_ascii=False)
        logger.info(
            f"Portafolio de {n} ({label}): retorno={perf[0]:.2%} vol={perf[1]:.2%} sharpe={perf[2]:.2f}"
        )


def main():
    prices, rf = load_inputs()
    logger.info(f"Universo de entrada: {prices.shape[1]} tickers, {prices.shape[0]} semanas, rf={rf:.4%}")

    run_dir = config.get_run_dir()
    logger.info(f"Carpeta de esta corrida: {run_dir}")

    # Corre el mismo modelo una vez por cada análisis configurado (ver
    # settings.ANALYSES): el principal (Asia + EEUU) y cualquier análisis
    # complementario (p. ej. "solo EEUU"). Agregar un análisis nuevo es
    # agregar una entrada a esa lista, no tocar este loop.
    primary_result = None
    for spec in config.ANALYSES:
        available = [t for t in prices.columns if t in spec.tickers]
        if len(available) < spec.min_tickers:
            logger.warning(
                f"Solo {len(available)} tickers disponibles para '{spec.label}' (mínimo "
                f"requerido: {spec.min_tickers}); se omite este análisis."
            )
            continue
        result = run_selection(
            prices,
            rf,
            run_dir,
            spec.tickers,
            spec.top_n,
            out_suffix=spec.key,
            label=spec.label,
        )
        if spec.key == "":
            primary_result = result

    return primary_result


if __name__ == "__main__":
    main()
