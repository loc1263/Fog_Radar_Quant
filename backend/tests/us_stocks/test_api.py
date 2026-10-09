"""API del módulo EEUU. Portado de ``Stock_Choice_EEUU/backend/tests/test_main.py``
y adaptado al sitio unificado (prefijo ``/api/us``, API key, errores ``{detail, code}``,
``503 no_data`` en vez de 404 cuando no hay corridas)."""

from __future__ import annotations

import json
import shutil
from collections.abc import Iterator
from pathlib import Path
from unittest import mock

import numpy as np
import pandas as pd
import pytest
from fastapi.testclient import TestClient

from fog_radar.core.jobs import JobRegistry
from fog_radar.jobs import __main__ as cli
from fog_radar.modules.us_stocks import pipeline, router, ticker_eval
from fog_radar.modules.us_stocks.ticker_eval import TickerNotFoundError
from fog_radar.settings import Settings

FIXTURE_RUN = Path(__file__).with_name("fixtures") / "runs" / "2026-09-25T21-00-01Z"


@pytest.fixture
def runs(settings: Settings) -> Path:
    d = settings.module_data_dir("us_stocks") / "runs"
    d.mkdir(parents=True)
    return d


@pytest.fixture(autouse=True)
def _clear_cache() -> Iterator[None]:
    router.clear_evaluation_cache()
    yield
    router.clear_evaluation_cache()


def _meta(run_id: str) -> dict:
    return {
        "run_id": run_id,
        "status": "done",
        "error": None,
        "started_at": "2025-01-01T00:00:00+00:00",
        "finished_at": "2025-01-01T00:10:00+00:00",
        "min_market_cap": 2e11,
        "pause_seconds": 0.6,
        "universe_total": 500,
        "candidates_total": 2,
        "top_n": 15,
    }


def _row(ticker: str, **extra: object) -> dict:
    return {"ticker": ticker, "market_cap": 3e11, "price": 10.0, "momentum_12_1": 0.5, **extra}


def _write_run(run_dir: Path, *, result=None, candidates=None, prices=None, with_log=True) -> None:
    run_dir.mkdir(parents=True, exist_ok=True)
    if with_log:
        (run_dir / "run.log").write_text("log", encoding="utf-8")
    for name, data in (("result.json", result), ("candidates.json", candidates), ("prices.json", prices)):
        if data is not None:
            (run_dir / name).write_text(json.dumps(data), encoding="utf-8")


# --- Corridas --------------------------------------------------------------------


def test_requiere_api_key(client: TestClient) -> None:
    assert client.get("/api/us/rankings/latest").status_code == 401


@pytest.mark.parametrize(
    "path",
    ["/api/us/rankings/latest", "/api/us/rankings/latest/candidates", "/api/us/rankings/latest/prices"],
)
def test_sin_corridas_503_no_data(client: TestClient, auth: dict, runs: Path, path: str) -> None:
    res = client.get(path, headers=auth)
    assert res.status_code == 503 and res.json()["code"] == "no_data"


def test_latest_devuelve_la_corrida_terminada_mas_reciente(
    client: TestClient, auth: dict, runs: Path
) -> None:
    _write_run(runs / "2025-01-01T00-00-00Z", result={**_meta("old"), "rows": []})
    _write_run(runs / "2025-01-02T00-00-00Z", result={**_meta("new"), "rows": [_row("AAA")]})
    _write_run(runs / "2025-01-03T00-00-00Z", result=None)  # en curso: solo run.log
    body = client.get("/api/us/rankings/latest", headers=auth).json()
    assert body["run_id"] == "new"
    row = body["rows"][0]
    assert row["ticker"] == "AAA"
    # Bloques ausentes en el JSON se completan con nulos (contrato estable para el front)
    assert row["valuation"]["per_trailing"] is None and row["score"]["label"] == "Sin datos"


def test_candidates_falta_archivo(client: TestClient, auth: dict, runs: Path) -> None:
    _write_run(runs / "2025-01-01T00-00-00Z", result={**_meta("r1"), "rows": []})
    res = client.get("/api/us/rankings/latest/candidates", headers=auth)
    assert res.status_code == 503 and "candidates.json" in res.json()["detail"]


def test_candidates_de_una_corrida_real(client: TestClient, auth: dict, runs: Path) -> None:
    shutil.copytree(FIXTURE_RUN, runs / FIXTURE_RUN.name)
    body = client.get("/api/us/rankings/latest/candidates", headers=auth).json()
    original = json.loads((FIXTURE_RUN / "candidates.json").read_text(encoding="utf-8"))
    assert len(body["rows"]) == len(original["rows"])
    assert body["rows"][0]["score"] == original["rows"][0]["score"]
    assert body["started_at"].startswith("2026-09-25T21:00:01")


