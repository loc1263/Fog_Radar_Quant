from __future__ import annotations

import json
from unittest import mock

import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.us_stocks import scanner as s
from tests.us_stocks.helpers import make_price_series, make_volume_series

# --------------------------------------------------------------------------
# _price_at
# --------------------------------------------------------------------------


def test_price_at_before_history_start_is_none():
    closes = make_price_series(n_days=100)
    first_date = closes.index[0]
    target = first_date - pd.Timedelta(5, unit="D")
    assert s._price_at(closes, first_date, target) is None


def test_price_at_returns_last_close_on_or_before_target():
    closes = make_price_series(n_days=100, start=10, end=20)
    first_date = closes.index[0]
    target = closes.index[50]
    assert s._price_at(closes, first_date, target) == float(closes.asof(target))


# --------------------------------------------------------------------------
# _compute_momentum
# --------------------------------------------------------------------------


def test_compute_momentum_empty_series_is_none():
    assert s._compute_momentum(pd.Series(dtype=float)) is None


def test_compute_momentum_short_history_is_none(short_closes):
    assert s._compute_momentum(short_closes) is None


def test_compute_momentum_uptrend_has_positive_returns(long_closes):
    result = s._compute_momentum(long_closes)
    assert result is not None
    expected_keys = {
        "price",
        "momentum_12_1",
        "return_1d",
        "return_5d",
        "return_3m",
        "return_6m",
        "return_1y",
        "return_ytd",
    }
    assert set(result.keys()) == expected_keys
    # Serie sintética estrictamente creciente: todos los retornos con
    # suficiente historia detrás deben ser positivos.
    assert result["return_1d"] > 0
    assert result["return_5d"] > 0
    assert result["return_3m"] > 0
    assert result["return_6m"] > 0
    assert result["return_1y"] > 0


def test_compute_momentum_downtrend_has_negative_momentum():
    closes = make_price_series(n_days=400, start=200, end=100)
    result = s._compute_momentum(closes)
    assert result is not None
    assert result["momentum_12_1"] < 0
    assert result["return_1y"] < 0


def test_compute_momentum_invalid_price_at_one_target_does_not_break_the_rest():
    # Histórico fijo (no depende de la fecha de hoy) con un dato corrupto
    # justo en la fecha que usaría el cálculo de YTD (1 de enero de 2025):
    # un precio inválido en un plazo puntual debe quedar en None sin
    # invalidar los demás plazos.
    dates = pd.date_range(start="2024-01-01", end="2025-06-15", freq="D")
    closes = pd.Series(np.linspace(100, 300, len(dates)), index=dates)
    closes.loc[pd.Timestamp("2025-01-01")] = -1.0

    result = s._compute_momentum(closes)
    assert result is not None
    assert result["return_ytd"] is None
    assert result["return_1y"] is not None
    assert result["return_6m"] is not None


# --------------------------------------------------------------------------
# _extract_closes_volumes
# --------------------------------------------------------------------------


def _make_multi_ticker_history(tickers: list[str]) -> pd.DataFrame:
    frames = {}
    for i, t in enumerate(tickers):
        closes = make_price_series(n_days=400, start=100 + i, end=200 + i)
        frames[(t, "Close")] = closes
        frames[(t, "Volume")] = make_volume_series(closes)
    df = pd.DataFrame(frames)
    df.columns = pd.MultiIndex.from_tuples(df.columns)
    return df


def test_extract_closes_volumes_multi_ticker():
    history = _make_multi_ticker_history(["AAA", "BBB"])
    result = s._extract_closes_volumes(history, "AAA", single_ticker=False)
    assert result is not None
    closes, volumes = result
    assert len(closes) == 400
    assert len(volumes) == 400


def test_extract_closes_volumes_missing_ticker_is_none():
    history = _make_multi_ticker_history(["AAA"])
    assert s._extract_closes_volumes(history, "ZZZ", single_ticker=False) is None


def test_extract_closes_volumes_single_ticker():
    closes = make_price_series(n_days=50)
    volumes = make_volume_series(closes)
    history = pd.DataFrame({"Close": closes, "Volume": volumes})
    result = s._extract_closes_volumes(history, "AAA", single_ticker=True)
    assert result is not None
    got_closes, got_volumes = result
    assert list(got_closes) == list(closes)
    assert list(got_volumes) == list(volumes)


# --------------------------------------------------------------------------
# _build_price_matrix
# --------------------------------------------------------------------------


