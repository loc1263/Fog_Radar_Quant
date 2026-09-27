"""Extractor Yahoo Finance → ``indicadores_acciones_<timestamp>.csv``.

Port de ``Stock_Choice_Nacional/WEB/backend/pyYahooStockAnalisis.py``.
Cambios respecto del original (sin alterar los datos producidos):

- La lista de tickers vive junto al módulo (``ipsa_tickers.csv``).
- La conversión de moneda usa ``core.fx.FxRates`` (misma lógica, testeable).
- Las funciones reciben el objeto ``yf.Ticker`` para poder testearlas con
  dobles sin red; ``print`` pasa a ``logging``.

Notas de metodología (del original):

- Si ``currency`` != ``financialCurrency`` (p. ej. cotiza en CLP y reporta en
  USD), los montos de estados financieros se convierten a la moneda de
  cotización y se **recalculan** P/S, P/B, EV/Revenue y EV/EBITDA.
- Yields: ``dividendYield`` y el promedio 5 años vienen en puntos
  porcentuales; ``trailingAnnualDividendYield`` en fracción → se lleva a %.
- Quarterly Revenue Growth y Levered FCF se calculan desde los estados
  trimestrales (Yahoo no los entrega para ``.SN``).
"""

from __future__ import annotations

import time
from collections.abc import Callable
from pathlib import Path
from typing import Any

import pandas as pd

from fog_radar.core.fx import FxRates
from fog_radar.core.logging import get_logger

log = get_logger("cl.extractor")

TICKERS_CSV = Path(__file__).with_name("ipsa_tickers.csv")
COLUMNA_TICKER = "Ticker (Yahoo Finance)"

# Pausa anti-bloqueo: cada N tickers, esperar S segundos.
PAUSA_CADA_N_TICKERS = 15
PAUSA_SEGUNDOS = 30

CLAVES_RETORNO = ("Retorno 5D", "Retorno 1M", "Retorno 3M", "Retorno 6M", "Retorno YTD", "Retorno 1A")


def cargar_tickers(csv_path: Path = TICKERS_CSV) -> list[str]:
    df = pd.read_csv(csv_path)
    if COLUMNA_TICKER not in df.columns:
        raise ValueError(f"No se encontró la columna '{COLUMNA_TICKER}' en {csv_path}")
    return df[COLUMNA_TICKER].dropna().unique().tolist()


def calcular_quarterly_revenue_growth_yoy(accion: Any) -> float | None:
    """Ingresos del último trimestre vs. el mismo trimestre del año anterior (≥ 5 trimestres)."""
    try:
        qis = accion.quarterly_income_stmt
        if qis is None or qis.empty or "Total Revenue" not in qis.index:
            return None
        serie = qis.loc["Total Revenue"].dropna()
        if len(serie) < 5:
            return None
        actual, hace_un_anio = serie.iloc[0], serie.iloc[4]
        if not hace_un_anio:
            return None
        return (actual - hace_un_anio) / abs(hace_un_anio)
    except Exception as exc:
        log.warning("No se pudo calcular Quarterly Revenue Growth: %s", exc)
        return None


def calcular_levered_fcf_ttm(accion: Any, operating_cash_flow_ttm: float | None) -> float | None:
    """Operating Cash Flow (ttm) + CapEx de los últimos 4 trimestres (CapEx viene negativo)."""
    if operating_cash_flow_ttm is None:
        return None
    try:
        qcf = accion.quarterly_cashflow
        if qcf is None or qcf.empty or "Capital Expenditure" not in qcf.index:
            return None
        capex = qcf.loc["Capital Expenditure"].dropna()
        if capex.empty:
            return None
        return operating_cash_flow_ttm + capex.iloc[:4].sum()
    except Exception as exc:
        log.warning("No se pudo calcular Levered Free Cash Flow: %s", exc)
        return None


