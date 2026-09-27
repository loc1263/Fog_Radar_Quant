"""Catálogo de columnas del CSV de indicadores → claves de la API.

El CSV conserva los nombres del original (con espacios y tildes); la API los
expone con claves snake_case estables, una etiqueta en español, un grupo y
una unidad que el frontend usa para formatear.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

Unidad = Literal["texto", "numero", "multiplo", "pct", "pct_pts", "monto", "cantidad", "fecha"]
"""``pct`` = fracción (0.12 → 12 %); ``pct_pts`` = ya en % (5.1 → 5,1 %);
``multiplo`` = veces (x); ``monto`` = moneda de cotización; ``fecha`` = epoch (s)."""


@dataclass(frozen=True)
class Columna:
    csv: str
    key: str
    label: str
    grupo: str
    unidad: Unidad
    descripcion: str | None = None


IDENT = "Identificación"
VAL = "Valoración"
REN = "Rentabilidad"
CRE = "Crecimiento"
EEFF = "Estados financieros"
SAL = "Salud financiera"
PRE = "Precio y momentum"
VOL = "Volumen y propiedad"
DIV = "Dividendos"

GRUPOS: tuple[str, ...] = (IDENT, VAL, REN, CRE, EEFF, SAL, PRE, VOL, DIV)

COLUMNAS: tuple[Columna, ...] = (
    Columna("Moneda Cotización", "moneda_cotizacion", "Moneda cotización", IDENT, "texto"),
    Columna(
        "Moneda Estados Financieros",
        "moneda_eeff",
        "Moneda EEFF",
        IDENT,
        "texto",
        "Si difiere de la de cotización, los montos se convierten con la tasa aplicada.",
    ),
    Columna("Tasa Cambio Aplicada", "tasa_cambio", "Tasa de cambio aplicada", IDENT, "numero"),
    Columna("Market Cap", "market_cap", "Capitalización", VAL, "monto"),
    Columna("Enterprise Value", "enterprise_value", "Enterprise Value", VAL, "monto"),
    Columna("Trailing P/E", "trailing_pe", "P/E (ttm)", VAL, "multiplo"),
    Columna("Forward P/E", "forward_pe", "P/E forward", VAL, "multiplo"),
    Columna("PEG Ratio (5yr expected)", "peg", "PEG (5 años)", VAL, "multiplo"),
    Columna("Price/Sales", "price_sales", "Precio/Ventas", VAL, "multiplo"),
    Columna("Price/Book", "price_book", "Precio/Libro", VAL, "multiplo"),
    Columna("Enterprise Value/Revenue", "ev_revenue", "EV/Ventas", VAL, "multiplo"),
    Columna("Enterprise Value/EBITDA", "ev_ebitda", "EV/EBITDA", VAL, "multiplo"),
    Columna("Profit Margin", "profit_margin", "Margen neto", REN, "pct"),
    Columna("Operating Margin (ttm)", "operating_margin", "Margen operacional", REN, "pct"),
    Columna("Return on Assets (ttm)", "roa", "ROA", REN, "pct"),
    Columna("Return on Equity (ttm)", "roe", "ROE", REN, "pct"),
    Columna("Quarterly Revenue Growth (yoy)", "revenue_growth_q", "Crec. ingresos trim. (a/a)", CRE, "pct"),
    Columna(
        "Quarterly Earnings Growth (yoy)", "earnings_growth_q", "Crec. utilidades trim. (a/a)", CRE, "pct"
    ),
    Columna("Revenue (ttm)", "revenue", "Ingresos (ttm)", EEFF, "monto"),
    Columna("Revenue Per Share (ttm)", "revenue_per_share", "Ingresos por acción", EEFF, "numero"),
    Columna("Gross Profit (ttm)", "gross_profit", "Utilidad bruta (ttm)", EEFF, "monto"),
    Columna("EBITDA", "ebitda", "EBITDA", EEFF, "monto"),
    Columna("Net Income Avi to Common (ttm)", "net_income", "Utilidad neta (ttm)", EEFF, "monto"),
    Columna("Diluted EPS (ttm)", "eps", "UPA diluida (ttm)", EEFF, "numero"),
    Columna("Operating Cash Flow (ttm)", "operating_cash_flow", "Flujo operacional (ttm)", EEFF, "monto"),
    Columna("Levered Free Cash Flow (ttm)", "levered_fcf", "Flujo libre apalancado (ttm)", EEFF, "monto"),
    Columna("Total Cash (mrq)", "total_cash", "Caja", SAL, "monto"),
    Columna("Total Cash Per Share (mrq)", "cash_per_share", "Caja por acción", SAL, "numero"),
    Columna("Total Debt (mrq)", "total_debt", "Deuda total", SAL, "monto"),
    Columna("Total Debt/Equity (mrq)", "debt_equity", "Deuda/Patrimonio (%)", SAL, "pct_pts"),
    Columna("Current Ratio (mrq)", "current_ratio", "Liquidez corriente", SAL, "multiplo"),
    Columna("Book Value Per Share (mrq)", "book_value_per_share", "Valor libro por acción", SAL, "numero"),
    Columna("Beta (5Y Monthly)", "beta", "Beta (5 años)", PRE, "numero"),
    Columna("52 Week Change", "change_52w", "Variación 52 semanas", PRE, "pct"),
    Columna("S&P 500 52-Week Change", "sp500_change_52w", "S&P 500 52 semanas", PRE, "pct"),
    Columna("52 Week High", "high_52w", "Máximo 52 semanas", PRE, "numero"),
    Columna("52 Week Low", "low_52w", "Mínimo 52 semanas", PRE, "numero"),
    Columna("50-Day Moving Average", "sma_50", "Media móvil 50 días", PRE, "numero"),
    Columna("200-Day Moving Average", "sma_200", "Media móvil 200 días", PRE, "numero"),
    Columna("Retorno 5D", "ret_5d", "Retorno 5 días", PRE, "pct"),
    Columna("Retorno 1M", "ret_1m", "Retorno 1 mes", PRE, "pct"),
    Columna("Retorno 3M", "ret_3m", "Retorno 3 meses", PRE, "pct"),
    Columna("Retorno 6M", "ret_6m", "Retorno 6 meses", PRE, "pct"),
    Columna("Retorno YTD", "ret_ytd", "Retorno YTD", PRE, "pct"),
    Columna("Retorno 1A", "ret_1a", "Retorno 1 año", PRE, "pct"),
    Columna("Avg Vol (3 month)", "avg_vol_3m", "Volumen prom. 3 meses", VOL, "cantidad"),
    Columna("Avg Vol (10 day)", "avg_vol_10d", "Volumen prom. 10 días", VOL, "cantidad"),
    Columna("Shares Outstanding", "shares_outstanding", "Acciones en circulación", VOL, "cantidad"),
    Columna("Implied Shares Outstanding", "implied_shares", "Acciones implícitas", VOL, "cantidad"),
    Columna("Float", "float_shares", "Free float", VOL, "cantidad"),
    Columna("% Held by Insiders", "held_insiders", "% en manos de controladores", VOL, "pct"),
    Columna("% Held by Institutions", "held_institutions", "% en manos de instituciones", VOL, "pct"),
    Columna("Forward Annual Dividend Rate", "dividend_rate", "Dividendo anual (forward)", DIV, "numero"),
    Columna(
        "Forward Annual Dividend Yield (%)", "dividend_yield", "Dividend yield (forward)", DIV, "pct_pts"
    ),
    Columna(
        "Trailing Annual Dividend Rate", "trailing_dividend_rate", "Dividendo anual (ttm)", DIV, "numero"
    ),
    Columna(
        "Trailing Annual Dividend Yield (%)",
        "trailing_dividend_yield",
        "Dividend yield (ttm)",
        DIV,
        "pct_pts",
    ),
    Columna(
        "5 Year Average Dividend Yield (%)",
        "dividend_yield_5y",
        "Dividend yield prom. 5 años",
        DIV,
        "pct_pts",
    ),
    Columna(
        "Payout Ratio",
        "payout_ratio",
        "Payout",
        DIV,
        "pct",
        "Sobre 100 % el dividendo no se considera sostenible y no puntúa en el pilar Dividendo.",
    ),
    Columna("Ex-Dividend Date", "ex_dividend_date", "Fecha ex-dividendo", DIV, "fecha"),
    Columna("Last Split Factor", "last_split_factor", "Último split", DIV, "texto"),
    Columna("Last Split Date", "last_split_date", "Fecha último split", DIV, "fecha"),
)

COLUMNAS_POR_CSV: dict[str, Columna] = {c.csv: c for c in COLUMNAS}
