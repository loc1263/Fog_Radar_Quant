"""history.db del módulo EEUU: backfill desde ``runs/`` y endpoints ``/history/*``."""

from __future__ import annotations

import json
import shutil
import sqlite3
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from fog_radar.modules.us_stocks import history
from fog_radar.settings import Settings

FIXTURE_RUN = Path(__file__).with_name("fixtures") / "runs" / "2026-09-25T21-00-01Z"
RUN_A, RUN_B, RUN_ERR = "2026-09-25T21-00-01Z", "2026-09-26T21-00-01Z", "2026-09-27T21-00-01Z"


def _json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


@pytest.fixture
def runs(settings: Settings) -> Path:
    """Corrida A (fixture real), B (candidatos en orden inverso, un día más de precios)
    y una corrida con error que el historial ignora."""
    d = settings.module_data_dir("us_stocks") / "runs"
    shutil.copytree(FIXTURE_RUN, d / RUN_A)

    b = d / RUN_B
    b.mkdir()
    cand = _json(FIXTURE_RUN / "candidates.json")
    cand["run_id"] = RUN_B
    cand["rows"] = list(reversed(cand["rows"]))
    result = {**cand, "rows": cand["rows"][: cand["top_n"]]}
    precios = _json(FIXTURE_RUN / "prices.json")
    precios["dates"] = [*precios["dates"][1:], "2099-01-01"]
    (b / "candidates.json").write_text(json.dumps(cand), encoding="utf-8")
    (b / "result.json").write_text(json.dumps(result), encoding="utf-8")
    (b / "prices.json").write_text(json.dumps(precios), encoding="utf-8")

    err = d / RUN_ERR
    err.mkdir()
    (err / "result.json").write_text(
        json.dumps({**result, "run_id": RUN_ERR, "status": "error", "rows": []}), encoding="utf-8"
    )
    return d


def test_backfill_reproduce_los_json(settings: Settings, runs: Path) -> None:
    resumen = history.backfill(settings)
    assert resumen.startswith("backfill: 2 corridas cargadas") and RUN_ERR in resumen

    con = sqlite3.connect(history.db_path(settings))
    con.row_factory = sqlite3.Row
    cand = _json(FIXTURE_RUN / "candidates.json")
    filas = con.execute(
        "SELECT c.* FROM candidates c JOIN runs r ON r.id = c.run_pk WHERE r.run_id = ? ORDER BY c.rank",
        (RUN_A,),
    ).fetchall()
    assert [f["ticker"] for f in filas] == [r["ticker"] for r in cand["rows"]]
    for f, r in zip(filas, cand["rows"], strict=True):
        assert f["momentum_12_1"] == r["momentum_12_1"]
        assert f["score"] == r["score"]["overall"]
        assert f["valuation"] == r["score"]["categories"]["valuation"]
        assert f["return_ytd"] == r["return_ytd"]
        assert bool(f["in_top"]) == (f["rank"] <= cand["top_n"])

    # Cierres deduplicados entre corridas: A ∪ B = fechas de A + 1 fecha nueva.
    precios = _json(FIXTURE_RUN / "prices.json")
    n_tickers = len(precios["prices"])
    n_no_nulos = sum(v is not None for serie in precios["prices"].values() for v in serie)
    total = con.execute("SELECT COUNT(*) FROM prices").fetchone()[0]
    assert n_no_nulos < total <= n_no_nulos + n_tickers
    ticker, serie = next(iter(precios["prices"].items()))
    assert con.execute(
        "SELECT close FROM prices WHERE ticker = ? AND date = ?", (ticker, precios["dates"][0])
    ).fetchone()[0] == pytest.approx(serie[0])


def test_backfill_es_idempotente(settings: Settings, runs: Path) -> None:
    history.backfill(settings)
    history.backfill(settings)
    con = sqlite3.connect(history.db_path(settings))
    assert con.execute("SELECT COUNT(*) FROM runs").fetchone()[0] == 2
    n = len(_json(FIXTURE_RUN / "candidates.json")["rows"])
    assert con.execute("SELECT COUNT(*) FROM candidates").fetchone()[0] == 2 * n


def test_endpoints_historial(
    client: TestClient, auth: dict[str, str], settings: Settings, runs: Path
) -> None:
    assert client.get("/api/us/history/runs", headers=auth).json() == []
    history.backfill(settings)
    cand = _json(FIXTURE_RUN / "candidates.json")
    top_n, primero, ultimo = cand["top_n"], cand["rows"][0], cand["rows"][-1]

    corridas = client.get("/api/us/history/runs", headers=auth).json()
    assert [c["run_id"] for c in corridas] == [RUN_A, RUN_B]
    assert corridas[0]["top1_ticker"] == primero["ticker"]
    assert corridas[1]["top1_ticker"] == ultimo["ticker"]

    serie = client.get(f"/api/us/history/ticker/{primero['ticker'].lower()}", headers=auth).json()
    assert [p["run_id"] for p in serie] == [RUN_A, RUN_B]
    assert serie[0]["rank"] == 1 and serie[0]["in_top"] is True
    assert serie[1]["rank"] == len(cand["rows"]) and serie[1]["in_top"] is False
    assert serie[0]["pilares"]["profitability"] == primero["score"]["categories"]["profitability"]
    assert client.get("/api/us/history/ticker/ZZZZ", headers=auth).status_code == 404

    diff = client.get("/api/us/history/diff", headers=auth).json()
    assert (diff["desde"], diff["hasta"], diff["top_n"]) == (RUN_A, RUN_B, top_n)
    estados = {f["ticker"]: f["estado"] for f in diff["filas"]}
    assert estados[primero["ticker"]] == "salio"
    assert estados[ultimo["ticker"]] == "nuevo"
    assert diff["filas"][0]["ticker"] == ultimo["ticker"]  # el top de "hasta" primero
    assert client.get(f"/api/us/history/diff?desde={RUN_A}", headers=auth).status_code == 422

    top = client.get("/api/us/history/top?ultimas=2", headers=auth).json()
    por_ticker = {t["ticker"]: t for t in top}
    assert por_ticker[ultimo["ticker"]]["racha_actual"] == 1
    assert por_ticker[primero["ticker"]]["racha_actual"] == 0
    assert por_ticker[primero["ticker"]]["veces_en_top"] == 1
    assert all(t["corridas"] == 2 for t in top)
