"""history.db del módulo ETF: backfill desde ``hist/`` + ``current/`` y endpoints ``/history/*``.

Con los fixtures, el historial tiene 2 corridas completas: la archivada del 22-09
(``hist/etf_ranking_completo_20260922.csv``) y la vigente del 24-09.
"""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

import pandas as pd
import pytest
from fastapi.testclient import TestClient

from fog_radar.modules.etf import history, pipeline, storage
from fog_radar.settings import Settings

from .conftest import FIXTURES

PREV, VIGENTE = "20260922", "20260924"


def test_clave_de_la_vigente_sigue_el_esquema_de_archivado(settings: Settings, etf_corrida: Path) -> None:
    assert history.corrida_vigente(settings).run_key == VIGENTE
    # Si ya hubiera una corrida de esa fecha archivada, la vigente es la "_2".
    (storage.hist_dir(settings) / f"etf_ranking_completo_{VIGENTE}.csv").write_text("Ticker\n")
    assert history.corrida_vigente(settings).run_key == f"{VIGENTE}_2"


def test_backfill_reproduce_los_csv(settings: Settings, etf_corrida: Path) -> None:
    assert history.backfill(settings) == "backfill: 2 corridas cargadas en history.db"
    con = sqlite3.connect(history.db_path(settings))

    runs = pd.read_sql("SELECT * FROM runs ORDER BY fecha", con)
    assert runs["run_key"].tolist() == [PREV, VIGENTE]

    # La fila de la vigente coincide con historial_corridas.csv del original.
    hist = pd.read_csv(etf_corrida / storage.HISTORIAL, dtype={"Fecha": str}).set_index("Fecha")
    esperado = hist.loc[VIGENTE]
    vig = runs.set_index("run_key").loc[VIGENTE]
    assert vig["universo_bruto"] == esperado["Universo_Bruto"]
    assert vig["universo_depurado"] == esperado["Universo_Depurado"]
    assert vig["n_outliers"] == esperado["N_Outliers"]
    assert vig["sharpe_promedio"] == pytest.approx(esperado["Sharpe_Promedio"], abs=5e-5)
    assert vig["beta_promedio"] == pytest.approx(esperado["Beta_Promedio"], abs=5e-5)
    assert vig["top1_ticker"] == esperado["Top1_Ticker"]

    ranking = pd.read_csv(etf_corrida / storage.RANKING)
    db = pd.read_sql(
        "SELECT k.* FROM ranking k JOIN runs r ON r.id = k.run_pk WHERE r.run_key = ? ORDER BY k.rank",
        con,
        params=(VIGENTE,),
    )
    assert db["ticker"].tolist() == ranking["Ticker"].tolist()
    for col_csv, col_db in history.RANKING_COLS.items():
        pd.testing.assert_series_equal(db[col_db], ranking[col_csv], check_names=False, check_dtype=False)
    top15 = pd.read_csv(etf_corrida / storage.TOP15)
    assert db.loc[db["en_top15"] == 1, "ticker"].tolist() == top15["Ticker"].tolist()

    excl = pd.read_csv(etf_corrida / storage.EXCLUIDOS)
    n_excl = con.execute(
        "SELECT COUNT(*) FROM excluidos x JOIN runs r ON r.id = x.run_pk WHERE r.run_key = ?", (VIGENTE,)
    ).fetchone()[0]
    assert n_excl == len(excl)
    n_cat = con.execute(
        "SELECT COUNT(*) FROM categorias c JOIN runs r ON r.id = c.run_pk WHERE r.run_key = ?", (VIGENTE,)
    ).fetchone()[0]
    assert n_cat == len(pd.read_csv(etf_corrida / storage.CATEGORIAS))


def test_backfill_es_idempotente(settings: Settings, etf_corrida: Path) -> None:
    history.backfill(settings)
    history.backfill(settings)
    con = sqlite3.connect(history.db_path(settings))
    assert con.execute("SELECT COUNT(*) FROM runs").fetchone()[0] == 2


def test_solo_analisis_reemplaza_la_vigente(settings: Settings, etf_corrida: Path) -> None:
    pipeline.run(settings, solo_analisis=True)
    pipeline.run(settings, solo_analisis=True)
    assert [c["run_key"] for c in history.corridas(settings)] == [VIGENTE]


def test_diff_coincide_con_la_rotacion_del_original(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    history.backfill(settings)
    diff = client.get("/api/etf/history/diff", headers=auth).json()
    assert (diff["desde"], diff["hasta"], diff["top_n"]) == (PREV, VIGENTE, 15)

    rot = json.loads((FIXTURES / "expected" / "rotacion.json").read_text(encoding="utf-8"))
    por_estado: dict[str, set[str]] = {}
    for f in diff["filas"]:
        por_estado.setdefault(f["estado"], set()).add(f["ticker"])
    assert por_estado["sigue"] == {r["Ticker"] for r in rot["mantienen"]}
    assert por_estado["nuevo"] == {r["Ticker"] for r in rot["nuevos"]}
    assert por_estado["salio"] == {r["Ticker"] for r in rot["salidas"]}
    filas = {f["ticker"]: f for f in diff["filas"]}
    for r in rot["mantienen"]:
        assert filas[r["Ticker"]]["delta_score"] == pytest.approx(r["Delta_Score"], abs=1e-6)


def test_endpoints_historial(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    assert client.get("/api/etf/history/runs", headers=auth).json() == []
    history.backfill(settings)

    runs = client.get("/api/etf/history/runs", headers=auth).json()
    assert [r["run_key"] for r in runs] == [PREV, VIGENTE]
    assert runs[1]["fecha"] == "2026-09-24"

    top1 = runs[1]["top1_ticker"]
    serie = client.get(f"/api/etf/history/ticker/{top1.lower()}", headers=auth).json()
    assert serie[-1]["run_key"] == VIGENTE and serie[-1]["rank"] == 1 and serie[-1]["en_top15"] is True
    assert client.get("/api/etf/history/ticker/ZZZZZ", headers=auth).status_code == 404

    top = client.get("/api/etf/history/top?ultimas=2", headers=auth).json()
    assert {t["ticker"] for t in top if t["rank_ultima"] is not None} == set(
        pd.read_csv(etf_corrida / storage.TOP15)["Ticker"]
    )
    assert all(t["racha_actual"] in (0, 1, 2) for t in top)

    cats = client.get("/api/etf/history/categorias", headers=auth).json()
    assert sum(c["en_top15"] for c in cats if c["run_key"] == VIGENTE) <= 15
    assert client.get("/api/etf/history/diff?desde=20260922", headers=auth).status_code == 422
