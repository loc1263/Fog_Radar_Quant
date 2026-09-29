"""Validación de datos de ETFs."""

import time

import yfinance as yf

from fog_radar.core.logging import get_logger

from ..config import (
    HISTORY_PERIODS,
    HTTP_RATE_LIMIT,
    RATE_LIMIT_BASE_WAIT,
    RATE_LIMIT_LONG_WAIT,
    RATE_LIMIT_MAX_RETRIES,
)
from ..models import ETFData
from ..utils import incidencias

log = get_logger("etf.screener")


class ETFValidator:
    """Valida y obtiene datos de ETFs."""

    @staticmethod
    def validate_and_enrich(ticker):
        """Valida y enriquece la información de un ETF."""
        max_retries = RATE_LIMIT_MAX_RETRIES
        rate_limit_count = 0

        for attempt in range(max_retries):
            try:
                return ETFValidator._validate_etf_data(ticker)
            except Exception as e:
                if attempt == max_retries - 1:
                    log.info(f"Error procesando {ticker}: {e}")
                    incidencias.registrar(
                        "TICKER_DESCARTADO",
                        ticker,
                        f"se agotaron los {max_retries} intentos de validacion. "
                        f"Ultimo error: {incidencias.describir_excepcion(e)}",
                    )
                    return None

                if not ETFValidator._is_rate_limit_error(e):
                    rate_limit_count += 1
                    wait_time = RATE_LIMIT_BASE_WAIT * rate_limit_count
                    log.info(f"Rate limit alcanzado para {ticker}. Esperando {wait_time}s...")
                    incidencias.registrar(
                        "RATE_LIMIT",
                        ticker,
                        f"intento {attempt + 1}/{max_retries}, esperando {wait_time}s. "
                        f"{incidencias.describir_excepcion(e)}",
                    )
                    time.sleep(wait_time)

                    if rate_limit_count >= RATE_LIMIT_MAX_RETRIES:
                        log.info("Demasiados bloqueos. Pausando por 15 minutos...")
                        incidencias.registrar(
                            "RATE_LIMIT",
                            ticker,
                            "demasiados bloqueos consecutivos, pausa larga de 15 minutos",
                        )
                        time.sleep(RATE_LIMIT_LONG_WAIT)
                        rate_limit_count = 0

        return None

    @staticmethod
    def _validate_etf_data(ticker):
        """Valida y obtiene datos del ETF."""
        etf_ticker = yf.Ticker(ticker)
        etf_info = etf_ticker.info
        price = ETFValidator._get_price(etf_info)

        if not etf_info or not price:
            return None

        history_data = etf_ticker.history(period=HISTORY_PERIODS["1mo"])
        if history_data.empty:
            return None

        return ETFData(ticker=ticker, info=etf_info, price=price, hist=history_data)

    @staticmethod
    def _get_price(etf_info):
        """Obtiene el precio del ETF desde la información."""
        return (
            etf_info.get("regularMarketPrice")
            or etf_info.get("currentPrice")
            or etf_info.get("previousClose")
        )

    @staticmethod
    def _is_rate_limit_error(exc):
        """Verifica si una excepción es por rate limit (HTTP 429)."""
        response = getattr(exc, "response", None)
        status = getattr(response, "status_code", None)
        return status == HTTP_RATE_LIMIT
