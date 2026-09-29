import json
import os

import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.tendencia.modeling.markowitz import run_reduced_portfolios, select_top_n


def test_select_top_n_pads_with_highest_individual_sharpe_when_optimizer_leaves_zeros():
    weights = pd.Series({"A": 0.6, "B": 0.4, "C": 0.0, "D": 0.0})
    mu = pd.Series({"A": 0.10, "B": 0.08, "C": 0.20, "D": 0.05})
    S = pd.DataFrame(
        np.diag([0.04, 0.04, 0.09, 0.16]),
        index=["A", "B", "C", "D"],
        columns=["A", "B", "C", "D"],
    )
    rf = 0.03

    top = select_top_n(weights, mu, S, rf, top_n=4)

    assert set(top) == {"A", "B", "C", "D"}
    assert top[:2] == ["A", "B"]
    assert "C" in top[2:]


def test_select_top_n_no_padding_needed_when_enough_nonzero_weights():
    weights = pd.Series({"A": 0.5, "B": 0.3, "C": 0.2})
    mu = pd.Series({"A": 0.1, "B": 0.1, "C": 0.1})
    S = pd.DataFrame(np.eye(3) * 0.04, index=["A", "B", "C"], columns=["A", "B", "C"])
    top = select_top_n(weights, mu, S, rf=0.02, top_n=2)
    assert top == ["A", "B"]


def test_run_reduced_portfolios_reoptimizes_each_size_to_100_percent(tmp_path):
    rng = np.random.default_rng(0)
    tickers = [f"T{i}" for i in range(12)]
    idx = pd.date_range("2021-01-03", periods=200, freq="W")
    rets = rng.normal(0.003, 0.03, size=(200, len(tickers))) + np.linspace(0.004, 0, len(tickers))
    prices = pd.DataFrame(100 * np.cumprod(1 + rets, axis=0), index=idx, columns=tickers)
    universe = {t: (f"Empresa {t}", "EEUU", "USD") for t in tickers}

    run_reduced_portfolios(prices, 0.03, str(tmp_path), tickers, universe, out_suffix="_us")

    for n in (5, 10):
        weights = pd.read_csv(tmp_path / f"portfolio_final_weights_top{n}_us.csv", index_col=0)
        assert set(weights.index) == set(tickers[:n])  # las N primeras de la selección
        assert weights["peso_final"].sum() == pytest.approx(1.0, abs=1e-3)
        with open(tmp_path / f"portfolio_performance_top{n}_us.json", encoding="utf-8") as f:
            perf = json.load(f)
        assert perf["n_acciones"] == n
        assert perf["tickers_seleccionados"] == tickers[:n]


def test_run_reduced_portfolios_skips_sizes_not_smaller_than_selection(tmp_path):
    idx = pd.date_range("2021-01-03", periods=100, freq="W")
    rng = np.random.default_rng(1)
    tickers = [f"T{i}" for i in range(5)]
    prices = pd.DataFrame(
        100 * np.cumprod(1 + rng.normal(0.002, 0.02, (100, 5)), axis=0), index=idx, columns=tickers
    )
    universe = {t: (t, "EEUU", "USD") for t in tickers}

    run_reduced_portfolios(prices, 0.03, str(tmp_path), tickers, universe)

    assert os.listdir(tmp_path) == []  # 5 y 10 no son más chicos que la selección de 5
