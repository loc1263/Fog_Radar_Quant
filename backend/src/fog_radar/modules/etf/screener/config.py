"""Configuración global de la aplicación."""

from pathlib import Path

DEFAULT_NUM_ETFS = 30
BATCH_SIZE = 30
RISK_FREE_RATE = 0.02

HISTORY_PERIODS = {
    "1mo": "1mo",
    "1y": "1y",
    "6mo": "6mo",
    "3y": "3y",
    "5y": "5y",
    "ytd": "ytd",
}

TRADING_DAYS_PER_YEAR = 252
PERFORMANCE_COLUMNS = [
    "Performance 5D (%)",
    "Performance 1M (%)",
    "Performance 6M (%)",
    "Performance YTD (%)",
    "Performance 1Y (%)",
    "Performance 5Y (%)",
]

PERFORMANCE_COLUMNS_FOR_SELECTION = [
    "Performance 5D (%)",
    "Performance 1M (%)",
    "Performance 6M (%)",
    "Performance 1Y (%)",
    "Performance 5Y (%)",
]

HTTP_RATE_LIMIT = 429
NA_VALUE = "N/A"
SEPARATOR_LINE = "=" * 70

PROVIDER_KEYWORDS = {
    "ishares": "iShares",
    "blackrock": "iShares",
    "vanguard": "Vanguard",
    "spdr": "SPDR",
    "state street": "SPDR",
    "invesco": "Invesco",
    "global x": "Global X",
    "vaneck": "VanEck",
    "schwab": "Charles Schwab",
    "jpmorgan": "JPMorgan",
    "jp morgan": "JPMorgan",
}
DEFAULT_PROVIDER = "Otro"

RATE_LIMIT_BASE_WAIT = 60
RATE_LIMIT_MAX_RETRIES = 3
RATE_LIMIT_LONG_WAIT = 900
BATCH_PAUSE_TIME = 30

# Rutas: en Fog Radar no son constantes del módulo. La lista curada de tickers
# vive junto al código (List_All_ETF.csv) y la carpeta de salida la decide el
# pipeline (data/etf/screener/, ver modules/etf/storage.py).
INPUT_CSV = Path(__file__).with_name("List_All_ETF.csv")

MARKET_INDEX = "^GSPC"

RESULT_FIELDS = [
    "Ticker",
    "Nombre",
    "Precio",
    "Volumen",
    "AUM",
    "Expense Ratio",
    "Inception Date",
    "Category",
    "Performance 5D (%)",
    "Performance 1M (%)",
    "Performance 6M (%)",
    "Performance YTD (%)",
    "Performance 1Y (%)",
    "Performance 5Y (%)",
    "52W High",
    "52W Low",
    "Sharpe Ratio",
    "Treynor Ratio",
    "Beta",
    "Proveedor",
    "Fecha",
]