def test_prices_recorte(client: TestClient, auth: dict, runs: Path) -> None:
    prices = {
        "dates": ["2025-01-01", "2025-01-02", "2025-01-03"],
        "prices": {"AAA": [1.0, 2.0, 3.0], "BBB": [4.0, None, 6.0]},
    }
    _write_run(runs / "2025-01-01T00-00-00Z", result={**_meta("r1"), "rows": []}, prices=prices)
    assert client.get("/api/us/rankings/latest/prices", headers=auth).json() == prices
    body = client.get("/api/us/rankings/latest/prices?days=2&tickers=bbb", headers=auth).json()
    assert body == {"dates": ["2025-01-02", "2025-01-03"], "prices": {"BBB": [None, 6.0]}}


def test_runs_ordenadas(client: TestClient, auth: dict, runs: Path) -> None:
    for d in ("2025-01-01T00-00-00Z", "2025-01-03T00-00-00Z", "2025-01-02T00-00-00Z"):
        _write_run(runs / d, result={**_meta(d), "rows": []})
    assert client.get("/api/us/rankings/runs", headers=auth).json()["runs"] == [
        "2025-01-03T00-00-00Z",
        "2025-01-02T00-00-00Z",
        "2025-01-01T00-00-00Z",
    ]


# --- Evaluación en vivo ----------------------------------------------------------------

FAKE_EVAL = {
    "ticker": "AAPL",
    "name": "Apple",
    "sector": "Technology",
    "industry": "Consumer Electronics",
    "currency": "USD",
    "price": 200.0,
    "market_cap": 3e12,
    "fetched_at": "2026-09-27T12:00:00+00:00",
    "valuation": {},
    "profitability": {},
    "financial_health": {},
    "shareholder_return": {},
    "technical": {},
    "score": {"overall": 66.1, "label": "Sólido", "categories": {}},
}


def test_evaluate_ok_y_cache(client: TestClient, auth: dict) -> None:
    with mock.patch.object(ticker_eval, "evaluate_ticker", return_value=FAKE_EVAL) as ev:
        first = client.get("/api/us/tickers/aapl/evaluate", headers=auth)
        second = client.get("/api/us/tickers/AAPL/evaluate", headers=auth)
    assert first.status_code == 200 and first.json()["score"]["overall"] == 66.1
    assert first.json()["cached"] is False and second.json()["cached"] is True
    ev.assert_called_once_with("AAPL")


def test_evaluate_no_encontrado(client: TestClient, auth: dict) -> None:
    with mock.patch.object(ticker_eval, "evaluate_ticker", side_effect=TickerNotFoundError("no existe")):
        res = client.get("/api/us/tickers/ZZZZ/evaluate", headers=auth)
    assert res.status_code == 404 and res.json()["code"] == "ticker_not_found"


def test_evaluate_error_de_yahoo(client: TestClient, auth: dict) -> None:
    with mock.patch.object(ticker_eval, "evaluate_ticker", side_effect=RuntimeError("Yahoo caído")):
        res = client.get("/api/us/tickers/AAPL/evaluate", headers=auth)
    assert res.status_code == 502 and res.json()["code"] == "upstream_error"


@pytest.mark.parametrize("bad", ["AAPL;DROP", "A" * 16, "../etc"])
def test_evaluate_valida_ticker(client: TestClient, auth: dict, bad: str) -> None:
    with mock.patch.object(ticker_eval, "evaluate_ticker") as ev:
        res = client.get(f"/api/us/tickers/{bad}/evaluate", headers=auth)
    assert res.status_code in (404, 422)
    ev.assert_not_called()


def test_evaluate_rate_limit(settings: Settings, auth: dict) -> None:
    from fog_radar.core.ratelimit import reset_rate_limits
    from fog_radar.main import create_app

    reset_rate_limits()
    s = settings.model_copy(update={"rate_limit_in_tests": True})
    with (
        TestClient(create_app(s)) as c,
        mock.patch.object(ticker_eval, "evaluate_ticker", return_value=FAKE_EVAL),
    ):
        codes = [c.get(f"/api/us/tickers/T{i}/evaluate", headers=auth).status_code for i in range(21)]
    reset_rate_limits()
    assert codes[:20] == [200] * 20 and codes[20] == 429


# --- Portafolio -------------------------------------------------------------------------


def _synthetic_prices(tickers: list[str], n_days: int = 80, seed: int = 7) -> dict:
    rng = np.random.default_rng(seed)
    dates = pd.bdate_range(end="2025-06-30", periods=n_days)
    prices = {}
    for i, t in enumerate(tickers):
        daily = rng.normal(loc=0.0005 * (i + 1), scale=0.01 + 0.002 * i, size=n_days)
        prices[t] = (100 * (1 + daily).cumprod()).round(2).tolist()
    return {"dates": [d.strftime("%Y-%m-%d") for d in dates], "prices": prices}


