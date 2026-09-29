"""Tasas de cambio con caché por par de monedas.

Portado de ``Stock_Choice_Nacional/pyYahooStockAnalisis.py``
(``obtener_tasa_cambio``). Convención de Yahoo: el ticker ``XXX=X`` vale
"cuántas unidades de XXX compra 1 USD". La fuente de precios es inyectable
para poder testear sin red; por defecto usa yfinance.

Lo usan Chile (Fase 3) y, al portarse, Tendencia (Fase 6, TD-DATA-1).
"""

from __future__ import annotations

from collections.abc import Callable

from fog_radar.core.logging import get_logger

log = get_logger("fx")

# ticker de Yahoo ("CLP=X") -> último cierre, o None si no hay dato.
PriceSource = Callable[[str], float | None]


def yahoo_fx_price(pair: str) -> float | None:
    """Último cierre de un par FX de Yahoo (``CLP=X``) en los últimos 5 días."""
    import yfinance as yf  # import diferido: los tests y la API no lo necesitan

    hist = yf.Ticker(pair).history(period="5d")
    if hist is None or hist.empty:
        return None
    return float(hist["Close"].dropna().iloc[-1])


class FxRates:
    """Convierte montos entre monedas usando USD como pivote."""

    def __init__(self, price_source: PriceSource | None = None) -> None:
        self._price = price_source or yahoo_fx_price
        self._cache: dict[tuple[str, str], float | None] = {}

    def rate(self, origen: str, destino: str) -> float | None:
        """Tasa ``origen -> destino`` (monto_origen * tasa = monto_destino).

        Devuelve ``None`` si no se pudo obtener; el resultado (incluido el
        fallo) se cachea para no repetir llamadas a Yahoo.
        """
        if origen == destino:
            return 1.0
        key = (origen, destino)
        if key in self._cache:
            return self._cache[key]

        tasa: float | None = None
        try:
            if origen == "USD":
                tasa = self._price(f"{destino}=X")
            elif destino == "USD":
                precio = self._price(f"{origen}=X")
                tasa = None if not precio else 1 / precio
            else:
                origen_por_usd = self._price(f"{origen}=X")
                destino_por_usd = self._price(f"{destino}=X")
                if origen_por_usd and destino_por_usd:
                    tasa = (1 / origen_por_usd) * destino_por_usd
        except Exception as exc:  # red, parsing, bloqueo de Yahoo...
            log.warning("No se pudo obtener la tasa %s->%s: %s", origen, destino, exc)
            tasa = None

        self._cache[key] = tasa
        return tasa


# ---------------------------------------------------------------------------
# Series históricas (Tendencia) y monedas de Yahoo
# ---------------------------------------------------------------------------
# Códigos ISO 4217 vigentes (se excluyen metales, fondos y códigos de prueba).
MONEDAS_ISO_4217 = frozenset(
    """AED AFN ALL AMD ANG AOA ARS AUD AWG AZN BAM BBD BDT BGN BHD BIF BMD BND BOB BRL BSD BTN BWP BYN
    BZD CAD CDF CHF CLP CNY COP CRC CUP CVE CZK DJF DKK DOP DZD EGP ERN ETB EUR FJD FKP GBP GEL GHS
    GIP GMD GNF GTQ GYD HKD HNL HTG HUF IDR ILS INR IQD IRR ISK JMD JOD JPY KES KGS KHR KMF KPW KRW
    KWD KYD KZT LAK LBP LKR LRD LSL LYD MAD MDL MGA MKD MMK MNT MOP MRU MUR MVR MWK MXN MYR MZN NAD
    NGN NIO NOK NPR NZD OMR PAB PEN PGK PHP PKR PLN PYG QAR RON RSD RUB RWF SAR SBD SCR SDG SEK SGD
    SHP SLE SOS SRD SSP STN SYP SZL THB TJS TMT TND TOP TRY TTD TWD TZS UAH UGX USD UYU UZS VES VND
    VUV WST XAF XCD XOF XPF YER ZAR ZMW ZWL""".split()
)

# Yahoo cotiza algunas bolsas en la subunidad de la moneda (Londres en peniques,
# Johannesburgo en centavos, Tel Aviv en agorot): precio en moneda = precio / divisor.
SUBUNIDADES: dict[str, tuple[str, int]] = {
    "GBp": ("GBP", 100),
    "GBX": ("GBP", 100),
    "ZAc": ("ZAR", 100),
    "ZAC": ("ZAR", 100),
    "ILA": ("ILS", 100),
}


def normalizar_moneda(codigo: str | None) -> tuple[str, int] | None:
    """``(moneda ISO, divisor)`` de un código de Yahoo, o ``None`` si no es una moneda válida.

    >>> normalizar_moneda("GBp")
    ('GBP', 100)
    """
    if not codigo:
        return None
    if codigo in SUBUNIDADES:
        return SUBUNIDADES[codigo]
    iso = codigo.upper()
    return (iso, 1) if iso in MONEDAS_ISO_4217 and codigo == iso else None


def par_usd(moneda: str) -> str:
    """Ticker de Yahoo con las unidades de ``moneda`` por 1 USD (``USDJPY=X``)."""
    iso, _ = normalizar_moneda(moneda) or (moneda.upper(), 1)
    return f"USD{iso}=X"


def serie_a_usd(precios_locales, fx_local_por_usd, moneda: str = ""):  # noqa: ANN001, ANN201 - pandas
    """Convierte una serie de precios en moneda local a USD.

    ``fx_local_por_usd`` es la serie del par ``USDXXX=X`` (unidades locales por
    1 USD); se alinea al índice de los precios con *forward-fill* (el último
    tipo de cambio conocido, sin mirar al futuro). Si ``moneda`` es una
    subunidad (``GBp``), divide además por su divisor.
    """
    _, divisor = normalizar_moneda(moneda) or ("", 1)
    fx = fx_local_por_usd.reindex(precios_locales.index).ffill()
    return precios_locales / fx / divisor
