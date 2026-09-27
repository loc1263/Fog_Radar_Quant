"""
Indicadores técnicos calculados a mano sobre una serie de precios (no
dependen de ninguna librería de TA externa, solo pandas).
"""

from __future__ import annotations

import pandas as pd


def sma(closes: pd.Series, window: int) -> float | None:
    if len(closes) < window:
        return None
    value = closes.rolling(window=window).mean().iloc[-1]
    return float(value) if pd.notna(value) else None


def rsi(closes: pd.Series, window: int = 14) -> float | None:
    """RSI de Wilder (el estándar, el mismo que usan la mayoría de charts)."""
    if len(closes) < window + 1:
        return None

    delta = closes.diff()
    gain = delta.clip(lower=0)
    loss = -delta.clip(upper=0)

    avg_gain = gain.ewm(alpha=1 / window, min_periods=window, adjust=False).mean()
    avg_loss = loss.ewm(alpha=1 / window, min_periods=window, adjust=False).mean()

    last_gain = avg_gain.iloc[-1]
    last_loss = avg_loss.iloc[-1]
    if pd.isna(last_gain) or pd.isna(last_loss):
        return None
    if last_loss == 0:
        return 100.0

    rs = last_gain / last_loss
    return float(100 - (100 / (1 + rs)))


def macd(
    closes: pd.Series,
    fast: int = 12,
    slow: int = 26,
    signal: int = 9,
) -> dict[str, float] | None:
    if len(closes) < slow + signal:
        return None

    ema_fast = closes.ewm(span=fast, adjust=False).mean()
    ema_slow = closes.ewm(span=slow, adjust=False).mean()
    macd_line = ema_fast - ema_slow
    signal_line = macd_line.ewm(span=signal, adjust=False).mean()
    histogram = macd_line - signal_line

    macd_value = macd_line.iloc[-1]
    signal_value = signal_line.iloc[-1]
    hist_value = histogram.iloc[-1]
    if pd.isna(macd_value) or pd.isna(signal_value) or pd.isna(hist_value):
        return None

    return {
        "macd": float(macd_value),
        "signal": float(signal_value),
        "histogram": float(hist_value),
    }
