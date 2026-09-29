import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.tendencia.modeling.momentum import (
    compute_momentum_table,
    momentum_score,
    month_return,
    positive_months_ratio,
    resample_monthly,
)


def _monthly_series(values, start="2024-01-31"):
    idx = pd.date_range(start, periods=len(values), freq="ME")
    return pd.Series(values, index=idx)


def test_month_return_basic():
    s = _monthly_series([100, 110, 121])
    assert month_return(s, 1) == pytest.approx(0.10)
    assert month_return(s, 2) == pytest.approx(0.21)


def test_month_return_nan_when_not_enough_history():
    s = _monthly_series([100, 110])
    assert np.isnan(month_return(s, 5))


def test_positive_months_ratio():
    s = _monthly_series([100, 110, 105, 115, 120])  # retornos mes a mes: +,-,+,+
    assert positive_months_ratio(s, 4) == pytest.approx(0.75)


def test_positive_months_ratio_nan_when_not_enough_history():
    s = _monthly_series([100, 110])
    assert np.isnan(positive_months_ratio(s, 5))


def test_momentum_score_rewards_consistency():
    # Mismo retorno promedio en las 4 ventanas, pero distinta proporción de
    # meses positivos -> el más consistente debe rankear más alto.
    score_consistent = momentum_score(0.05, 0.05, 0.05, 0.05, pct_positive_12m=1.0)
    score_inconsistent = momentum_score(0.05, 0.05, 0.05, 0.05, pct_positive_12m=0.3)
    assert score_consistent > score_inconsistent


def test_momentum_score_nan_when_any_component_missing():
    assert np.isnan(momentum_score(0.05, np.nan, 0.05, 0.05, 0.8))


def test_resample_monthly_takes_last_available_price_of_each_month():
    idx = pd.to_datetime(["2024-01-05", "2024-01-12", "2024-01-19", "2024-01-26", "2024-02-02"])
    prices = pd.DataFrame({"AAA": [10, 20, 30, 40, 50]}, index=idx)
    monthly = resample_monthly(prices)
    assert monthly["AAA"].iloc[0] == 40  # último precio disponible en enero
    assert monthly["AAA"].iloc[1] == 50  # único precio de febrero


def test_compute_momentum_table_ranks_best_momentum_first(fresh_settings):
    months = 14
    idx = pd.date_range("2023-01-31", periods=months, freq="ME")

    strong = 100 * (1.03 ** np.arange(months))  # crecimiento sostenido
    weak = 100 * (0.98 ** np.arange(months))  # caída sostenida
    short = pd.Series(np.full(5, 100.0), index=idx[-5:])  # historia insuficiente

    prices = pd.DataFrame({"STRONG": strong, "WEAK": weak}, index=idx)
    prices["SHORT"] = short

    fresh_settings.ALL_TICKERS.update(
        {
            "STRONG": ("Strong Corp", "US", "USD"),
            "WEAK": ("Weak Corp", "US", "USD"),
            "SHORT": ("Short Corp", "US", "USD"),
        }
    )

    out = compute_momentum_table(prices)

    assert list(out["ticker"])[0] == "STRONG"
    assert "SHORT" not in set(out["ticker"])  # historia insuficiente -> excluido
    assert list(out["rank"]) == list(range(1, len(out) + 1))
    strong_row = out[out["ticker"] == "STRONG"].iloc[0]
    weak_row = out[out["ticker"] == "WEAK"].iloc[0]
    assert strong_row["momentum_score_pct"] > weak_row["momentum_score_pct"]


def test_compute_momentum_table_empty_when_no_ticker_has_enough_history(fresh_settings):
    idx = pd.date_range("2024-01-31", periods=3, freq="ME")
    prices = pd.DataFrame({"AAA": [100, 101, 102]}, index=idx)
    out = compute_momentum_table(prices)
    assert out.empty


def test_compute_momentum_table_weekly_uses_week_windows(fresh_settings):
    weeks = 8
    idx = pd.date_range("2024-01-07", periods=weeks, freq="W")
    strong = 100 * (1.01 ** np.arange(weeks))
    weak = 100 * (0.99 ** np.arange(weeks))
    short = pd.Series(np.full(4, 100.0), index=idx[-4:])
    prices = pd.DataFrame({"STRONG": strong, "WEAK": weak}, index=idx)
    prices["SHORT"] = short

    out = compute_momentum_table(prices, frequency="semanal")

    assert list(out["ticker"]) == ["STRONG", "WEAK"]  # SHORT: historia insuficiente
    for col in [
        "momentum_1s_pct",
        "momentum_2s_pct",
        "momentum_3s_pct",
        "momentum_4s_pct",
        "pct_semanas_positivas_4s",
    ]:
        assert col in out.columns
    strong_row = out.iloc[0]
    assert strong_row["momentum_1s_pct"] == pytest.approx(1.0, abs=0.01)
    assert strong_row["pct_semanas_positivas_4s"] == pytest.approx(100.0)
