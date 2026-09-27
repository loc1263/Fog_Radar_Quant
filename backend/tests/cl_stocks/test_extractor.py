"""Extractor con dobles de ``yf.Ticker`` (sin red)."""

from __future__ import annotations

from typing import Any

import numpy as np
import pandas as pd
import pytest

from fog_radar.core.fx import FxRates
from fog_radar.modules.cl_stocks import extractor as ex


class FakeTicker:
    def __init__(self, info: dict[str, Any], *, precios: pd.Series | None = None) -> None:
        self.info = info
        self._precios = precios
        self.quarterly_income_stmt = pd.DataFrame(
            [[130.0, 120.0, 110.0, 105.0, 100.0]], index=["Total Revenue"]
        )
        self.quarterly_cashflow = pd.DataFrame(
            [[-5.0, -5.0, -5.0, -5.0, -99.0]], index=["Capital Expenditure"]
        )

    def history(self, **_: Any) -> pd.DataFrame:
        if self._precios is None:
            return pd.DataFrame()
        return pd.DataFrame({"Close": self._precios})


INFO_USD = {
    "symbol": "CAP.SN",
    "shortName": "CAP",
    "sector": "Basic Materials",
    "industry": "Steel",
    "currency": "CLP",
    "financialCurrency": "USD",
    "marketCap": 1_000_000.0,
    "enterpriseValue": 2_000_000.0,
    "currentPrice": 10.0,
    "totalRevenue": 1_000.0,  # USD
    "ebitda": 100.0,  # USD
    "bookValue": 0.01,  # USD por acción
    "operatingCashflow": 50.0,
    "trailingAnnualDividendYield": 0.0513,
    "dividendYield": 5.16,
    "payoutRatio": 0.4,
}


def test_convierte_moneda_y_recalcula_ratios() -> None:
    fx = FxRates(lambda pair: {"CLP=X": 1000.0}[pair])
    fila = ex.indicadores_desde_accion(FakeTicker(INFO_USD), fx)
    assert fila["Tasa Cambio Aplicada"] == 1000.0
    assert fila["Revenue (ttm)"] == 1_000_000.0
    assert fila["Price/Sales"] == pytest.approx(1.0)  # 1.000.000 / (1.000 USD * 1.000)
    assert fila["Enterprise Value/EBITDA"] == pytest.approx(20.0)
    assert fila["Enterprise Value/Revenue"] == pytest.approx(2.0)
    assert fila["Price/Book"] == pytest.approx(1.0)  # 10 / (0.01 * 1000)
    # Levered FCF = (OCF + CapEx últimos 4 trimestres) convertido
    assert fila["Levered Free Cash Flow (ttm)"] == pytest.approx((50 - 20) * 1000)


def test_sin_tasa_los_montos_quedan_vacios() -> None:
    fila = ex.indicadores_desde_accion(FakeTicker(INFO_USD), FxRates(lambda _: None))
    assert fila["Tasa Cambio Aplicada"] is None
    assert fila["Revenue (ttm)"] is None and fila["Price/Sales"] is None
    assert fila["Market Cap"] == 1_000_000.0  # los datos de mercado no se tocan


def test_yields_normalizados_a_porcentaje() -> None:
    fila = ex.indicadores_desde_accion(FakeTicker(INFO_USD), FxRates(lambda _: 1000.0))
    assert fila["Trailing Annual Dividend Yield (%)"] == pytest.approx(5.13)
    assert fila["Forward Annual Dividend Yield (%)"] == 5.16


def test_misma_moneda_no_consulta_fx() -> None:
    def no_llamar(_: str) -> float:
        raise AssertionError("no debería consultar FX")

    info = INFO_USD | {"financialCurrency": "CLP"}
    fila = ex.indicadores_desde_accion(FakeTicker(info), FxRates(no_llamar))
    assert fila["Tasa Cambio Aplicada"] == 1.0 and fila["Revenue (ttm)"] == 1_000.0


def test_quarterly_revenue_growth() -> None:
    assert ex.calcular_quarterly_revenue_growth_yoy(FakeTicker({})) == pytest.approx(0.30)
    corto = FakeTicker({})
    corto.quarterly_income_stmt = pd.DataFrame([[1.0, 2.0, 3.0, 4.0]], index=["Total Revenue"])
    assert ex.calcular_quarterly_revenue_growth_yoy(corto) is None


def test_retornos_por_horizonte() -> None:
    idx = pd.bdate_range("2025-01-01", periods=300)
    precios = pd.Series(np.arange(1, 301, dtype=float), index=idx)
    r = ex.calcular_retornos_desde_precios(precios)
    assert r["Retorno 5D"] == pytest.approx(300 / 295 - 1)
    assert r["Retorno 1A"] == pytest.approx(300 / 48 - 1)
    ultimo_2025 = precios[precios.index.year == 2025].iloc[-1]
    assert r["Retorno YTD"] == pytest.approx(300 / ultimo_2025 - 1)


def test_retornos_con_poco_historial() -> None:
    idx = pd.bdate_range("2026-01-05", periods=10)
    r = ex.calcular_retornos_desde_precios(pd.Series(np.linspace(1, 2, 10), index=idx))
    assert r["Retorno 5D"] is not None
    assert r["Retorno 1M"] is None and r["Retorno YTD"] is None


def test_extraer_omite_errores_y_pausa() -> None:
    pausas: list[float] = []

    def factory(t: str) -> FakeTicker:
        if t == "MALO.SN":
            raise RuntimeError("404")
        return FakeTicker(INFO_USD | {"symbol": t, "financialCurrency": "CLP"})

    tickers = ["A.SN", "MALO.SN", "B.SN", "C.SN"]
    df, errores = ex.extraer_indicadores(
        tickers,
        ticker_factory=factory,
        fx=FxRates(lambda _: 1.0),
        pausa_cada=2,
        pausa_segundos=7,
        sleep=pausas.append,
    )
    assert df["Símbolo"].tolist() == ["A.SN", "B.SN", "C.SN"]
    assert errores == ["MALO.SN"]
    assert pausas == [7]  # tras el 2º ticker; no tras el último


def test_lista_de_tickers_ipsa() -> None:
    tickers = ex.cargar_tickers()
    assert len(tickers) == 30 and all(t.endswith(".SN") for t in tickers)
