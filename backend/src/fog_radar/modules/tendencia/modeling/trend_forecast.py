"""
Estimación de tendencia a 6 meses (26 semanas) para las acciones
seleccionadas por el modelo de Markowitz.

Metodología (transparente y deliberadamente simple para el MVP):
  1. Señal técnica de tendencia: cruce de medias móviles semanales
     (10 semanas vs 40 semanas) + momentum de 26 semanas. Además una
     "Señal 3m" de corto plazo con la misma regla (4 vs 13 semanas).
     Ambas señales son solo informativas: no intervienen en la
     proyección ni en los pesos de Markowitz.
  2. Proyección de precio a 6 meses tipo Movimiento Geométrico Browniano
     (GBM), usando el retorno esperado anualizado (mu) y la volatilidad
     anualizada (sigma) estimados a partir de la matriz de covarianza
     histórica:
         precio_6m = precio_actual * exp((mu - 0.5*sigma^2)*0.5)
     con banda de confianza 90% usando sigma*sqrt(0.5).
     Esto NO es una predicción determinística: es una proyección
     estadística basada en el comportamiento histórico de 5 años. Debe
     leerse como rango de escenarios, no como pronóstico garantizado.
  3. Contexto macro (best-effort): si existe macro_data_*.csv (de
     fetch_macro_data.py) o el ^VIX descargado, se agrega una nota de
     régimen de mercado (volatilidad alta/baja, curva de tasas invertida
     o no) para cualificar la lectura de las proyecciones.

Corre esta proyección una vez por cada análisis que `modeling.markowitz`
haya generado (ver config.ANALYSES): busca `portfolio_performance<key>.json`
para cada uno y, si existe, escribe `trend_forecast_6m<key>.csv` con la
misma metodología. Los análisis complementarios que se hayan omitido (por
falta de tickers) simplemente no tienen archivo y se saltan en silencio.

Ejecutar (vía wrapper de compatibilidad en la raíz del proyecto):
    python step05_trend_forecast.py

Genera, dentro de la carpeta de la corrida actual (output/<YYYYMMDD>_<NN>,
ver tendencia.config.get_run_dir):
    trend_forecast_6m<key>.csv (uno por análisis disponible)
    market_regime_notes.txt (compartido, no depende del análisis)
"""

import json
import math
import os

import numpy as np
import pandas as pd

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.ingestion.build_dataset import load_prices_usd
from fog_radar.modules.tendencia.logging_utils import get_logger
from fog_radar.modules.tendencia.net.robust_yf import cache_path

logger = get_logger("step05", config.LOGS_DIR, log_filename="step05_trend_forecast.log")

Z_90 = 1.645  # z-score para banda de confianza ~90%


def trend_signal(prices_series, short_weeks=None, long_weeks=None):
    """Cruce de medias móviles semanales. Por defecto 10 vs 40 semanas
    ("Señal 6m"); con 4 vs 13 semanas es la "Señal 3m" (ver settings)."""
    short_weeks = short_weeks or config.SHORT_MA_WEEKS
    long_weeks = long_weeks or config.LONG_MA_WEEKS
    short_ma = prices_series.rolling(short_weeks).mean().iloc[-1]
    long_ma = prices_series.rolling(long_weeks).mean().iloc[-1]
    last_price = prices_series.iloc[-1]

    if pd.isna(short_ma) or pd.isna(long_ma):
        return "Datos insuficientes", short_ma, long_ma

    if short_ma > long_ma and last_price >= short_ma:
        return "Alcista", short_ma, long_ma
    elif short_ma < long_ma and last_price <= short_ma:
        return "Bajista", short_ma, long_ma
    else:
        return "Lateral / mixta", short_ma, long_ma


def project_price(last_price, mu_i, sigma_i, horizon_years, z=Z_90):
    """
    Proyección GBM con banda de confianza. Extraída como función pura
    (antes estaba inline en el loop de main()) para poder testearla sin
    depender de archivos en disco.

    ``mu_i`` = retorno anual compuesto (CAGR); ``sigma_i`` = volatilidad anual.
    Devuelve (mediana, percentil 5, percentil 95) del precio a ``horizon_years``.
    """
    # Fog Radar (Fase 7, corrección): ``mu_i`` es el CAGR (retorno geométrico de
    # pypfopt.mean_historical_return), que ya descuenta el "volatility drag". La
    # mediana del GBM es P·(1+CAGR)^T → drift logarítmico = ln(1+CAGR)·T. El
    # original usaba exp((μ − σ²/2)·T): trataba el CAGR como tasa continua
    # (sobrestima con μ alto) y restaba σ²/2 otra vez (subestima con σ alta).
    drift = math.log1p(mu_i) * horizon_years
    sigma_h = sigma_i * math.sqrt(horizon_years)
    central = last_price * math.exp(drift)
    low = last_price * math.exp(drift - z * sigma_h)
    high = last_price * math.exp(drift + z * sigma_h)
    return central, low, high


