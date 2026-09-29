"""
Tests de api.app del original, sobre /api/tendencia (Fog Radar) — sobre una base sqlite temporal (tmp_path,
vía el fixture `client` de más abajo), nunca la real (data/history.db).
Usa TestClient (httpx), que corre la app in-process sin levantar un
puerto real.
"""

import pytest
from fastapi.testclient import TestClient

from fog_radar.modules.tendencia.storage import history_writer

from .test_history_writer import _sample_payload


@pytest.fixture
def client(settings, auth):
    """
    Fog Radar: la API es la app unificada (prefijo /api/tendencia, API key) y lee
    data/tendencia/history.db de los Settings del test (carpeta temporal). El
    TestClient lleva el prefijo en base_url y la API key por defecto, así las
    aserciones del original quedan igual.
    """
    from fog_radar.main import create_app

    with TestClient(create_app(settings), base_url="http://testserver/api/tendencia", headers=auth) as c:
        c.db_path = str(settings.module_data_dir("tendencia") / "history.db")
        yield c


def test_runs_empty_db_returns_empty_list(client):
    resp = client.get("/runs")
    assert resp.status_code == 200
    assert resp.json() == []


def test_analyses_and_runs_after_storing_a_run(client, monkeypatch):
    db_path = client.db_path
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    analyses = client.get("/analyses").json()
    assert analyses == [{"analysis_key": "", "label": "Asia + EEUU"}]

    runs = client.get("/runs", params={"analysis": ""}).json()
    assert len(runs) == 1
    assert runs[0]["run_dir"] == "20260921_01"
    assert runs[0]["sharpe_portafolio"] == 2.5


def test_runs_date_filters(client):
    db_path = client.db_path
    history_writer.store_run(_sample_payload(run_dir="20260914_01"), db_path=db_path)
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    only_latest = client.get("/runs", params={"analysis": "", "from": "2026-09-20"}).json()
    assert [r["run_dir"] for r in only_latest] == ["20260921_01"]


def test_get_run_detail_includes_holdings_sectors_momentum(client):
    db_path = client.db_path
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    resp = client.get("/runs/20260921_01")
    assert resp.status_code == 200
    data = resp.json()
    assert data["run_dir"] == "20260921_01"
    assert len(data["analyses"]) == 1
    analysis = data["analyses"][0]
    assert analysis["holdings"][0]["ticker"] == "NVDA"
    assert analysis["sectors"][0]["sector"] == "Technology"
    assert analysis["momentum"][0]["ticker"] == "NVDA"


def test_get_run_detail_404_for_unknown_run(client):
    resp = client.get("/runs/20990101_01")
    assert resp.status_code == 404
    # Fog Radar: un nombre que no es YYYYMMDD_NN se rechaza antes de ir a la base.
    assert client.get("/runs/no-existe").status_code == 422


def test_ticker_history_round_trip(client):
    db_path = client.db_path
    history_writer.store_run(_sample_payload(run_dir="20260914_01"), db_path=db_path)
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    resp = client.get("/history/ticker/NVDA", params={"analysis": ""})
    assert resp.status_code == 200
    points = resp.json()
    assert [p["run_date"] for p in points] == ["2026-09-14", "2026-09-21"]


def test_ticker_history_404_when_never_selected(client):
    db_path = client.db_path
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    resp = client.get("/history/ticker/NOEXISTE", params={"analysis": ""})
    assert resp.status_code == 404


def test_sector_history(client):
    db_path = client.db_path
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    resp = client.get("/history/sectors", params={"analysis": ""})
    assert resp.status_code == 200
    assert resp.json()[0]["sector"] == "Technology"


def test_momentum_leaders(client):
    db_path = client.db_path
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    resp = client.get("/history/momentum-leaders", params={"analysis": "", "top": 5})
    assert resp.status_code == 200
    assert resp.json()[0]["ticker"] == "NVDA"