def calcular_retornos_desde_precios(precios: pd.Series) -> dict[str, float | None]:
    """Retornos 5D/1M/3M/6M/YTD/1A (21/63/126/252 días hábiles) desde una serie de cierres."""
    vacio: dict[str, float | None] = dict.fromkeys(CLAVES_RETORNO)
    precios = precios.dropna()
    if precios.empty:
        return vacio
    actual = precios.iloc[-1]
    anio_actual = precios.index[-1].year

    def n_dias(n: int) -> float | None:
        if len(precios) <= n or not actual:
            return None
        base = precios.iloc[-(n + 1)]
        return None if not base else (actual / base) - 1

    def ytd() -> float | None:
        anterior = precios[precios.index.year < anio_actual]
        if anterior.empty or not actual:
            return None
        base = anterior.iloc[-1]
        return None if not base else (actual / base) - 1

    return {
        "Retorno 5D": n_dias(5),
        "Retorno 1M": n_dias(21),
        "Retorno 3M": n_dias(63),
        "Retorno 6M": n_dias(126),
        "Retorno YTD": ytd(),
        "Retorno 1A": n_dias(252),
    }


def calcular_retornos(accion: Any) -> dict[str, float | None]:
    """Una sola llamada ``history(period='2y')`` para todos los horizontes."""
    try:
        hist = accion.history(period="2y", auto_adjust=True)
        if hist is None or hist.empty or "Close" not in hist.columns:
            return dict.fromkeys(CLAVES_RETORNO)
        return calcular_retornos_desde_precios(hist["Close"])
    except Exception as exc:
        log.warning("No se pudieron calcular los retornos de precio: %s", exc)
        return dict.fromkeys(CLAVES_RETORNO)


def dividir_seguro(numerador: float | None, denominador: float | None) -> float | None:
    if numerador is None or denominador is None or denominador == 0:
        return None
    try:
        return numerador / denominador
    except Exception:
        return None