def test_build_price_matrix_multi_ticker():
    tickers = ["AAA", "BBB"]
    history = _make_multi_ticker_history(tickers)
    result = s._build_price_matrix(tickers, history, single_ticker=False)

    assert len(result["dates"]) == 400
    assert set(result["prices"].keys()) == set(tickers)
    assert len(result["prices"]["AAA"]) == 400
    assert result["prices"]["AAA"][0] == pytest.approx(100.0)


def test_build_price_matrix_skips_tickers_missing_from_history():
    history = _make_multi_ticker_history(["AAA"])
    result = s._build_price_matrix(["AAA", "ZZZ"], history, single_ticker=False)
    assert set(result["prices"].keys()) == {"AAA"}


# --------------------------------------------------------------------------
# _filter_by_market_cap
# --------------------------------------------------------------------------


def test_filter_by_market_cap_excludes_none_and_below_threshold():
    market_caps = {"AAA": 300e9, "BBB": 50e9, "CCC": None, "DDD": 200e9}
    result = s._filter_by_market_cap(market_caps, min_market_cap=200e9)
    assert result == ["AAA", "DDD"]  # preserva el orden de inserción


def test_filter_by_market_cap_empty_input():
    assert s._filter_by_market_cap({}, min_market_cap=200e9) == []


# --------------------------------------------------------------------------
# _fetch_one_market_cap / _fetch_market_caps (red mockeada)
# --------------------------------------------------------------------------


class _FakeFastInfo(dict):
    """Simula `yf.Ticker(...).fast_info`: accesible tanto por atributo
    (`.market_cap`) como por clave de diccionario (`["marketCap"]`)."""

    def __init__(self, market_cap):
        super().__init__({"marketCap": market_cap})
        self.market_cap = market_cap


def test_fetch_one_market_cap_success():
    with mock.patch.object(s.yf, "Ticker", return_value=mock.Mock(fast_info=_FakeFastInfo(123e9))):
        assert s._fetch_one_market_cap("AAA") == 123e9


def test_fetch_one_market_cap_retries_then_gives_up():
    calls = {"n": 0}

    def raise_always(_ticker):
        calls["n"] += 1
        raise RuntimeError("boom")

    with (
        mock.patch.object(s.yf, "Ticker", side_effect=raise_always),
        mock.patch.object(s.time, "sleep"),
    ):  # no perder tiempo real reintentando
        result = s._fetch_one_market_cap("AAA")

    assert result is None
    assert calls["n"] == s.MAX_RETRIES + 1


def test_fetch_market_caps_returns_one_entry_per_ticker():
    tickers = ["AAA", "BBB", "CCC"]
    with mock.patch.object(s.yf, "Ticker", return_value=mock.Mock(fast_info=_FakeFastInfo(500e9))):
        result = s._fetch_market_caps(tickers, pause_seconds=0.0)
    assert result == {t: 500e9 for t in tickers}


# --------------------------------------------------------------------------
# _fetch_fundamentals (red mockeada)
# --------------------------------------------------------------------------


def test_fetch_fundamentals_success_and_failure():
    def fake_ticker(ticker):
        obj = mock.Mock()
        if ticker == "BAD":
            obj.get_info.side_effect = RuntimeError("no info")
        else:
            obj.get_info.return_value = {"longName": f"{ticker} Inc."}
        return obj

    with mock.patch.object(s.yf, "Ticker", side_effect=fake_ticker), mock.patch.object(s.time, "sleep"):
        result = s._fetch_fundamentals(["AAA", "BAD"], pause_seconds=0.0)

    assert result["AAA"] == {"longName": "AAA Inc."}
    assert result["BAD"] is None


# --------------------------------------------------------------------------
# _evaluate_candidates (pura, sin red)
# --------------------------------------------------------------------------


def test_evaluate_candidates_attaches_fundamentals_and_score():
    momentum_rows = [
        {
            "ticker": "AAA",
            "market_cap": 3e11,
            "price": 150.0,
            "momentum_12_1": 0.1,
            "technical": {"rsi_14": 55, "macd": None, "sma_50": None, "sma_200": None},
        },
        {
            "ticker": "NOINFO",
            "market_cap": 2e11,
            "price": 50.0,
            "momentum_12_1": -0.05,
            "technical": {},
        },
    ]
    fundamentals = {
        "AAA": {
            "longName": "AAA Inc.",
            "sector": "Technology",
            "trailingPE": 20.0,
            "returnOnEquity": 0.2,
        },
        "NOINFO": None,
    }

    result = s._evaluate_candidates(momentum_rows, fundamentals)

    aaa = next(r for r in result if r["ticker"] == "AAA")
    assert aaa["name"] == "AAA Inc."
    assert aaa["sector"] == "Technology"
    assert aaa["valuation"]["per_trailing"] == 20.0
    assert aaa["score"]["overall"] is not None

    noinfo = next(r for r in result if r["ticker"] == "NOINFO")
    assert noinfo["name"] == "NOINFO"  # fallback al ticker
    assert noinfo["sector"] == ""
    assert noinfo["valuation"] == {}


