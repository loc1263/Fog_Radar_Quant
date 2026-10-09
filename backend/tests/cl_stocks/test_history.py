"""history.db del módulo Chile: backfill desde los CSV de fixtures y endpoints ``/history/*``."""

from __future__ import annotations

import shutil
import sqlite3
from pathlib import Path

import pandas as pd
import pytest
from fastapi.testclient import TestClient

from fog_radar.core import histdb
from fog_radar.modules.cl_stocks import history, pipeline
from fog_radar.settings import Settings

FIXTURES = Path(__file__).with_name("fixtures")
TS_A, TS_B = "20260924_221437", "20260925_183129"


@pytest.fixture
def cl_dir(settings: Settings) -> Path:
    d = settings.module_data_dir("cl_stocks")
    d.mkdir(parents=True)
    for f in FIXTURES.glob("*.csv"):
        shutil.copy(f, d / f.name)
    return d


def test_backfill_reproduce_los_csv(settings: Settings, cl_dir: Path) -> None:
    assert history.backfill(settings) == "backfill: 2 corridas cargadas en history.db"
    con = sqlite3.connect(history.db_path(settings))
    assert con.execute("PRAGMA user_version").fetchone()[0] == len(history.MIGRACIONES)
    assert con.execute("PRAGMA journal_mode").fetchone()[0] == "wal"

    for ts in (TS_A, TS_B):
        csv = pd.read_csv(cl_dir / f"ranking_acciones_{ts}.csv")
        db = pd.read_sql(
            "SELECT k.* FROM ranking k JOIN runs r ON r.id = k.run_pk WHERE r.ts = ? ORDER BY k.rank",
            con,
            params=(ts,),
        )
        assert db["ticker"].tolist() == csv["Símbolo"].tolist()
        assert db["rank"].tolist() == csv["Ranking"].tolist()
        pd.testing.assert_series_equal(
            db["score_compuesto"], csv["Score Compuesto"], check_names=False, check_exact=True
        )
        for col_csv, col_db in history.PILARES_SQL.items():
            pd.testing.assert_series_equal(db[col_db], csv[col_csv], check_names=False, check_exact=True)

    # Indicadores en formato largo: mismo valor que la celda del CSV.
    ind = pd.read_csv(cl_dir / f"indicadores_acciones_{TS_B}.csv").set_index("Símbolo")
    valor = con.execute(
        "SELECT valor FROM indicadores i JOIN runs r ON r.id = i.run_pk "
        "WHERE r.ts = ? AND i.ticker = 'VAPORES.SN' AND i.metrica = 'profit_margin'",
        (TS_B,),
    ).fetchone()[0]
    assert valor == ind.loc["VAPORES.SN", "Profit Margin"]


def test_backfill_es_idempotente(settings: Settings, cl_dir: Path) -> None:
    history.backfill(settings)
    history.backfill(settings)
    con = sqlite3.connect(history.db_path(settings))
    assert con.execute("SELECT COUNT(*) FROM runs").fetchone()[0] == 2
    assert con.execute("SELECT COUNT(*) FROM ranking").fetchone()[0] == 60


def test_pipeline_solo_scoring_guarda_la_corrida(settings: Settings, cl_dir: Path) -> None:
    for f in cl_dir.glob("ranking_*"):
        f.unlink()
    pipeline.run(settings, solo_scoring=True)
    tss = [c["ts"] for c in history.corridas(settings)]
    assert tss == [TS_B]


def test_falla_de_la_base_no_hace_fallar_el_pipeline(
    settings: Settings, cl_dir: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    def roto(*_a: object, **_k: object) -> None:
        raise sqlite3.OperationalError("disk I/O error")

    monkeypatch.setattr(history, "guardar_corrida", roto)
    assert "top 3" in pipeline.run(settings, solo_scoring=True)


def test_endpoints_sin_base_devuelven_vacio(client: TestClient, auth: dict[str, str], cl_dir: Path) -> None:
    assert client.get("/api/cl/history/runs", headers=auth).json() == []
    assert client.get("/api/cl/history/ticker/VAPORES.SN", headers=auth).status_code == 404
    assert client.get("/api/cl/history/diff", headers=auth).status_code == 404


def test_endpoints_historial(
    client: TestClient, auth: dict[str, str], settings: Settings, cl_dir: Path
) -> None:
    history.backfill(settings)

    runs = client.get("/api/cl/history/runs", headers=auth).json()
    assert [r["ts"] for r in runs] == [TS_A, TS_B]
    assert runs[1]["top1_ticker"] == "VAPORES.SN"
    assert runs[1]["run_at"] == "2026-09-25T18:31:29-03:00"

    serie = client.get("/api/cl/history/ticker/vapores.sn", headers=auth).json()
    assert [p["ts"] for p in serie] == [TS_A, TS_B]
    assert serie[1]["rank"] == 1
    assert serie[1]["score_compuesto"] == pytest.approx(0.6121)
    assert set(serie[1]["pilares"]) == {
        "valor",
        "calidad",
        "crecimiento",
        "salud_financiera",
        "dividendo",
        "momentum",
    }

    ind = client.get("/api/cl/history/ticker/VAPORES.SN/indicador/profit_margin", headers=auth)
    assert ind.status_code == 200 and len(ind.json()) == 2
    assert client.get("/api/cl/history/ticker/VAPORES.SN/indicador/nada", headers=auth).status_code == 404

    diff = client.get("/api/cl/history/diff", headers=auth).json()
    assert (diff["desde"], diff["hasta"]) == (TS_A, TS_B)
    assert len(diff["filas"]) == 30
    assert all(f["estado"] == "sigue" for f in diff["filas"])
    primera = diff["filas"][0]
    assert primera["rank_hasta"] == 1
    assert primera["delta_rank"] == primera["rank_desde"] - 1

    assert client.get(f"/api/cl/history/diff?desde={TS_A}", headers=auth).status_code == 422
    assert (
        client.get(f"/api/cl/history/diff?desde={TS_A}&hasta=20990101_000000", headers=auth).status_code
        == 404
    )


def test_diff_corridas_con_top() -> None:
    a = [
        {"ticker": "A", "nombre": "a", "rank": 1, "score": 9.0},
        {"ticker": "B", "nombre": "b", "rank": 2, "score": 8.0},
        {"ticker": "C", "nombre": "c", "rank": 3, "score": 7.0},
    ]
    b = [
        {"ticker": "C", "nombre": "c", "rank": 1, "score": 9.5},
        {"ticker": "A", "nombre": "a", "rank": 2, "score": 8.5},
        {"ticker": "B", "nombre": "b", "rank": 3, "score": 6.0},
        {"ticker": "D", "nombre": "d", "rank": 4, "score": 5.0},
    ]
    filas = {f["ticker"]: f for f in histdb.diff_corridas(a, b, top_n=2)}
    assert filas["C"]["estado"] == "nuevo" and filas["C"]["delta_rank"] == 2
    assert filas["A"]["estado"] == "sigue" and filas["A"]["delta_score"] == pytest.approx(-0.5)
    assert filas["B"]["estado"] == "salio"
    assert filas["D"]["estado"] == "fuera" and filas["D"]["rank_desde"] is None
    assert [f["ticker"] for f in histdb.diff_corridas(a, b, top_n=2)] == ["C", "A", "B", "D"]
