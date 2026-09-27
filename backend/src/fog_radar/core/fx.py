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