# --------------------------------------------------------------------------
# run_batch: pipeline completo, de punta a punta (red mockeada)
# --------------------------------------------------------------------------


class _FakeTicker:
    """Doble de `yf.Ticker` para las pruebas end-to-end de `run_batch`."""

    MARKET_CAPS = {"AAA": 300e9, "BBB": 50e9, "CCC": 250e9}

    def __init__(self, ticker):
        self.ticker = ticker

    @property
    def fast_info(self):
        return _FakeFastInfo(self.MARKET_CAPS[self.ticker])

    def get_info(self):
        return {
            "longName": f"{self.ticker} Inc.",
            "sector": "Technology",
            "trailingPE": 20.0,
            "returnOnEquity": 0.2,
            "profitMargins": 0.15,
            "debtToEquity": 50.0,
            "currentRatio": 1.5,
            "freeCashflow": 1e9,
            "dividendYield": 1.2,
        }


def _fake_history_frame(tickers: list[str]) -> pd.DataFrame:
    frames = {}
    for i, t in enumerate(tickers):
        closes = make_price_series(n_days=400, start=100 + i, end=200 + i)
        frames[(t, "Close")] = closes
        frames[(t, "Volume")] = make_volume_series(closes)
    df = pd.DataFrame(frames)
    df.columns = pd.MultiIndex.from_tuples(df.columns)
    return df


@pytest.fixture
def mocked_yfinance():
    tickers = list(_FakeTicker.MARKET_CAPS)
    with (
        mock.patch.object(s, "get_us_large_cap_universe", return_value=tickers),
        mock.patch.object(s.yf, "Ticker", side_effect=_FakeTicker),
        mock.patch.object(s.yf, "download", return_value=_fake_history_frame(tickers)),
    ):
        yield tickers


def test_run_batch_end_to_end(tmp_path, mocked_yfinance):
    run_dir = s.run_batch(min_market_cap=200e9, pause_seconds=0.0, output_root=tmp_path)

    result = json.loads((run_dir / "result.json").read_text(encoding="utf-8"))
    candidates = json.loads((run_dir / "candidates.json").read_text(encoding="utf-8"))
    prices = json.loads((run_dir / "prices.json").read_text(encoding="utf-8"))
    assert (run_dir / "run.log").exists()

    assert result["status"] == "done"
    assert result["error"] is None
    assert result["universe_total"] == 3
    # BBB (50B) no supera el umbral de 200B.
    assert result["candidates_total"] == 2
    assert {r["ticker"] for r in candidates["rows"]} == {"AAA", "CCC"}
    assert set(prices["prices"].keys()) == {"AAA", "CCC"}
    assert len(prices["dates"]) == len(prices["prices"]["AAA"])

    for row in candidates["rows"]:
        assert row["score"]["overall"] is not None
        assert row["name"].endswith("Inc.")
        assert "return_ytd" in row

    # El ranking final está ordenado por momentum descendente.
    momentums = [r["momentum_12_1"] for r in result["rows"]]
    assert momentums == sorted(momentums, reverse=True)
    assert len(result["rows"]) <= s.TOP_N


def test_run_batch_no_candidates_writes_empty_rows(tmp_path):
    with (
        mock.patch.object(s, "get_us_large_cap_universe", return_value=["AAA"]),
        mock.patch.object(s.yf, "Ticker", side_effect=_FakeTicker),
    ):
        # Umbral más alto que cualquier market cap simulado.
        run_dir = s.run_batch(min_market_cap=10e12, pause_seconds=0.0, output_root=tmp_path)

    result = json.loads((run_dir / "result.json").read_text(encoding="utf-8"))
    candidates = json.loads((run_dir / "candidates.json").read_text(encoding="utf-8"))

    assert result["status"] == "done"
    assert result["candidates_total"] == 0
    assert result["rows"] == []
    assert candidates["rows"] == []
    assert not (run_dir / "prices.json").exists()


def test_run_batch_records_error_status(tmp_path):
    with mock.patch.object(s, "get_us_large_cap_universe", side_effect=RuntimeError("Wikipedia caído")):
        run_dir = s.run_batch(min_market_cap=200e9, pause_seconds=0.0, output_root=tmp_path)

    result = json.loads((run_dir / "result.json").read_text(encoding="utf-8"))
    assert result["status"] == "error"
    assert "Wikipedia" in result["error"]
    assert result["rows"] == []
