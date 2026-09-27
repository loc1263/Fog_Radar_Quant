"""
Evaluación puntual de un ticker: a diferencia del scanner (`app/scanner.py`,
que corre como batch sobre cientos de tickers), esto consulta yfinance
en vivo para UN solo ticker a pedido del usuario, así que sí puede
dispararse desde un request HTTP sin riesgo de bloqueo por parte de
Yahoo Finance.

Junta los datos pedidos para evaluar una acción (valoración, rentabilidad,
salud financiera, retorno al accionista y técnico — ver `fundamentals.py`)
y les asigna un score de salud 0-100 (ver `scoring.py`).
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

import yfinance as yf

from .fundamentals import (
    build_financial_health,
    build_profitability,
    build_shareholder_return,
    build_technical,
    build_valuation,
    num,
)
from .scoring import evaluate_score

HISTORY_PERIOD = "1y"


class TickerNotFoundError(Exception):
    """El ticker no existe o Yahoo Finance no devolvió datos utilizables."""


def evaluate_ticker(ticker: str) -> dict[str, Any]:
    ticker = ticker.strip().upper()
    if not ticker:
        raise TickerNotFoundError("Ticker vacío")

    ticker_obj = yf.Ticker(ticker)

    try:
        info = ticker_obj.get_info()
    except Exception as exc:
        raise TickerNotFoundError(f"No se pudo obtener información de {ticker}: {exc}") from exc

    history = ticker_obj.history(period=HISTORY_PERIOD, interval="1d", auto_adjust=True)
    if history.empty:
        raise TickerNotFoundError(
            f"'{ticker}' no parece ser un ticker válido o no tiene histórico disponible"
        )

    closes = history["Close"].dropna()
    volumes = history["Volume"]

    price = num(info.get("currentPrice") or info.get("regularMarketPrice"))
    if price is None and not closes.empty:
        price = float(closes.iloc[-1])

    market_cap = num(info.get("marketCap"))

    result: dict[str, Any] = {
        "ticker": ticker,
        "name": info.get("longName") or info.get("shortName") or ticker,
        "sector": info.get("sector") or "",
        "industry": info.get("industry") or "",
        "currency": info.get("currency") or "",
        "price": price,
        "market_cap": market_cap,
        "fetched_at": datetime.now(UTC).isoformat(),
        "valuation": build_valuation(info),
        "profitability": build_profitability(info),
        "financial_health": build_financial_health(info),
        "shareholder_return": build_shareholder_return(info),
        "technical": build_technical(closes, volumes),
    }
    result["score"] = evaluate_score(result)
    return result
