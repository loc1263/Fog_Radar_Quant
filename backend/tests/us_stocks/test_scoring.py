from __future__ import annotations

import pytest

from fog_radar.modules.us_stocks.scoring import (
    _label,
    _piecewise,
    _score_roe,
    evaluate_score,
    score_financial_health,
    score_profitability,
    score_shareholder_return,
    score_technical,
    score_valuation,
)

# --- _piecewise: la curva base que usan todos los scores de métrica -------


def test_piecewise_clamps_below_range():
    points = [(10, 0), (20, 100)]
    assert _piecewise(5, points) == 0


def test_piecewise_clamps_above_range():
    points = [(10, 0), (20, 100)]
    assert _piecewise(25, points) == 100


def test_piecewise_interpolates_midpoint():
    points = [(0, 0), (10, 100)]
    assert _piecewise(5, points) == 50


def test_piecewise_none_is_none():
    assert _piecewise(None, [(0, 0), (10, 100)]) is None


# --- Categorías individuales -----------------------------------------------


def test_score_valuation_lower_per_scores_higher():
    cheap = score_valuation({"per_trailing": 10, "peg_ratio": 1, "ev_to_ebitda": 8, "price_to_book": 1})
    expensive = score_valuation({"per_trailing": 55, "peg_ratio": 4, "ev_to_ebitda": 40, "price_to_book": 15})
    assert cheap > expensive


def test_score_valuation_negative_per_is_penalized():
    result = score_valuation({"per_trailing": -5})
    assert result is not None
    assert result < 30


def test_score_valuation_all_missing_is_none():
    assert score_valuation({}) is None


def test_score_profitability_negative_roe_scores_low():
    result = score_profitability({"roe": -0.5, "roa": None, "net_margin": None})
    assert result is not None
    assert result <= 10


def test_score_profitability_averages_only_available_metrics():
    # Con un único dato disponible (roa y net_margin ausentes), el score de
    # la categoría es exactamente el de ese dato solo, no un promedio con
    # valores faltantes tratados como 0.
    assert score_profitability({"roe": 0.30}) == _score_roe(0.30)


def test_score_financial_health_needs_market_cap_for_fcf():
    without_mc = score_financial_health(
        {"debt_to_equity": 50, "current_ratio": 2, "free_cash_flow": 1e9}, None
    )
    with_mc = score_financial_health({"debt_to_equity": 50, "current_ratio": 2, "free_cash_flow": 1e9}, 1e10)
    assert without_mc is not None
    assert with_mc is not None
    # Sin market cap, el promedio se calcula solo con debt/equity y current ratio.
    assert without_mc != with_mc


def test_score_shareholder_return_peaks_around_5_percent():
    low = score_shareholder_return({"dividend_yield": 0.0})
    healthy = score_shareholder_return({"dividend_yield": 0.05})
    very_high = score_shareholder_return({"dividend_yield": 0.20})
    assert healthy > low
    assert healthy > very_high


def test_score_technical_uptrend_scores_higher_than_downtrend():
    uptrend = score_technical(
        {"rsi_14": 60, "macd": {"histogram": 2.0}, "sma_50": 100, "sma_200": 90},
        price=110,
    )
    downtrend = score_technical(
        {"rsi_14": 25, "macd": {"histogram": -2.0}, "sma_50": 100, "sma_200": 110},
        price=90,
    )
    assert uptrend > downtrend


# --- _label ------------------------------------------------------------


@pytest.mark.parametrize(
    "score,expected",
    [
        (90, "Fuerte"),
        (75, "Fuerte"),
        (74.9, "Sólido"),
        (60, "Sólido"),
        (59.9, "Neutral"),
        (45, "Neutral"),
        (44.9, "Débil"),
        (30, "Débil"),
        (29.9, "Muy débil"),
        (0, "Muy débil"),
    ],
)
def test_label_thresholds(score, expected):
    assert _label(score) == expected


# --- evaluate_score: composición completa -----------------------------


def test_evaluate_score_empty_evaluation_has_no_score():
    result = evaluate_score({})
    assert result["overall"] is None
    assert result["label"] == "Sin datos"
    assert all(v is None for v in result["categories"].values())


def test_evaluate_score_renormalizes_when_categories_missing():
    # Solo hay datos de rentabilidad: el score global debería ser
    # exactamente el score de esa categoría (peso renormalizado a 100%).
    evaluation = {
        "price": None,
        "valuation": {},
        "profitability": {"roe": 0.30, "roa": 0.20, "net_margin": 0.25},
        "financial_health": {},
        "shareholder_return": {},
        "technical": {},
    }
    result = evaluate_score(evaluation)
    assert result["categories"]["profitability"] is not None
    assert result["overall"] == pytest.approx(result["categories"]["profitability"], abs=0.1)


def test_evaluate_score_uses_explicit_market_cap_over_evaluation_value():
    evaluation = {
        "market_cap": 1.0,  # a propósito distinto del que se pasa explícito
        "financial_health": {"free_cash_flow": 5e7},
    }
    result_low_mc = evaluate_score(evaluation, market_cap=1e8)  # yield alto
    result_high_mc = evaluate_score(evaluation, market_cap=1e12)  # yield bajo
    assert result_low_mc["categories"]["financial_health"] != result_high_mc["categories"]["financial_health"]


def test_evaluate_score_full_evaluation_returns_all_categories():
    evaluation = {
        "price": 150.0,
        "market_cap": 5e11,
        "valuation": {"per_trailing": 18, "peg_ratio": 1.2, "ev_to_ebitda": 12, "price_to_book": 4},
        "profitability": {"roe": 0.22, "roa": 0.10, "net_margin": 0.18},
        "financial_health": {"debt_to_equity": 60, "current_ratio": 1.6, "free_cash_flow": 2e10},
        "shareholder_return": {"dividend_yield": 0.015},
        "technical": {"rsi_14": 55, "macd": {"histogram": 1.0}, "sma_50": 140, "sma_200": 130},
    }
    result = evaluate_score(evaluation)
    assert result["overall"] is not None
    assert 0 <= result["overall"] <= 100
    assert result["label"] != "Sin datos"
    assert set(result["categories"].keys()) == {
        "valuation",
        "profitability",
        "financial_health",
        "shareholder_return",
        "technical",
    }
    assert all(v is not None for v in result["categories"].values())
