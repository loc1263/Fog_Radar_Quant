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

from fog_radar.core import yahoo
from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.logging_utils import get_logger

# Fog Radar (Fase 7): la lógica (caché en disco, reintentos con backoff,
# pausa con jitter) vive ahora en ``fog_radar.core.yahoo`` y la comparten
# todos los módulos; este archivo solo le pasa los parámetros de
# ``tendencia.config`` y conserva los nombres que usa el resto del paquete.
# Todas las llamadas pasan además por la sesión común de Yahoo (espaciado
# global y reintento ante 429) cuando el proceso la instaló.
logger = get_logger("robust_yf", config.LOGS_DIR, log_filename="robust_yf.log")

cache_path = yahoo.cache_path


def _is_cache_fresh(path, max_age_hours=20):
    return yahoo.cache_fresca(path, max_age_hours)


def _opciones(period=None, interval=None, max_retries=None):
    return yahoo.OpcionesHistorial(
        period=period or config.HISTORY_PERIOD,
        interval=interval or config.HISTORY_INTERVAL,
        max_retries=max_retries if max_retries is not None else config.MAX_RETRIES,
        pausa_min_s=config.MIN_SLEEP_BETWEEN_REQUESTS,
        pausa_max_s=config.MAX_SLEEP_BETWEEN_REQUESTS,
        backoff_base_s=config.BACKOFF_BASE_SECONDS,
        backoff_max_s=config.BACKOFF_MAX_SECONDS,
        timeout_s=config.REQUEST_TIMEOUT,
    )


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
    return yahoo.descargar_historial(
        ticker,
        cache_dir or config.RAW_PRICES_DIR,
        _opciones(period, interval, max_retries),
        force_refresh=force_refresh,
        logger=logger,
    )


def download_universe(tickers, cache_dir=None, force_refresh=False, label=""):
    """
    Descarga secuencialmente un diccionario o lista de tickers.
    Devuelve dict {ticker: DataFrame} solo con los que tuvieron éxito, y
    también reporta la lista de fallidos.
    """
    return yahoo.descargar_varios(
        tickers,
        cache_dir or config.RAW_PRICES_DIR,
        _opciones(),
        force_refresh=force_refresh,
        label=label,
        logger=logger,
    )


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    print("Prueba rápida: descargando AAPL...")
    df = download_history("AAPL", period="1mo", interval="1d")
    print(df.tail())