def _candidates(tickers: list[str]) -> dict:
    return {
        **_meta("r1"),
        "rows": [
            _row(t, market_cap=3e11 + i * 1e10, score={"overall": 50.0 + i * 10})
            for i, t in enumerate(tickers)
        ],
    }


def _portfolio_run(runs: Path, tickers: list[str], seed: int = 7) -> None:
    _write_run(
        runs / "2025-06-30T00-00-00Z",
        result={**_meta("r1"), "rows": []},
        candidates=_candidates(tickers),
        prices=_synthetic_prices(tickers, seed=seed),
    )


WIDE = ["AAA", "BBB", "CCC", "DDD", "EEE", "FFF"]


def test_optimize(client: TestClient, auth: dict, runs: Path) -> None:
    _portfolio_run(runs, WIDE, seed=13)
    res = client.post("/api/us/portfolio/optimize", json={"tickers": WIDE, "n_assets": 4}, headers=auth)
    assert res.status_code == 200
    body = res.json()
    assert body["universe"] == WIDE and len(body["selection"]) == 4
    chosen = {x["ticker"] for x in body["selection"]}
    assert set(body["weights"]) == chosen == set(body["views"])
    assert all(0 <= w <= 0.5 + 1e-6 for w in body["weights"].values())  # tope 25% -> sube a 1/4 si hace falta
    assert sum(body["weights"].values()) == pytest.approx(1.0, abs=1e-6)


def test_optimize_views_manuales(client: TestClient, auth: dict, runs: Path) -> None:
    _portfolio_run(runs, WIDE, seed=13)
    payload = {"tickers": WIDE, "n_assets": 6, "views": {"aaa": 0.9}}
    body = client.post("/api/us/portfolio/optimize", json=payload, headers=auth).json()
    assert body["views"]["AAA"] == pytest.approx(0.9)


def test_optimize_sin_corridas(client: TestClient, auth: dict, runs: Path) -> None:
    res = client.post("/api/us/portfolio/optimize", json={"tickers": ["AAA", "BBB"]}, headers=auth)
    assert res.status_code == 503


def test_optimize_ticker_sin_precios(client: TestClient, auth: dict, runs: Path) -> None:
    _portfolio_run(runs, ["AAA", "BBB", "CCC"])
    res = client.post("/api/us/portfolio/optimize", json={"tickers": ["AAA", "ZZZZ"]}, headers=auth)
    assert (
        res.status_code == 400 and res.json()["code"] == "portfolio_data" and "ZZZZ" in res.json()["detail"]
    )


def test_optimize_un_solo_ticker(client: TestClient, auth: dict, runs: Path) -> None:
    _portfolio_run(runs, ["AAA", "BBB", "CCC"])
    res = client.post("/api/us/portfolio/optimize", json={"tickers": ["AAA"]}, headers=auth)
    assert res.status_code == 400


def test_optimize_ticker_invalido_422(client: TestClient, auth: dict, runs: Path) -> None:
    res = client.post("/api/us/portfolio/optimize", json={"tickers": ["AAA", "B B"]}, headers=auth)
    assert res.status_code == 422 and res.json()["code"] == "validation_error"


def test_limits(client: TestClient, auth: dict) -> None:
    body = client.get("/api/us/portfolio/limits", headers=auth).json()
    assert body["default_assets"] == 10 and body["default_max_weight"] == 0.25


# --- Pipeline / CLI ---------------------------------------------------------------------


def test_pipeline_registra_job(
    client: TestClient, auth: dict, settings: Settings, runs: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    def fake_run_batch(*, output_root: Path) -> Path:
        dest = output_root / FIXTURE_RUN.name
        shutil.copytree(FIXTURE_RUN, dest)
        return dest

    monkeypatch.setattr(pipeline.scanner, "run_batch", fake_run_batch)
    monkeypatch.setattr(cli, "get_settings", lambda: settings)
    assert cli.main(["us_stocks"]) == 0
    job = JobRegistry(settings.jobs_dir).latest("us_stocks")
    assert job is not None and job.status == "success"
    assert job.message is not None and "top 3:" in job.message
    status = client.get("/api/us/status", headers=auth).json()
    assert status["migrated"] is True and status["data_available"] is True


def test_pipeline_error_de_corrida(settings: Settings, runs: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    def failed_run(*, output_root: Path) -> Path:
        d = output_root / "2025-01-01T00-00-00Z"
        _write_run(d, result={**_meta("x"), "status": "error", "error": "Yahoo bloqueó", "rows": []})
        return d

    monkeypatch.setattr(pipeline.scanner, "run_batch", failed_run)
    with pytest.raises(RuntimeError, match="Yahoo bloqueó"):
        pipeline.run(settings)
