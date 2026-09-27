from __future__ import annotations

import json
import math

import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.us_stocks import portfolio as p


def _synthetic_returns(tickers: list[str], n_days: int = 200, seed: int = 3) -> pd.DataFrame:
    """Retornos diarios sintéticos, cada ticker con distinto drift/volatilidad
    y algo de correlación cruzada (vía un factor común), para que la
    covarianza no sea degenerada."""
    rng = np.random.default_rng(seed)
    dates = pd.bdate_range(end="2025-06-30", periods=n_days)
    common_factor = rng.normal(0, 0.006, size=n_days)
    data = {}
    for i, t in enumerate(tickers):
        idio = rng.normal(0.0003 * (i + 1), 0.008 + 0.002 * i, size=n_days)
        data[t] = 0.4 * common_factor + idio
    return pd.DataFrame(data, index=dates)


@pytest.fixture
def returns() -> pd.DataFrame:
    return _synthetic_returns(["AAA", "BBB", "CCC", "DDD"])


def _assert_long_only_and_normalized(weights: dict[str, float]):
    assert all(w >= -1e-9 for w in weights.values())
    assert sum(weights.values()) == pytest.approx(1.0, abs=1e-6)


# --------------------------------------------------------------------------
# load_price_matrix / daily_returns
# --------------------------------------------------------------------------


def test_load_price_matrix_missing_file_raises(tmp_path):
    with pytest.raises(p.PortfolioDataError, match="prices.json"):
        p.load_price_matrix(tmp_path, ["AAA", "BBB"])


def test_load_price_matrix_requires_at_least_two_tickers(tmp_path):
    with pytest.raises(p.PortfolioDataError, match="al menos 2"):
        p.load_price_matrix(tmp_path, ["AAA"])


def test_load_price_matrix_missing_ticker_raises(tmp_path):
    dates = pd.bdate_range(end="2025-06-30", periods=60)
    (tmp_path / "prices.json").write_text(
        json.dumps({"dates": [d.strftime("%Y-%m-%d") for d in dates], "prices": {"AAA": [100.0] * 60}}),
        encoding="utf-8",
    )
    with pytest.raises(p.PortfolioDataError, match="ZZZ"):
        p.load_price_matrix(tmp_path, ["AAA", "ZZZ"])


def test_load_price_matrix_insufficient_common_history_raises(tmp_path):
    dates = pd.bdate_range(end="2025-06-30", periods=10)
    (tmp_path / "prices.json").write_text(
        json.dumps(
            {
                "dates": [d.strftime("%Y-%m-%d") for d in dates],
                "prices": {"AAA": [100.0] * 10, "BBB": [50.0] * 10},
            }
        ),
        encoding="utf-8",
    )
    with pytest.raises(p.PortfolioDataError, match="insuficiente"):
        p.load_price_matrix(tmp_path, ["AAA", "BBB"])


def test_load_price_matrix_reads_and_aligns(tmp_path):
    dates = pd.bdate_range(end="2025-06-30", periods=60)
    (tmp_path / "prices.json").write_text(
        json.dumps(
            {
                "dates": [d.strftime("%Y-%m-%d") for d in dates],
                "prices": {"AAA": list(np.linspace(100, 150, 60)), "BBB": list(np.linspace(50, 40, 60))},
            }
        ),
        encoding="utf-8",
    )
    df = p.load_price_matrix(tmp_path, ["AAA", "BBB"])
    assert list(df.columns) == ["AAA", "BBB"]
    assert len(df) == 60


def test_daily_returns_empty_raises():
    prices = pd.DataFrame({"AAA": [100.0], "BBB": [50.0]})
    with pytest.raises(p.PortfolioDataError):
        p.daily_returns(prices)


# --------------------------------------------------------------------------
# markowitz_weights / efficient_frontier
# --------------------------------------------------------------------------


def test_markowitz_weights_long_only_and_normalized(returns):
    weights = p.markowitz_weights(returns)
    assert set(weights.keys()) == set(returns.columns)
    _assert_long_only_and_normalized(weights)


def test_markowitz_efficient_frontier_returns_points(returns):
    frontier = p.markowitz_efficient_frontier(returns, n_points=10)
    assert len(frontier) > 0
    for point in frontier:
        assert point["volatility"] >= 0
    # Solo se muestra la rama eficiente: tanto el retorno como la
    # volatilidad deben crecer monótonamente a lo largo de la grilla (si no,
    # el gráfico de frontera del frontend queda zigzagueante).
    rets = [pt["return"] for pt in frontier]
    vols = [pt["volatility"] for pt in frontier]
    assert rets == sorted(rets)
    assert all(vols[i] <= vols[i + 1] + 1e-6 for i in range(len(vols) - 1))


def test_markowitz_efficient_frontier_flat_expected_returns_is_empty():
    dates = pd.bdate_range(end="2025-06-30", periods=50)
    returns = pd.DataFrame({"AAA": [0.001] * 50, "BBB": [0.001] * 50}, index=dates)
    assert p.markowitz_efficient_frontier(returns) == []


# --------------------------------------------------------------------------
# black_litterman_weights
# --------------------------------------------------------------------------


def test_black_litterman_weights_long_only_and_normalized(returns):
    tickers = list(returns.columns)
    market_caps = {t: 1e11 * (i + 1) for i, t in enumerate(tickers)}
    scores = {t: 50.0 + i * 10 for i, t in enumerate(tickers)}

    result = p.black_litterman_weights(returns, market_caps, scores)
    _assert_long_only_and_normalized(result["weights"])
    assert set(result["views"].keys()) == set(tickers)
    assert set(result["prior_returns"].keys()) == set(tickers)


