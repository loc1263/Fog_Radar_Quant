from __future__ import annotations

import pytest

from fog_radar.core.fx import FxRates


class FakeSource:
    def __init__(self, prices: dict[str, float | None]) -> None:
        self.prices = prices
        self.calls: list[str] = []

    def __call__(self, pair: str) -> float | None:
        self.calls.append(pair)
        value = self.prices.get(pair)
        if isinstance(value, Exception):
            raise value
        return value


def test_misma_moneda_es_1() -> None:
    src = FakeSource({})
    assert FxRates(src).rate("CLP", "CLP") == 1.0
    assert src.calls == []


def test_usd_a_clp_y_vuelta() -> None:
    fx = FxRates(FakeSource({"CLP=X": 950.0}))
    assert fx.rate("USD", "CLP") == 950.0
    assert fx.rate("CLP", "USD") == pytest.approx(1 / 950)


def test_cruce_via_usd() -> None:
    fx = FxRates(FakeSource({"EUR=X": 0.9, "CLP=X": 950.0}))
    assert fx.rate("EUR", "CLP") == pytest.approx(950 / 0.9)


def test_cachea_incluso_fallos() -> None:
    src = FakeSource({"CLP=X": RuntimeError("bloqueo de Yahoo")})  # type: ignore[dict-item]
    fx = FxRates(src)
    assert fx.rate("USD", "CLP") is None
    assert fx.rate("USD", "CLP") is None
    assert src.calls == ["CLP=X"]


def test_precio_cero_o_ausente_da_none() -> None:
    fx = FxRates(FakeSource({"CLP=X": 0.0}))
    assert fx.rate("CLP", "USD") is None
    assert FxRates(FakeSource({})).rate("EUR", "CLP") is None
