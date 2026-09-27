from __future__ import annotations

from unittest import mock

import pandas as pd
import pytest

from fog_radar.modules.us_stocks import ticker_eval as te
from tests.us_stocks.helpers import make_price_series, make_volume_series


def _fake_history(closes: pd.Series, volumes: pd.Series) -> pd.DataFrame:
    return pd.DataFrame({"Close": closes, "Volume": volumes})


def test_evaluate_ticker_empty_string_raises():
    with pytest.raises(te.TickerNotFoundError):
        te.evaluate_ticker("   ")


def test_evaluate_ticker_get_info_failure_raises():
    fake_obj = mock.Mock()
    fake_obj.get_info.side_effect = RuntimeError("HTTP 404")

    with mock.patch.object(te.yf, "Ticker", return_value=fake_obj):
        with pytest.raises(te.TickerNotFoundError):
            te.evaluate_ticker("ZZZZINVALID")


def test_evaluate_ticker_empty_history_raises():
    fake_obj = mock.Mock()
    fake_obj.get_info.return_value = {"longName": "Nada Inc."}
    fake_obj.history.return_value = pd.DataFrame()

    with mock.patch.object(te.yf, "Ticker", return_value=fake_obj):
        with pytest.raises(te.TickerNotFoundError):
            te.evaluate_ticker("NADA")


def test_evaluate_ticker_success_returns_full_evaluation():
    closes = make_price_series(n_days=400, start=100, end=300)
    volumes = make_volume_series(closes)

    fake_obj = mock.Mock()
    fake_obj.get_info.return_value = {
        "longName": "Fake Corp.",
        "sector": "Technology",
        "industry": "Software",
        "currency": "USD",
        "currentPrice": 300.5,
        "marketCap": 5e11,
        "trailingPE": 22.0,
        "returnOnEquity": 0.25,
        "profitMargins": 0.18,
        "debtToEquity": 40.0,
        "currentRatio": 1.8,
        "freeCashflow": 3e9,
        "dividendYield": 1.5,
    }
    fake_obj.history.return_value = _fake_history(closes, volumes)

    with mock.patch.object(te.yf, "Ticker", return_value=fake_obj):
        result = te.evaluate_ticker("  fake  ")

    assert result["ticker"] == "FAKE"  # strip + upper
    assert result["name"] == "Fake Corp."
    assert result["sector"] == "Technology"
    assert result["price"] == 300.5
    assert result["market_cap"] == 5e11
    assert result["valuation"]["per_trailing"] == 22.0
    assert result["profitability"]["roe"] == 0.25
    assert result["shareholder_return"]["dividend_yield"] == pytest.approx(0.015)
    assert result["technical"]["sma_200"] is not None
    assert result["score"]["overall"] is not None
    assert "fetched_at" in result


def test_evaluate_ticker_falls_back_to_last_close_when_price_missing():
    closes = make_price_series(n_days=60, start=10, end=20)
    volumes = make_volume_series(closes)

    fake_obj = mock.Mock()
    fake_obj.get_info.return_value = {}  # sin currentPrice ni regularMarketPrice
    fake_obj.history.return_value = _fake_history(closes, volumes)

    with mock.patch.object(te.yf, "Ticker", return_value=fake_obj):
        result = te.evaluate_ticker("NOPRICE")

    assert result["price"] == float(closes.iloc[-1])
