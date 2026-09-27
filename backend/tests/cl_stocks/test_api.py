from __future__ import annotations

import shutil
from pathlib import Path

import pandas as pd
import pytest
from fastapi.testclient import TestClient

from fog_radar.core.jobs import JobRegistry
from fog_radar.jobs import __main__ as cli
from fog_radar.modules.cl_stocks import pipeline
from fog_radar.settings import Settings

FIXTURES = Path(__file__).with_name("fixtures")
TS = "20260925_183129"


@pytest.fixture
def cl_dir(settings: Settings) -> Path:
    d = settings.module_data_dir("cl_stocks")
    d.mkdir(parents=True)
    return d


def _copiar(cl_dir: Path, *nombres: str) -> None:
    for n in nombres:
        shutil.copy(FIXTURES / n, cl_dir / n)


def test_sin_datos_da_503_no_data(client: TestClient, auth: dict[str, str], cl_dir: Path) -> None:
    for path in ["/api/cl/ranking", "/api/cl/indicadores"]:
        r = client.get(path, headers=auth)
        assert r.status_code == 503 and r.json()["code"] == "no_data", path


def test_requiere_api_key(client: TestClient) -> None:
    assert client.get("/api/cl/ranking").status_code == 401


def test_ranking(client: TestClient, auth: dict[str, str], cl_dir: Path) -> None:
    _copiar(cl_dir, f"ranking_acciones_{TS}.csv", "ranking_acciones_20260924_221437.csv")
    body = client.get("/api/cl/ranking", headers=auth).json()
    assert body["generado_desde"] == f"ranking_acciones_{TS}.csv"  # el más reciente
    assert body["generado_en"] == "2026-09-25T18:31:29-03:00"
    assert len(body["filas"]) == 30
    primera = body["filas"][0]
    assert primera["ranking"] == 1 and primera["simbolo"] == "VAPORES.SN"
    assert primera["score_compuesto"] == pytest.approx(0.6121)
    assert primera["pilares"]["crecimiento"] is None  # NaN -> null
    assert set(primera["metricas"]) >= {"trailing_pe", "ret_1a", "dividend_yield"}
    pilares = body["modelo"]["pilares"]
    assert [p["key"] for p in pilares] == [
        "valor",
        "calidad",
        "crecimiento",
        "salud_financiera",
        "dividendo",
        "momentum",
    ]
    assert {p["key"]: p["peso"] for p in pilares}["momentum"] == 2.0
    assert sum(len(p["metricas"]) for p in pilares) == 20


def test_indicadores(client: TestClient, auth: dict[str, str], cl_dir: Path) -> None:
    _copiar(cl_dir, f"indicadores_acciones_{TS}.csv")
    body = client.get("/api/cl/indicadores", headers=auth).json()
    assert len(body["filas"]) == 30 and len(body["columnas"]) == 61
    assert body["grupos"][0] == "Identificación"
    col = {c["key"]: c for c in body["columnas"]}
    assert col["roe"]["unidad"] == "pct" and col["dividend_yield"]["unidad"] == "pct_pts"
    fila = body["filas"][0]
    assert fila["simbolo"] == "AGUAS-A.SN"
    assert fila["valores"]["moneda_cotizacion"] == "CLP"
    assert set(fila["valores"]) == set(col)


def test_csv_con_columnas_faltantes_da_503_invalid_data(
    client: TestClient, auth: dict[str, str], cl_dir: Path
) -> None:
    pd.DataFrame({"Símbolo": ["A.SN"], "Score Compuesto": [1.0]}).to_csv(
        cl_dir / "ranking_acciones_20990101_000000.csv", index=False
    )
    r = client.get("/api/cl/ranking", headers=auth)
    assert r.status_code == 503
    assert r.json()["code"] == "invalid_data" and "Sector" in r.json()["detail"]


def test_csv_vacio_da_503(client: TestClient, auth: dict[str, str], cl_dir: Path) -> None:
    (cl_dir / "indicadores_acciones_20990101_000000.csv").write_text("")
    r = client.get("/api/cl/indicadores", headers=auth)
    assert r.status_code == 503 and r.json()["code"] == "invalid_data"


def test_pipeline_solo_scoring(settings: Settings, cl_dir: Path) -> None:
    _copiar(cl_dir, f"indicadores_acciones_{TS}.csv")
    resumen = pipeline.run(settings, solo_scoring=True)
    assert "top 3: VAPORES.SN, ITAUCL.SN, SQM-B.SN" in resumen
    generados = sorted(p.name for p in cl_dir.glob("ranking_acciones_*"))
    assert [Path(n).suffix for n in generados] == [".csv", ".xlsx"]
    nuevo = pd.read_csv(cl_dir / generados[0])
    esperado = pd.read_csv(FIXTURES / f"ranking_acciones_{TS}.csv")
    pd.testing.assert_frame_equal(nuevo, esperado)
    assert not list(cl_dir.glob(".*.tmp"))


def test_pipeline_completo_con_extractor_simulado(
    settings: Settings, cl_dir: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    indicadores = pd.read_csv(FIXTURES / f"indicadores_acciones_{TS}.csv")
    monkeypatch.setattr(pipeline, "extraer_indicadores", lambda tickers: (indicadores, ["X.SN"]))
    resumen = pipeline.run(settings)
    assert resumen.startswith("30/30 tickers · sin datos: X.SN")
    assert len(list(cl_dir.glob("indicadores_acciones_*.csv"))) == 1
    assert len(list(cl_dir.glob("ranking_acciones_*.csv"))) == 1


def test_cli_registra_job_y_status(
    client: TestClient,
    auth: dict[str, str],
    settings: Settings,
    cl_dir: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    _copiar(cl_dir, f"indicadores_acciones_{TS}.csv")
    monkeypatch.setattr(cli, "get_settings", lambda: settings)
    assert cli.main(["cl_stocks", "--solo-scoring"]) == 0
    job = JobRegistry(settings.jobs_dir).latest("cl_stocks")
    assert job is not None and job.status == "success" and "solo scoring" in (job.message or "")
    status = client.get("/api/cl/status", headers=auth).json()
    assert status["migrated"] is True and status["data_available"] is True
    assert status["last_success"]["id"] == job.id


def test_cli_rechaza_opcion_de_otro_modulo(monkeypatch: pytest.MonkeyPatch, settings: Settings) -> None:
    monkeypatch.setattr(cli, "get_settings", lambda: settings)
    assert cli.main(["etf", "--solo-scoring"]) == 2  # etf aún no portado
