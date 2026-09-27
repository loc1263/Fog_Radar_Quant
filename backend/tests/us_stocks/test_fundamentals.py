from __future__ import annotations

import math

import pytest

from fog_radar.modules.us_stocks.fundamentals import (
    build_financial_health,
    build_profitability,
    build_shareholder_return,
    build_technical,
    build_valuation,
    num,
)
from tests.us_stocks.helpers import make_price_series, make_volume_series


def test_num_none_stays_none():
    assert num(None) is None


def test_num_parses_numeric_strings():
    assert num("12.5") == 12.5


def test_num_rejects_non_numeric():
    assert num("no-numero") is None


def test_num_rejects_nan():
    assert num(math.nan) is None


def test_num_accepts_int_and_float():
    assert num(10) == 10.0
    assert num(3.14) == 3.14


def test_build_valuation_maps_expected_fields():
    info = {
        "trailingPE": 25.0,
        "forwardPE": 20.0,
        "trailingPegRatio": 1.5,
        "pegRatio": 999.0,  # no debería usarse: trailingPegRatio tiene prioridad
        "enterpriseToEbitda": 12.0,
        "priceToBook": 4.0,
        "trailingEps": 3.2,
        "forwardEps": 3.8,
    }
    result = build_valuation(info)
    assert result == {
        "per_trailing": 25.0,
        "per_forward": 20.0,
        "peg_ratio": 1.5,
        "ev_to_ebitda": 12.0,
        "price_to_book": 4.0,
        "eps_trailing": 3.2,
        "eps_forward": 3.8,
    }


def test_build_valuation_falls_back_to_peg_ratio_when_trailing_missing():
    info = {"pegRatio": 2.0}
    assert build_valuation(info)["peg_ratio"] == 2.0


def test_build_valuation_missing_fields_are_none():
    result = build_valuation({})
    assert all(v is None for v in result.values())


def test_build_profitability_maps_expected_fields():
    info = {"returnOnEquity": 0.2, "returnOnAssets": 0.1, "profitMargins": 0.15}
    assert build_profitability(info) == {"roe": 0.2, "roa": 0.1, "net_margin": 0.15}


def test_build_financial_health_maps_expected_fields():
    info = {"debtToEquity": 80.0, "currentRatio": 1.4, "freeCashflow": 5_000_000.0}
    assert build_financial_health(info) == {
        "debt_to_equity": 80.0,
        "current_ratio": 1.4,
        "free_cash_flow": 5_000_000.0,
    }


def test_build_shareholder_return_normalizes_percent_to_fraction():
    # yfinance devuelve dividendYield ya en porcentaje (5.68 = 5.68%).
    info = {"dividendYield": 5.68}
    result = build_shareholder_return(info)
    assert result["dividend_yield"] == pytest.approx(0.0568, rel=1e-9)


def test_build_shareholder_return_none_when_missing():
    assert build_shareholder_return({})["dividend_yield"] is None


def test_build_technical_all_windows_available():
    closes = make_price_series(n_days=400, start=100, end=300)
    volumes = make_volume_series(closes)
    result = build_technical(closes, volumes)

    assert result["sma_50"] is not None
    assert result["sma_200"] is not None
    assert result["rsi_14"] is not None
    assert result["macd"] is not None
    assert result["volume"] == float(volumes.iloc[-1])
    assert result["average_volume"] == float(volumes.tail(30).mean())


def test_build_technical_short_history_has_partial_nulls():
    closes = make_price_series(n_days=60, start=50, end=55)
    volumes = make_volume_series(closes)
    result = build_technical(closes, volumes)

    assert result["sma_50"] is not None  # justo alcanza
    assert result["sma_200"] is None  # no alcanza
    assert result["rsi_14"] is not None


def test_build_technical_without_volumes():
    closes = make_price_series(n_days=60)
    result = build_technical(closes, volumes=None)
    assert result["volume"] is None
    assert result["average_volume"] is None
