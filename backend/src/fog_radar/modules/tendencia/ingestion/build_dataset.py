"""
Construye el dataset consolidado a partir de los CSV crudos.

  - Convierte a USD los tickers asiáticos que cotizan en moneda local,
    usando los pares FX descargados por fetch_fx.
  - Alinea todas las series en un calendario semanal común.
  - Calcula retornos semanales (simples y logarítmicos).
  - Filtra tickers con historia insuficiente (< MIN_WEEKS_REQUIRED).
  - Determina la tasa libre de riesgo: usa macro_data_*.csv (fetch_macro_data.py,
    columna "Treasury 3M (%)") si existe; si no, usa el último valor de ^IRX
    descargado por yfinance como respaldo.

Ejecutar (vía wrapper de compatibilidad en la raíz del proyecto):
    python step03_build_dataset.py

Genera:
    data/processed/prices_usd.csv
    data/processed/weekly_returns.csv
    data/processed/risk_free_rate.txt
    data/processed/excluded_tickers.txt
"""

import os

import pandas as pd

from fog_radar.core.fx import serie_a_usd
from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.logging_utils import get_logger
from fog_radar.modules.tendencia.net.robust_yf import cache_path

logger = get_logger("step03", config.LOGS_DIR, log_filename="step03_build_dataset.log")


def _load_price_csv(path):
    df = pd.read_csv(path, index_col=0, parse_dates=True)
    if "Close" not in df.columns:
        return None
    s = df["Close"].copy()
    s.index = pd.to_datetime(s.index, utc=True).tz_localize(None)
    # Normaliza a fin de semana (viernes) para que todas las series calcen
    s.index = s.index.to_period("W").to_timestamp("W")
    s = s[~s.index.duplicated(keep="last")].sort_index()
    return s


def load_fx_rates():
    """Devuelve dict currency -> Series (unidades locales por 1 USD, semanal)."""
    # Solo se avisa por monedas que el universo ACTUAL realmente necesita (el
    # universo es dinámico: qué monedas hacen falta puede cambiar de una
    # corrida a otra, así que no tiene sentido advertir sobre pares FX que
    # ningún ticker de hoy usa).
    currencies_needed = {currency for _, (_, _, currency) in config.ALL_TICKERS.items() if currency != "USD"}
    fx_rates = {}
    for currency in sorted(currencies_needed):
        fx_ticker = config.fx_ticker_for(currency)
        path = cache_path(config.RAW_FX_DIR, fx_ticker)
        if not os.path.exists(path):
            logger.warning(
                f"Falta el par FX para {currency} ({fx_ticker}); "
                f"los tickers en {currency} se excluirán si no se resuelve."
            )
            continue
        s = _load_price_csv(path)
        if s is not None and not s.empty:
            fx_rates[currency] = s
    return fx_rates


def build_price_matrix():
    fx_rates = load_fx_rates()
    series_usd = {}
    excluded = []

    for ticker, (name, _country, currency) in config.ALL_TICKERS.items():
        path = cache_path(config.RAW_PRICES_DIR, ticker)
        if not os.path.exists(path):
            logger.warning(f"Sin datos descargados para {ticker} ({name}); se excluye.")
            excluded.append((ticker, name, "sin datos descargados"))
            continue

        s = _load_price_csv(path)
        if s is None or s.empty:
            excluded.append((ticker, name, "csv vacío o sin columna Close"))
            continue

        if currency != "USD":
            if currency not in fx_rates:
                excluded.append((ticker, name, f"falta FX para {currency}"))
                continue
            # precio_local / (moneda_local por USD) = precio_USD (core.fx, con subunidades GBp/ZAc/ILA)
            s = serie_a_usd(s, fx_rates[currency], currency)

        if len(s.dropna()) < config.MIN_WEEKS_REQUIRED:
            excluded.append((ticker, name, f"historia insuficiente ({len(s.dropna())} semanas)"))
            continue

        series_usd[ticker] = s

    if not series_usd:
        raise RuntimeError(
            "No se pudo construir ninguna serie de precios en USD. "
            "Verifica que fetch_prices y fetch_fx se hayan ejecutado con éxito "
            "(revisa los .log por bloqueos de Yahoo Finance)."
        )

    prices = pd.DataFrame(series_usd).sort_index()
    # Se usa unión de fechas + forward-fill acotado (máx. 2 semanas) para no
    # introducir sesgos por feriados/desalineación de mercados distintos.
    prices = prices.ffill(limit=2)

    return prices, excluded


def determine_risk_free_rate():
    """
    Devuelve la tasa libre de riesgo anualizada (decimal, ej. 0.045 = 4.5%).
    Prioridad: macro_data_*.csv (Treasury 3M) generado por fetch_macro_data.py
    > ^IRX descargado vía yfinance > valor por defecto conservador.
    """
    latest = config.latest_macro_file()
    if latest:
        try:
            df = pd.read_csv(latest)
            row = df[df["Indicador"].str.contains("Treasury 3M", na=False)]
            if not row.empty:
                rate = float(row.iloc[0]["Último valor"]) / 100.0
                logger.info(f"Tasa libre de riesgo desde {latest}: {rate:.4%}")
                return rate
        except Exception as e:
            logger.warning(f"No se pudo leer tasa libre de riesgo de {latest}: {e}")

    irx_path = cache_path(config.RAW_PRICES_DIR, "^IRX")
    if os.path.exists(irx_path):
        s = _load_price_csv(irx_path)
        if s is not None and not s.dropna().empty:
            rate = float(s.dropna().iloc[-1]) / 100.0
            logger.info(f"Tasa libre de riesgo desde ^IRX (yfinance): {rate:.4%}")
            return rate

    logger.warning("No se encontró tasa libre de riesgo; se usa 4.0% por defecto.")
    return 0.04


def load_prices_usd():
    """Matriz de precios semanales en USD que genera este paso (una
    columna por ticker). La leen todos los pasos de modeling/."""
    if not os.path.exists(config.PRICES_USD_PATH):
        raise RuntimeError(f"Falta {config.PRICES_USD_PATH}. Corre build_dataset (step03) primero.")
    return pd.read_csv(config.PRICES_USD_PATH, index_col=0, parse_dates=True)


def main():
    prices, excluded = build_price_matrix()

    weekly_returns = prices.pct_change().dropna(how="all")

    prices.to_csv(config.PRICES_USD_PATH)
    weekly_returns.to_csv(os.path.join(config.PROCESSED_DIR, "weekly_returns.csv"))

    rf = determine_risk_free_rate()
    with open(os.path.join(config.PROCESSED_DIR, "risk_free_rate.txt"), "w") as f:
        f.write(str(rf))

    with open(os.path.join(config.PROCESSED_DIR, "excluded_tickers.txt"), "w") as f:
        for ticker, name, reason in excluded:
            f.write(f"{ticker}\t{name}\t{reason}\n")

    logger.info(f"Dataset construido: {prices.shape[1]} tickers, {prices.shape[0]} semanas")
    logger.info(f"Excluidos: {len(excluded)} -> ver excluded_tickers.txt")
    logger.info(f"Tasa libre de riesgo usada: {rf:.4%}")

    return prices, weekly_returns, rf


if __name__ == "__main__":
    main()
