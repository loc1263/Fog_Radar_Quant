"""Fixtures del módulo EEUU (portadas de Stock_Choice_EEUU/backend/tests/conftest.py)."""

from __future__ import annotations

import pandas as pd
import pytest

from tests.us_stocks.helpers import make_price_series, make_volume_series


@pytest.fixture
def long_closes() -> pd.Series:
    """~400 días de histórico: suficiente para momentum 12-1, SMA 200, etc."""
    return make_price_series(n_days=400)


@pytest.fixture
def long_volumes(long_closes: pd.Series) -> pd.Series:
    return make_volume_series(long_closes)


@pytest.fixture
def short_closes() -> pd.Series:
    """Solo 60 días: insuficiente para momentum 12-1 o SMA 200."""
    return make_price_series(n_days=60, start=50.0, end=55.0)
