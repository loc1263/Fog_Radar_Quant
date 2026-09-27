from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.us_stocks.indicators import macd, rsi, sma
from tests.us_stocks.helpers import make_price_series


def test_sma_returns_none_with_insufficient_data():
    closes = make_price_series(n_days=10)
    assert sma(closes, window=50) is None


def test_sma_matches_plain_rolling_mean():
    closes = make_price_series(n_days=100, start=10, end=20)
    result = sma(closes, window=10)
    expected = float(closes.rolling(window=10).mean().iloc[-1])
    assert result == expected


def test_sma_exact_window_length_is_valid():
    closes = make_price_series(n_days=50)
    assert sma(closes, window=50) is not None


def test_rsi_returns_none_with_insufficient_data():
    closes = make_price_series(n_days=10)
    assert rsi(closes, window=14) is None


def test_rsi_all_gains_is_100():
    dates = pd.date_range("2025-01-01", periods=30, freq="D")
    closes = pd.Series(np.arange(1, 31, dtype=float), index=dates)  # estrictamente creciente
    assert rsi(closes, window=14) == 100.0


def test_rsi_all_losses_is_0():
    dates = pd.date_range("2025-01-01", periods=30, freq="D")
    closes = pd.Series(np.arange(30, 0, -1, dtype=float), index=dates)  # estrictamente decreciente
    assert rsi(closes, window=14) == 0.0


def test_rsi_flat_series_is_between_0_and_100():
    dates = pd.date_range("2025-01-01", periods=30, freq="D")
    closes = pd.Series([100.0] * 30, index=dates)
    value = rsi(closes, window=14)
    assert value is not None
    assert 0.0 <= value <= 100.0


def test_macd_returns_none_with_insufficient_data():
    closes = make_price_series(n_days=20)
    assert macd(closes) is None


def test_macd_histogram_is_macd_minus_signal():
    closes = make_price_series(n_days=100, start=50, end=150)
    result = macd(closes)
    assert result is not None
    assert set(result.keys()) == {"macd", "signal", "histogram"}
    assert result["histogram"] == pytest.approx(result["macd"] - result["signal"], rel=1e-9)


def test_macd_uptrend_is_positive():
    closes = make_price_series(n_days=100, start=50, end=150)  # tendencia alcista sostenida
    result = macd(closes)
    assert result is not None
    assert result["macd"] > 0
