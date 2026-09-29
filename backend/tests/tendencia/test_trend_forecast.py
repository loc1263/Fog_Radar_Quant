import math

import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.tendencia.modeling.trend_forecast import project_price, trend_signal


def test_project_price_zero_vol_matches_deterministic_drift():
    last_price = 100.0
    mu, sigma, horizon = 0.10, 0.0, 0.5
    central, low, high = project_price(last_price, mu, sigma, horizon)
    expected = last_price * math.exp((mu - 0.5 * sigma**2) * horizon)
    assert central == pytest.approx(expected)
    assert low == pytest.approx(central)
    assert high == pytest.approx(central)


def test_project_price_band_widens_with_horizon():
    last_price = 100.0
    _, low_3m, high_3m = project_price(last_price, 0.08, 0.25, 0.25)
    _, low_1y, high_1y = project_price(last_price, 0.08, 0.25, 1.0)
    assert (high_1y - low_1y) > (high_3m - low_3m)


def test_trend_signal_alcista_when_short_ma_above_long_ma():
    prices = pd.Series(np.linspace(100, 200, 50))
    signal, short_ma, long_ma = trend_signal(prices)
    assert signal == "Alcista"
    assert short_ma > long_ma


def test_trend_signal_bajista_when_short_ma_below_long_ma():
    prices = pd.Series(np.linspace(200, 100, 50))
    signal, short_ma, long_ma = trend_signal(prices)
    assert signal == "Bajista"
    assert short_ma < long_ma


def test_trend_signal_insufficient_data():
    prices = pd.Series(np.linspace(100, 110, 5))
    signal, short_ma, long_ma = trend_signal(prices)
    assert signal == "Datos insuficientes"


def test_trend_signal_3m_windows_react_faster_than_default():
    # 60 semanas subiendo y una caída corta de 5: la señal de 3m (4 vs 13
    # semanas) ya es bajista, la de largo plazo (10 vs 40) todavía no.
    prices = pd.Series(np.concatenate([np.linspace(100, 200, 60), np.linspace(198, 185, 5)]))
    signal_3m, short_ma, long_ma = trend_signal(prices, short_weeks=4, long_weeks=13)
    assert signal_3m == "Bajista"
    assert short_ma < long_ma
    signal_default, _, _ = trend_signal(prices)
    assert signal_default != "Bajista"
