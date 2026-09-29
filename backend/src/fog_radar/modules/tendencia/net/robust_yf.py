"""
Descargador robusto de datos de Yahoo Finance vía yfinance.

Objetivo: bajar históricos semanales de un universo de ~40-45 tickers sin
que Yahoo Finance bloquee la IP ni devuelva errores 429 (Too Many
Requests). Estrategia (basada en las prácticas recomendadas por la
comunidad de yfinance a 2026):

  1. Descarga SECUENCIAL (nunca threads=True / paralelo): las descargas en
     paralelo son la causa más común de bloqueo.
  2. Pausa aleatoria (jitter) entre cada request para no generar un patrón
     de tráfico robótico y predecible.
  3. Reintentos con backoff exponencial + jitter ante errores de red,
     429 o respuestas vacías.
  4. Caché en disco por ticker: si ya existe un CSV descargado hoy, no se
     vuelve a pedir (idempotente, permite reanudar tras un corte o bloqueo
     sin perder lo ya avanzado).
  5. Sesión persistente (yfinance ya usa internamente curl_cffi con
     impersonation de navegador desde la v0.2.41+, lo cual evita el 403 de
     Yahoo por "cliente no reconocido").
  6. Si una descarga individual falla tras agotar los reintentos, se
     registra el error y se CONTINÚA con el resto (no se aborta el
     pipeline completo por un ticker problemático).

Uso:
    from fog_radar.modules.tendencia.net.robust_yf import download_history
    df = download_history("AAPL", period="5y", interval="1wk")
"""

import logging
import os
import random
import time
from datetime import UTC, datetime

import pandas as pd

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.logging_utils import get_logger

# Antes este módulo usaba logging.getLogger(...) "a secas", sin handlers
# propios. Eso significaba que sus mensajes (el progreso real de la
# descarga: "[ok] AAPL: 260 filas...", "[RATE-LIMIT] ...") dependían de
# que el proceso hubiera configurado el root logger en algún otro lado
# (lo cual pasaba al correr run_pipeline.py completo, pero NO al correr
# un step*.py suelto) — es decir, se perdían silenciosamente en varios
# escenarios de uso normales. get_logger() le da su propio archivo
# (logs/robust_yf.log), consistente con el resto de los pasos.
logger = get_logger("robust_yf", config.LOGS_DIR, log_filename="robust_yf.log")


def _sleep_jitter(min_s=None, max_s=None):
    min_s = config.MIN_SLEEP_BETWEEN_REQUESTS if min_s is None else min_s
    max_s = config.MAX_SLEEP_BETWEEN_REQUESTS if max_s is None else max_s
    time.sleep(random.uniform(min_s, max_s))


def _backoff_sleep(attempt):
    base = config.BACKOFF_BASE_SECONDS * (2**attempt)
    capped = min(base, config.BACKOFF_MAX_SECONDS)
    time.sleep(capped + random.uniform(0, 2))


def cache_path(cache_dir, ticker):
    """Ruta del CSV en caché de `ticker` (^ y = no son válidos en nombres de archivo)."""
    safe_name = ticker.replace("^", "_").replace("=", "_")
    return os.path.join(cache_dir, f"{safe_name}.csv")


def _is_cache_fresh(path, max_age_hours=20):
    if not os.path.exists(path):
        return False
    mtime = datetime.fromtimestamp(os.path.getmtime(path), tz=UTC)
    age_hours = (datetime.now(UTC) - mtime).total_seconds() / 3600
    return age_hours < max_age_hours


def download_history(
    ticker,
    period=None,
    interval=None,
    cache_dir=None,
    force_refresh=False,
    max_retries=None,
):
    """
    Descarga el histórico de precios de un ticker con reintentos y caché.
    Devuelve un DataFrame (columnas: Open, High, Low, Close, Volume, ...)
    indexado por fecha, o un DataFrame vacío si falla definitivamente.
    """
    period = period or config.HISTORY_PERIOD
    interval = interval or config.HISTORY_INTERVAL
    cache_dir = cache_dir or config.RAW_PRICES_DIR
    max_retries = max_retries if max_retries is not None else config.MAX_RETRIES

    os.makedirs(cache_dir, exist_ok=True)
    cache_file = cache_path(cache_dir, ticker)

    if not force_refresh and _is_cache_fresh(cache_file):
        try:
            df = pd.read_csv(cache_file, index_col=0, parse_dates=True)
            if not df.empty:
                logger.info(f"[cache] {ticker}: {len(df)} filas (sin request nuevo)")
                return df
        except Exception:
            pass  # cache corrupta, se re-descarga

    import yfinance as yf

    last_error = None
    for attempt in range(max_retries):
        try:
            t = yf.Ticker(ticker)
            df = t.history(period=period, interval=interval, timeout=config.REQUEST_TIMEOUT, auto_adjust=True)
            if df is None or df.empty:
                raise ValueError("respuesta vacía de Yahoo Finance")

            df.to_csv(cache_file)
            logger.info(f"[ok] {ticker}: {len(df)} filas descargadas (intento {attempt + 1})")
            _sleep_jitter()
            return df

        except Exception as e:
            last_error = e
            msg = str(e).lower()
            is_rate_limit = "429" in msg or "rate" in msg or "too many requests" in msg
            level = "RATE-LIMIT" if is_rate_limit else "ERROR"
            logger.warning(f"[{level}] {ticker}: intento {attempt + 1}/{max_retries} falló: {e}")
            if attempt < max_retries - 1:
                _backoff_sleep(attempt)

    logger.error(f"[FALLO DEFINITIVO] {ticker}: {last_error}")
    return pd.DataFrame()


def download_universe(tickers, cache_dir=None, force_refresh=False, label=""):
    """
    Descarga secuencialmente un diccionario o lista de tickers.
    Devuelve dict {ticker: DataFrame} solo con los que tuvieron éxito, y
    también reporta la lista de fallidos.
    """
    results = {}
    failed = []
    tickers_list = list(tickers)
    total = len(tickers_list)

    for i, ticker in enumerate(tickers_list, start=1):
        logger.info(f"({i}/{total}) {label} descargando {ticker} ...")
        df = download_history(ticker, cache_dir=cache_dir, force_refresh=force_refresh)
        if df.empty:
            failed.append(ticker)
        else:
            results[ticker] = df

    if failed:
        logger.warning(f"Tickers fallidos ({len(failed)}/{total}): {failed}")

    return results, failed


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    print("Prueba rápida: descargando AAPL...")
    df = download_history("AAPL", period="1mo", interval="1d")
    print(df.tail())
