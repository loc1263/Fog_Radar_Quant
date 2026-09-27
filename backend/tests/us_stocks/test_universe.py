from __future__ import annotations

from unittest import mock

import pandas as pd
import pytest

from fog_radar.modules.us_stocks import universe as u

# --------------------------------------------------------------------------
# _clean_ticker
# --------------------------------------------------------------------------


@pytest.mark.parametrize(
    "raw,expected",
    [
        ("aapl", "AAPL"),
        ("  msft  ", "MSFT"),
        ("brk.b", "BRK-B"),
        ("BF.B", "BF-B"),
    ],
)
def test_clean_ticker(raw, expected):
    assert u._clean_ticker(raw) == expected


# --------------------------------------------------------------------------
# get_us_large_cap_universe
# --------------------------------------------------------------------------


class _FakeResponse:
    def __init__(self, html: str):
        self.text = html

    def raise_for_status(self):
        pass


def test_get_universe_falls_back_when_scraping_fails():
    with mock.patch.object(u.requests, "get", side_effect=RuntimeError("sin conexión")):
        result = u.get_us_large_cap_universe()
    assert result == sorted(set(u.FALLBACK_TICKERS))
    assert len(result) > 0


def test_get_universe_merges_sp500_and_nasdaq100():
    sp500_html = pd.DataFrame({"Symbol": ["aapl", "msft"]}).to_html()
    nasdaq_html = pd.DataFrame({"Ticker": ["googl", "amzn"]}).to_html()

    def fake_get(url, headers=None, timeout=None):
        if url == u._SP500_URL:
            return _FakeResponse(sp500_html)
        if url == u._NASDAQ100_URL:
            return _FakeResponse(nasdaq_html)
        raise AssertionError(f"URL inesperada: {url}")

    with mock.patch.object(u.requests, "get", side_effect=fake_get):
        result = u.get_us_large_cap_universe()

    assert result == ["AAPL", "AMZN", "GOOGL", "MSFT"]


def test_get_universe_uses_nasdaq_symbol_column_as_fallback_name():
    # El Nasdaq-100 a veces usa "Symbol" en lugar de "Ticker" como
    # encabezado de columna; _scrape_table debe aceptar ambos.
    sp500_html = pd.DataFrame({"Symbol": ["aapl"]}).to_html()
    nasdaq_html = pd.DataFrame({"Symbol": ["tsla"]}).to_html()

    def fake_get(url, headers=None, timeout=None):
        html = sp500_html if url == u._SP500_URL else nasdaq_html
        return _FakeResponse(html)

    with mock.patch.object(u.requests, "get", side_effect=fake_get):
        result = u.get_us_large_cap_universe()

    assert result == ["AAPL", "TSLA"]


def test_get_universe_partial_failure_still_returns_the_other_index():
    nasdaq_html = pd.DataFrame({"Ticker": ["nvda"]}).to_html()

    def fake_get(url, headers=None, timeout=None):
        if url == u._SP500_URL:
            raise RuntimeError("Wikipedia caído")
        return _FakeResponse(nasdaq_html)

    with mock.patch.object(u.requests, "get", side_effect=fake_get):
        result = u.get_us_large_cap_universe()

    assert result == ["NVDA"]
