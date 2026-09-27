"""
Universo de acciones de EEUU a evaluar.

yfinance no ofrece un "screener" nativo por market cap, así que primero
necesitamos una lista de tickers candidatos. Usamos la unión de los
componentes del S&P 500 y el Nasdaq-100 (scrapeados de Wikipedia), que
cubre de sobra cualquier compañía estadounidense con más de 200B de
capitalización. Si Wikipedia no está disponible, caemos a una lista
estática de respaldo con las mega/large caps más conocidas.
"""

from __future__ import annotations

import io
import logging

import pandas as pd
import requests

logger = logging.getLogger("fog_radar.us.universe")

_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/124.0 Safari/537.36"
    )
}

_SP500_URL = "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies"
_NASDAQ100_URL = "https://en.wikipedia.org/wiki/Nasdaq-100"

# Respaldo estático: large/mega caps de EEUU muy conocidas. Se usa solo si
# el scraping de Wikipedia falla (sin conexión, bloqueo, cambio de formato).
FALLBACK_TICKERS: list[str] = sorted(
    set(
        [
            "AAPL",
            "MSFT",
            "GOOGL",
            "GOOG",
            "AMZN",
            "NVDA",
            "META",
            "TSLA",
            "BRK.B",
            "LLY",
            "AVGO",
            "JPM",
            "V",
            "WMT",
            "XOM",
            "UNH",
            "ORCL",
            "MA",
            "HD",
            "PG",
            "COST",
            "JNJ",
            "NFLX",
            "BAC",
            "ABBV",
            "CRM",
            "CVX",
            "KO",
            "AMD",
            "PEP",
            "TMO",
            "MRK",
            "ADBE",
            "LIN",
            "ACN",
            "MCD",
            "ABT",
            "WFC",
            "CSCO",
            "NOW",
            "IBM",
            "GE",
            "PM",
            "TXN",
            "INTU",
            "DIS",
            "VZ",
            "AXP",
            "CAT",
            "AMAT",
            "ISRG",
            "QCOM",
            "PFE",
            "DHR",
            "NEE",
            "CMCSA",
            "UBER",
            "T",
            "AMGN",
            "SPGI",
            "LOW",
            "UNP",
            "PGR",
            "HON",
            "COP",
            "BLK",
            "ETN",
            "SYK",
            "BSX",
            "TJX",
            "VRTX",
            "BX",
            "MDT",
            "PANW",
            "C",
            "MU",
            "LRCX",
            "BA",
            "SBUX",
            "GILD",
            "ADI",
            "PLD",
            "MMC",
            "ANET",
            "SCHW",
            "KLAC",
            "APH",
            "ELV",
            "REGN",
            "SO",
            "CB",
            "DE",
            "MO",
            "ICE",
            "PYPL",
            "SHW",
            "FI",
            "DUK",
            "WM",
            "NKE",
            "ABNB",
            "APO",
            "EQIX",
            "CME",
            "AON",
            "CI",
            "SNPS",
            "MCK",
            "CDNS",
            "ITW",
            "MDLZ",
            "MSI",
            "TT",
            "ZTS",
            "USB",
            "EMR",
            "NOC",
            "FDX",
            "CL",
            "GD",
            "PH",
            "MMM",
            "CTAS",
            "PNC",
            "COF",
            "TDG",
            "ORLY",
            "MAR",
            "APD",
            "ECL",
            "ROP",
            "CSX",
            "WELL",
            "AJG",
            "CARR",
            "AZO",
            "FTNT",
            "CPRT",
            "TFC",
            "SPG",
            "MCO",
            "OKE",
            "AEP",
            "TRV",
            "NSC",
            "PSA",
            "AFL",
            "ALL",
            "NXPI",
            "PCAR",
            "SLB",
            "MET",
            "BDX",
            "URI",
            "GWW",
            "KMB",
            "F",
            "GM",
            "HLT",
            "COIN",
            "PWR",
            "DHI",
            "AIG",
            "MSCI",
            "IQV",
            "STZ",
            "O",
            "AME",
        ]
    )
)


def _clean_ticker(raw: str) -> str:
    """Normaliza tickers al formato que espera Yahoo Finance (BRK.B -> BRK-B)."""
    return raw.strip().upper().replace(".", "-")


def _scrape_table(url: str, ticker_column_candidates: tuple[str, ...]) -> set[str]:
    resp = requests.get(url, headers=_HEADERS, timeout=15)
    resp.raise_for_status()
    tables = pd.read_html(io.StringIO(resp.text))
    for table in tables:
        cols = [str(c) for c in table.columns]
        for candidate in ticker_column_candidates:
            if candidate in cols:
                tickers = table[candidate].dropna().astype(str).map(_clean_ticker)
                return set(t for t in tickers if t and t.isascii())
    return set()


def get_us_large_cap_universe() -> list[str]:
    """Devuelve la lista de tickers candidatos (S&P 500 ∪ Nasdaq-100)."""
    tickers: set[str] = set()

    try:
        tickers |= _scrape_table(_SP500_URL, ("Symbol",))
    except Exception:
        logger.warning("No se pudo scrapear el S&P 500 de Wikipedia", exc_info=True)

    try:
        tickers |= _scrape_table(_NASDAQ100_URL, ("Ticker", "Symbol"))
    except Exception:
        logger.warning("No se pudo scrapear el Nasdaq-100 de Wikipedia", exc_info=True)

    if not tickers:
        logger.warning("Usando lista de respaldo estática de tickers")
        tickers = set(FALLBACK_TICKERS)

    return sorted(tickers)
