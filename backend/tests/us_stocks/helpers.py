"""Series sintéticas compartidas por los tests del módulo EEUU (portadas del original)."""

from __future__ import annotations

import numpy as np
import pandas as pd


def make_price_series(
    n_days: int = 400,
    start: float = 100.0,
    end: float = 200.0,
    tz: str | None = "America/New_York",
) -> pd.Series:
    """Serie de precios sintética con `n_days` días corridos (no solo
    hábiles, para simplificar) creciendo linealmente de `start` a `end`,
    con el mismo tipo de índice (tz-aware) que devuelve yfinance."""
    end_date = pd.Timestamp.now(tz=tz).normalize()
    dates = pd.date_range(end=end_date, periods=n_days, freq="D")
    return pd.Series(np.linspace(start, end, n_days), index=dates)


def make_volume_series(closes: pd.Series, base: int = 1_000_000) -> pd.Series:
    rng = np.random.default_rng(seed=42)
    return pd.Series(rng.integers(base, base * 5, size=len(closes)), index=closes.index)
