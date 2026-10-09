from __future__ import annotations

import json

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
# portfolio_metrics
# --------------------------------------------------------------------------


def test_portfolio_metrics_shape(returns):
    weights = {t: 1.0 / len(returns.columns) for t in returns.columns}
    metrics = p.portfolio_metrics(weights, returns)
    assert set(metrics.keys()) == {"expected_return", "volatility", "sharpe"}
    assert metrics["volatility"] >= 0


# --------------------------------------------------------------------------
# max_weight (tope por activo)
# --------------------------------------------------------------------------


def test_black_litterman_respeta_el_tope_de_peso(returns):
    tickers = list(returns.columns)
    caps = {t: 1e11 for t in tickers}
    scores = {t: 50.0 + 10 * i for i, t in enumerate(tickers)}
    res = p.black_litterman_weights(returns, caps, scores, max_weight=0.3)
    assert max(res["weights"].values()) <= 0.3 + 1e-6
    _assert_long_only_and_normalized(res["weights"])


# --------------------------------------------------------------------------
# select_by_cluster
# --------------------------------------------------------------------------


def _clustered_returns(seed: int = 5, n_days: int = 250) -> pd.DataFrame:
    """3 grupos de 3 tickers muy correlacionados dentro del grupo, independientes entre grupos."""
    rng = np.random.default_rng(seed)
    dates = pd.bdate_range(end="2025-06-30", periods=n_days)
    data = {}
    for g in "ABC":
        factor = rng.normal(0, 0.01, size=n_days)
        for i in range(3):
            data[f"{g}{i}"] = factor + rng.normal(0, 0.001, size=n_days)
    return pd.DataFrame(data, index=dates)


def test_select_by_cluster_elige_un_representante_por_grupo_con_mayor_score():
    rets = _clustered_returns()
    scores = {
        "A0": 10.0,
        "A1": 90.0,
        "A2": 20.0,
        "B0": 70.0,
        "B1": 30.0,
        "B2": 40.0,
        "C0": 5.0,
        "C1": 6.0,
        "C2": 80.0,
    }
    picks = p.select_by_cluster(rets, scores, n_assets=3)
    assert [x["ticker"] for x in picks] == ["A1", "C2", "B0"]  # orden por score desc
    assert [x["cluster"] for x in picks] == [1, 2, 3]


def test_select_by_cluster_universo_menor_que_n_assets_devuelve_todos():
    rets = _clustered_returns()[["A0", "B0", "C0"]]
    picks = p.select_by_cluster(rets, {}, n_assets=10)
    assert sorted(x["ticker"] for x in picks) == ["A0", "B0", "C0"]


def test_select_by_cluster_completa_si_hay_menos_clusters_que_n_assets():
    rets = _clustered_returns()
    scores = {t: float(i) for i, t in enumerate(rets.columns)}
    picks = p.select_by_cluster(rets, scores, n_assets=5)
    assert len(picks) == 5 and len({x["ticker"] for x in picks}) == 5


def test_select_by_cluster_n_assets_invalido():
    with pytest.raises(p.PortfolioDataError):
        p.select_by_cluster(_clustered_returns(), {}, n_assets=1)


# --------------------------------------------------------------------------
# build_portfolio
# --------------------------------------------------------------------------


def test_build_portfolio_integra_seleccion_y_pesos():
    rets = _clustered_returns()
    caps = {t: 1e11 for t in rets.columns}
    scores = {t: 50.0 + i for i, t in enumerate(rets.columns)}
    res = p.build_portfolio(rets, caps, scores, n_assets=3, max_weight=0.5)
    chosen = [x["ticker"] for x in res["selection"]]
    assert len(chosen) == 3 and set(res["weights"]) == set(chosen)
    _assert_long_only_and_normalized(res["weights"])
    assert max(res["weights"].values()) <= 0.5 + 1e-6
    assert set(res["metrics"]) == {"expected_return", "volatility", "sharpe"}


def test_build_portfolio_tope_invalido():
    rets = _clustered_returns()
    with pytest.raises(p.PortfolioDataError):
        p.build_portfolio(rets, {}, {}, max_weight=0.01)