def indicadores_desde_accion(accion: Any, fx: FxRates) -> dict[str, Any]:
    """Fila de indicadores de un ticker (claves = columnas del CSV original)."""
    info: dict[str, Any] = accion.info

    moneda_cot = info.get("currency")
    moneda_fin = info.get("financialCurrency")
    tasa: float | None = 1.0
    if moneda_cot and moneda_fin and moneda_cot != moneda_fin:
        tasa = fx.rate(moneda_fin, moneda_cot)  # None si falló

    def conv(valor: float | None) -> float | None:
        """Monto de estados financieros → moneda de cotización."""
        if valor is None or tasa is None:
            return None
        return valor * tasa

    ocf_raw = info.get("operatingCashflow")
    levered_fcf_raw = calcular_levered_fcf_ttm(accion, ocf_raw)

    revenue = conv(info.get("totalRevenue"))
    ebitda = conv(info.get("ebitda"))
    book_value_ps = conv(info.get("bookValue"))

    market_cap = info.get("marketCap")
    enterprise_value = info.get("enterpriseValue")
    precio = info.get("currentPrice") or dividir_seguro(market_cap, info.get("sharesOutstanding"))

    trailing_yield = info.get("trailingAnnualDividendYield")
    if trailing_yield is not None:
        trailing_yield *= 100

    retornos = calcular_retornos(accion)

    return {
        "Símbolo": info.get("symbol"),
        "Nombre corto": info.get("shortName"),
        "Sector": info.get("sector"),
        "Industria": info.get("industry"),
        "Moneda Cotización": moneda_cot,
        "Moneda Estados Financieros": moneda_fin,
        "Tasa Cambio Aplicada": tasa,
        "Market Cap": market_cap,
        "Enterprise Value": enterprise_value,
        "Trailing P/E": info.get("trailingPE"),
        "Forward P/E": info.get("forwardPE"),
        "PEG Ratio (5yr expected)": info.get("pegRatio"),
        "Price/Sales": dividir_seguro(market_cap, revenue),
        "Price/Book": dividir_seguro(precio, book_value_ps),
        "Enterprise Value/Revenue": dividir_seguro(enterprise_value, revenue),
        "Enterprise Value/EBITDA": dividir_seguro(enterprise_value, ebitda),
        "Profit Margin": info.get("profitMargins"),
        "Operating Margin (ttm)": info.get("operatingMargins"),
        "Return on Assets (ttm)": info.get("returnOnAssets"),
        "Return on Equity (ttm)": info.get("returnOnEquity"),
        "Revenue (ttm)": revenue,
        "Revenue Per Share (ttm)": conv(info.get("revenuePerShare")),
        "Quarterly Revenue Growth (yoy)": calcular_quarterly_revenue_growth_yoy(accion),
        "Gross Profit (ttm)": conv(info.get("grossProfits")),
        "EBITDA": ebitda,
        "Net Income Avi to Common (ttm)": conv(info.get("netIncomeToCommon")),
        "Diluted EPS (ttm)": info.get("trailingEps"),
        "Quarterly Earnings Growth (yoy)": info.get("earningsQuarterlyGrowth"),
        "Total Cash (mrq)": conv(info.get("totalCash")),
        "Total Cash Per Share (mrq)": conv(info.get("totalCashPerShare")),
        "Total Debt (mrq)": conv(info.get("totalDebt")),
        "Total Debt/Equity (mrq)": info.get("debtToEquity"),
        "Current Ratio (mrq)": info.get("currentRatio"),
        "Book Value Per Share (mrq)": book_value_ps,
        "Operating Cash Flow (ttm)": conv(ocf_raw),
        "Levered Free Cash Flow (ttm)": conv(levered_fcf_raw),
        "Beta (5Y Monthly)": info.get("beta"),
        "52 Week Change": info.get("52WeekChange"),
        "S&P 500 52-Week Change": info.get("SandP52WeekChange"),
        "52 Week High": info.get("fiftyTwoWeekHigh"),
        "52 Week Low": info.get("fiftyTwoWeekLow"),
        "50-Day Moving Average": info.get("fiftyDayAverage"),
        "200-Day Moving Average": info.get("twoHundredDayAverage"),
        **{k: retornos.get(k) for k in CLAVES_RETORNO},
        "Avg Vol (3 month)": info.get("averageVolume"),
        "Avg Vol (10 day)": info.get("averageVolume10days"),
        "Shares Outstanding": info.get("sharesOutstanding"),
        "Implied Shares Outstanding": info.get("impliedSharesOutstanding"),
        "Float": info.get("floatShares"),
        "% Held by Insiders": info.get("heldPercentInsiders"),
        "% Held by Institutions": info.get("heldPercentInstitutions"),
        "Forward Annual Dividend Rate": info.get("dividendRate"),
        "Forward Annual Dividend Yield (%)": info.get("dividendYield"),
        "Trailing Annual Dividend Rate": info.get("trailingAnnualDividendRate"),
        "Trailing Annual Dividend Yield (%)": trailing_yield,
        "5 Year Average Dividend Yield (%)": info.get("fiveYearAvgDividendYield"),
        "Payout Ratio": info.get("payoutRatio"),
        "Ex-Dividend Date": info.get("exDividendDate"),
        "Last Split Factor": info.get("lastSplitFactor"),
        "Last Split Date": info.get("lastSplitDate"),
    }


def _yahoo_ticker(symbol: str) -> Any:
    import yfinance as yf

    return yf.Ticker(symbol)


def extraer_indicadores(
    tickers: list[str],
    *,
    ticker_factory: Callable[[str], Any] = _yahoo_ticker,
    fx: FxRates | None = None,
    pausa_cada: int = PAUSA_CADA_N_TICKERS,
    pausa_segundos: float = PAUSA_SEGUNDOS,
    sleep: Callable[[float], None] = time.sleep,
) -> tuple[pd.DataFrame, list[str]]:
    """Descarga los indicadores de todos los tickers.

    Devuelve (DataFrame, tickers_con_error). Un ticker que falla se omite
    (igual que el original) y queda registrado para el mensaje del job.
    """
    fx = fx or FxRates()
    filas: list[dict[str, Any]] = []
    errores: list[str] = []
    total = len(tickers)
    for i, ticker in enumerate(tickers, start=1):
        log.info("Indicadores de %s (%d/%d)", ticker, i, total)
        try:
            filas.append(indicadores_desde_accion(ticker_factory(ticker), fx))
        except Exception as exc:
            log.error("No se pudo obtener %s, se omite: %s", ticker, exc)
            errores.append(ticker)
        if pausa_cada and i % pausa_cada == 0 and i < total:
            log.info("Pausa de %ss para evitar bloqueo de Yahoo Finance", pausa_segundos)
            sleep(pausa_segundos)
    return pd.DataFrame(filas), errores
