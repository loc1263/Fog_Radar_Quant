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


def test_normalizar_moneda_y_par_usd() -> None:
    from fog_radar.core import fx

    assert fx.normalizar_moneda("JPY") == ("JPY", 1)
    assert fx.normalizar_moneda("GBp") == ("GBP", 100)
    assert fx.normalizar_moneda("ZZZ") is None
    assert fx.normalizar_moneda("jpy") is None  # Yahoo siempre usa mayúsculas; minúscula = subunidad
    assert fx.normalizar_moneda(None) is None
    assert fx.par_usd("SGD") == "USDSGD=X"
    assert fx.par_usd("GBp") == "USDGBP=X"


def test_serie_a_usd_forward_fill_y_subunidad() -> None:
    import pandas as pd

    from fog_radar.core import fx

    idx = pd.date_range("2026-01-02", periods=3, freq="W-FRI")
    precios = pd.Series([1000.0, 1100.0, 1200.0], index=idx)
    # Falta el FX de la última semana: se usa el último conocido (sin mirar al futuro).
    tasa = pd.Series([0.8, 0.8], index=idx[:2])
    usd = fx.serie_a_usd(precios, tasa, "GBp")
    assert list(usd.round(4)) == [12.5, 13.75, 15.0]