def test_black_litterman_falls_back_to_equal_weight_market_when_no_caps(returns):
    tickers = list(returns.columns)
    market_caps = {t: None for t in tickers}
    scores = {t: 50.0 for t in tickers}
    # No debe explotar por división por cero al no haber market caps.
    result = p.black_litterman_weights(returns, market_caps, scores)
    _assert_long_only_and_normalized(result["weights"])


def test_black_litterman_view_override_shifts_that_tickers_view(returns):
    tickers = list(returns.columns)
    market_caps = {t: 1e11 for t in tickers}
    scores = {t: 50.0 for t in tickers}

    baseline = p.black_litterman_weights(returns, market_caps, scores)
    overridden = p.black_litterman_weights(
        returns,
        market_caps,
        scores,
        views={tickers[0]: baseline["views"][tickers[0]] + 0.5},
    )
    assert overridden["views"][tickers[0]] == pytest.approx(baseline["views"][tickers[0]] + 0.5)
    # Con una view mucho más optimista sobre ese ticker, su peso debería subir.
    assert overridden["weights"][tickers[0]] >= baseline["weights"][tickers[0]]


# --------------------------------------------------------------------------
# hrp_weights
# --------------------------------------------------------------------------


def test_hrp_weights_long_only_and_normalized(returns):
    weights = p.hrp_weights(returns)
    assert set(weights.keys()) == set(returns.columns)
    _assert_long_only_and_normalized(weights)


def test_hrp_does_not_require_expected_returns(returns):
    # HRP solo usa covarianza/correlación: acciones con retorno esperado
    # idéntico pero distinta volatilidad igual reciben pesos distintos.
    dates = returns.index
    rng = np.random.default_rng(1)
    low_vol = rng.normal(0.0005, 0.004, size=len(dates))
    high_vol = rng.normal(0.0005, 0.02, size=len(dates))
    df = pd.DataFrame({"LOW": low_vol, "HIGH": high_vol}, index=dates)
    weights = p.hrp_weights(df)
    # HRP tiende a asignar más peso al activo de menor varianza.
    assert weights["LOW"] > weights["HIGH"]


# --------------------------------------------------------------------------
# portfolio_metrics
# --------------------------------------------------------------------------


def test_portfolio_metrics_shape(returns):
    weights = {t: 1.0 / len(returns.columns) for t in returns.columns}
    metrics = p.portfolio_metrics(weights, returns)
    assert set(metrics.keys()) == {"expected_return", "volatility", "sharpe"}
    assert metrics["volatility"] >= 0


# --------------------------------------------------------------------------
# best_combinations
# --------------------------------------------------------------------------


@pytest.fixture
def wide_returns() -> pd.DataFrame:
    return _synthetic_returns([f"T{i}" for i in range(8)], seed=11)


@pytest.mark.parametrize("method", ["markowitz", "black_litterman", "hrp"])
def test_best_combinations_evaluates_all_and_sorts_by_rank(wide_returns, method):
    tickers = list(wide_returns.columns)
    market_caps = {t: 1e11 * (i + 1) for i, t in enumerate(tickers)}
    scores = {t: 40.0 + i * 5 for i, t in enumerate(tickers)}

    result = p.best_combinations(
        wide_returns,
        method,
        k=3,
        market_caps=market_caps,
        scores=scores,
        top_n=3,
    )

    assert result["combinations_evaluated"] == math.comb(len(tickers), 3)
    assert len(result["results"]) == 3
    for combo in result["results"]:
        assert len(combo["tickers"]) == 3
        assert set(combo["tickers"]).issubset(set(tickers))
        _assert_long_only_and_normalized(combo["weights"])

    # Ordenado descendente por el criterio de ranking por defecto.
    rets = [c["metrics"]["expected_return"] for c in result["results"]]
    assert rets == sorted(rets, reverse=True)


def test_best_combinations_rank_by_sharpe_picks_highest_sharpe(wide_returns):
    result = p.best_combinations(wide_returns, "markowitz", k=3, rank_by="sharpe", top_n=3)
    sharpes = [c["metrics"]["sharpe"] for c in result["results"]]
    assert sharpes == sorted(sharpes, reverse=True)


def test_best_combinations_invalid_rank_by(wide_returns):
    with pytest.raises(p.PortfolioDataError, match="rank_by"):
        p.best_combinations(wide_returns, "hrp", k=3, rank_by="volatility")


def test_best_combinations_k_too_small(wide_returns):
    with pytest.raises(p.PortfolioDataError, match="al menos"):
        p.best_combinations(wide_returns, "hrp", k=1)


def test_best_combinations_k_larger_than_universe(wide_returns):
    with pytest.raises(p.PortfolioDataError, match="universo"):
        p.best_combinations(wide_returns, "hrp", k=len(wide_returns.columns) + 1)


def test_best_combinations_k_larger_than_max_combination_size(monkeypatch, wide_returns):
    monkeypatch.setattr(p, "MAX_COMBINATION_SIZE", 3)
    with pytest.raises(p.PortfolioDataError, match="más de 3 tickers"):
        p.best_combinations(wide_returns, "hrp", k=4)


def test_best_combinations_respects_max_combinations_cap(monkeypatch, wide_returns):
    monkeypatch.setattr(p, "MAX_COMBINATIONS", 10)
    with pytest.raises(p.PortfolioDataError, match="10"):
        p.best_combinations(wide_returns, "hrp", k=4)
