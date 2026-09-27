"""
Construye las secciones fundamentales y técnicas de la evaluación de un
ticker a partir de:

- `info`: el dict crudo de yfinance (`Ticker.get_info()`).
- `closes` / `volumes`: el histórico de precios ya descargado (puede venir
  de una descarga en lote del batch o de una consulta puntual en vivo).

Estas funciones son compartidas por `scanner.py` (evalúa TODOS los
candidatos como parte del batch, reutilizando el histórico que ya bajó en
lote) y por `ticker_eval.py` (evalúa un solo ticker en vivo a pedido del
usuario), para no duplicar la lógica de mapeo de campos.
"""

from __future__ import annotations

from typing import Any

import pandas as pd

from .indicators import macd, rsi, sma


def num(value: Any) -> float | None:
    if value is None:
        return None
    try:
        f = float(value)
    except (TypeError, ValueError):
        return None
    return f if pd.notna(f) else None


def build_valuation(info: dict) -> dict[str, float | None]:
    return {
        "per_trailing": num(info.get("trailingPE")),
        "per_forward": num(info.get("forwardPE")),
        "peg_ratio": num(info.get("trailingPegRatio") or info.get("pegRatio")),
        "ev_to_ebitda": num(info.get("enterpriseToEbitda")),
        "price_to_book": num(info.get("priceToBook")),
        "eps_trailing": num(info.get("trailingEps")),
        "eps_forward": num(info.get("forwardEps")),
    }


def build_profitability(info: dict) -> dict[str, float | None]:
    return {
        "roe": num(info.get("returnOnEquity")),
        "roa": num(info.get("returnOnAssets")),
        "net_margin": num(info.get("profitMargins")),
    }


def build_financial_health(info: dict) -> dict[str, float | None]:
    return {
        "debt_to_equity": num(info.get("debtToEquity")),
        "current_ratio": num(info.get("currentRatio")),
        "free_cash_flow": num(info.get("freeCashflow")),
    }


def build_shareholder_return(info: dict) -> dict[str, float | None]:
    # yfinance devuelve `dividendYield` ya como porcentaje (5.68 = 5.68%),
    # no como fracción. Lo normalizamos a fracción para que sea consistente
    # con el resto de los porcentajes de la app (momentum, retornos, etc.),
    # que el frontend formatea como `valor * 100`.
    raw = num(info.get("dividendYield"))
    return {
        "dividend_yield": raw / 100 if raw is not None else None,
    }


def build_technical(closes: pd.Series, volumes: pd.Series | None = None) -> dict[str, Any]:
    closes = closes.dropna()
    technical: dict[str, Any] = {
        "sma_50": sma(closes, 50),
        "sma_200": sma(closes, 200),
        "rsi_14": rsi(closes, 14),
        "macd": macd(closes),
        "volume": None,
        "average_volume": None,
    }
    if volumes is not None:
        volumes = volumes.dropna()
        if not volumes.empty:
            technical["volume"] = num(volumes.iloc[-1])
            tail = volumes.tail(30)
            technical["average_volume"] = num(tail.mean()) if not tail.empty else None
    return technical