def get_market_regime_notes():
    notes = []
    vix_path = cache_path(config.RAW_PRICES_DIR, "^VIX")
    if os.path.exists(vix_path):
        try:
            df = pd.read_csv(vix_path, index_col=0, parse_dates=True)
            last_vix = float(df["Close"].dropna().iloc[-1])
            if last_vix >= 25:
                notes.append(f"VIX en {last_vix:.1f}: volatilidad elevada, entorno risk-off.")
            elif last_vix <= 15:
                notes.append(f"VIX en {last_vix:.1f}: volatilidad baja, entorno risk-on / complacencia.")
            else:
                notes.append(f"VIX en {last_vix:.1f}: volatilidad en rango normal.")
        except Exception as e:
            logger.warning(f"No se pudo leer ^VIX: {e}")

    macro_file = config.latest_macro_file()
    if macro_file:
        try:
            df = pd.read_csv(macro_file)
            spread_row = df[df["Indicador"].str.contains("Spread 10Y-2Y", na=False)]
            if not spread_row.empty:
                spread = float(spread_row.iloc[0]["Último valor"])
                if spread < 0:
                    notes.append(
                        f"Curva 10Y-2Y invertida ({spread:.2f} pp): señal histórica de "
                        "precaución macro / riesgo de recesión."
                    )
                else:
                    notes.append(f"Curva 10Y-2Y positiva ({spread:.2f} pp): sin señal de inversión.")
            hy_row = df[df["Indicador"].str.contains("Credit Spread High Yield", na=False)]
            if not hy_row.empty:
                hy = float(hy_row.iloc[0]["Último valor"])
                notes.append(f"Spread de crédito High Yield: {hy:.2f} pp (referencia de apetito por riesgo).")
        except Exception as e:
            logger.warning(f"No se pudo leer macro_data csv: {e}")
    else:
        notes.append(
            "No se encontró macro_data_*.csv (fetch_macro_data.py no se ha corrido "
            "con FRED_API_KEY); contexto macro limitado a VIX/^IRX vía yfinance."
        )

    return notes


def compute_forecast_table(prices, top_tickers):
    """
    Calcula la tabla de proyección a 6 meses (señal técnica + GBM) para
    `top_tickers`. Extraída de main() para poder correrla tanto sobre la
    selección del universo combinado como sobre la del análisis "solo
    EEUU" sin duplicar el loop.
    """
    from pypfopt import expected_returns, risk_models

    prices_top = prices[top_tickers]
    mu = expected_returns.mean_historical_return(prices_top, frequency=config.WEEKS_PER_YEAR)
    S = risk_models.sample_cov(prices_top, frequency=config.WEEKS_PER_YEAR)
    sigma = pd.Series(np.sqrt(np.diag(S)), index=S.index)

    horizon_years = config.FORECAST_HORIZON_WEEKS / config.WEEKS_PER_YEAR
    rows = []
    for ticker in top_tickers:
        s = prices_top[ticker].dropna()
        last_price = s.iloc[-1]
        signal, short_ma, long_ma = trend_signal(s)
        signal_3m, short_ma_3m, long_ma_3m = trend_signal(
            s, config.SHORT_MA_WEEKS_3M, config.LONG_MA_WEEKS_3M
        )

        lookback = config.MOMENTUM_LOOKBACK_WEEKS
        momentum = (s.iloc[-1] / s.iloc[-lookback - 1] - 1) if len(s) > lookback else np.nan

        mu_i = mu[ticker]
        sigma_i = sigma[ticker]
        price_central, price_low, price_high = project_price(last_price, mu_i, sigma_i, horizon_years)

        rows.append(
            {
                "ticker": ticker,
                "nombre": config.ALL_TICKERS[ticker][0],
                "pais": config.ALL_TICKERS[ticker][1],
                "sector": config.SECTOR_MAP.get(ticker, "Otro"),
                "precio_actual_usd": round(last_price, 2),
                "senal_tecnica": signal,
                "ma_10s": round(short_ma, 2) if pd.notna(short_ma) else None,
                "ma_40s": round(long_ma, 2) if pd.notna(long_ma) else None,
                "senal_tecnica_3m": signal_3m,
                "ma_4s": round(short_ma_3m, 2) if pd.notna(short_ma_3m) else None,
                "ma_13s": round(long_ma_3m, 2) if pd.notna(long_ma_3m) else None,
                "momentum_26s_pct": round(momentum * 100, 2) if pd.notna(momentum) else None,
                "retorno_esperado_anual_pct": round(mu_i * 100, 2),
                "volatilidad_anual_pct": round(sigma_i * 100, 2),
                "precio_proyectado_6m_usd": round(price_central, 2),
                "precio_min_90pct_6m_usd": round(price_low, 2),
                "precio_max_90pct_6m_usd": round(price_high, 2),
                "retorno_esperado_6m_pct": round((price_central / last_price - 1) * 100, 2),
            }
        )

    return pd.DataFrame(rows).sort_values("retorno_esperado_6m_pct", ascending=False)


def main():
    run_dir = config.get_run_dir()
    prices = load_prices_usd()

    # Una proyección por cada análisis que modeling.markowitz haya generado
    # (ver config.ANALYSES). El principal siempre debe existir si markowitz
    # corrió bien; si falta, es una falla real (no "universo insuficiente")
    # y se avisa fuerte en vez de seguir en silencio.
    primary_result = None
    for spec in config.ANALYSES:
        perf_path = os.path.join(run_dir, f"portfolio_performance{spec.key}.json")
        if not os.path.exists(perf_path):
            if spec.key == "":
                raise RuntimeError(
                    f"Falta {perf_path}. Corre modeling.markowitz (step04) antes de este paso."
                )
            continue  # análisis complementario que se omitió por falta de tickers, ver markowitz

        with open(perf_path) as f:
            perf = json.load(f)
        out_df = compute_forecast_table(prices, perf["tickers_seleccionados"])
        out_df.to_csv(os.path.join(run_dir, f"trend_forecast_6m{spec.key}.csv"), index=False)
        logger.info(f"Proyección a 6 meses ({spec.label}) generada para {len(out_df)} acciones.")
        if spec.key == "":
            primary_result = out_df

    notes = get_market_regime_notes()
    with open(os.path.join(run_dir, "market_regime_notes.txt"), "w", encoding="utf-8") as f:
        f.write("\n".join(notes))

    for n in notes:
        logger.info(f"[régimen] {n}")

    return primary_result, notes


if __name__ == "__main__":
    main()
