"""Hub con datos reales y buscador global (fog_radar.transversal)."""

from __future__ import annotations

import importlib.util
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

import pytest
from fastapi.testclient import TestClient

from fog_radar.settings import Settings
from fog_radar.transversal import servicio

SCL = ZoneInfo("America/Santiago")
REPO = Path(__file__).resolve().parents[2]


def _fixtures_e2e():  # noqa: ANN202
    spec = importlib.util.spec_from_file_location("fixtures_e2e", REPO / "scripts" / "fixtures_e2e.py")
    assert spec and spec.loader
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


@pytest.fixture
def datos(settings: Settings) -> Path:
    """Los mismos fixtures que usa el E2E (sin el backfill de history.db)."""
    fx = _fixtures_e2e()
    data = settings.data_dir
    tests = REPO / "backend" / "tests"
    fx.copiar(tests / "cl_stocks" / "fixtures", data / "cl_stocks", "*_20260925_183129.csv")
    fx.copiar(tests / "us_stocks" / "fixtures" / "runs", data / "us_stocks" / "runs")
    etf = tests / "etf" / "fixtures"
    fx.copiar(etf / "hist", data / "etf" / "hist")
    fx.copiar(etf / "input", data / "etf" / "current")
    for f in (etf / "expected").iterdir():
        if f.suffix in (".csv", ".gz"):
            fx.copiar(f.parent, data / "etf" / "current", f.name)
    td = tests / "tendencia" / "fixtures"
    fx.copiar(td / "data", data / "tendencia")
    fx.copiar(td / "expected", data / "tendencia" / "output")
    servicio.cache_indice.limpiar()
    return data


def test_frescura_segun_programa() -> None:
    # Martes 29-09-2026 20:00 → la diaria del lunes 18:00 ya debió terminar; la de hoy aún no (margen 3 h).
    ahora = datetime(2026, 9, 29, 20, 0, tzinfo=SCL)
    lunes_19 = datetime(2026, 9, 28, 19, 0, tzinfo=SCL)
    assert servicio.frescura("cl_stocks", lunes_19, ahora) == "al_dia"
    assert servicio.frescura("cl_stocks", datetime(2026, 9, 25, 19, 0, tzinfo=SCL), ahora) == "atrasado"
    assert servicio.frescura("cl_stocks", None, ahora) == "sin_datos"
    # Martes 22:00: ya pasó la ventana de la corrida de hoy → los datos del lunes quedan atrasados.
    assert servicio.frescura("cl_stocks", lunes_19, datetime(2026, 9, 29, 22, 0, tzinfo=SCL)) == "atrasado"
    # ETF semanal: datos del lunes 28 07:00 al día hasta el lunes siguiente.
    assert servicio.frescura("etf", datetime(2026, 9, 28, 7, 0, tzinfo=SCL), ahora) == "al_dia"


def test_hub_sin_datos(client: TestClient, auth: dict[str, str]) -> None:
    body = client.get("/api/hub", headers=auth).json()
    assert {m["key"] for m in body["modulos"]} == {"cl_stocks", "us_stocks", "etf", "tendencia"}
    assert all(m["frescura"] == "sin_datos" and m["top"] == [] for m in body["modulos"])


def test_hub_con_datos(client: TestClient, auth: dict[str, str], datos: Path) -> None:
    body = client.get("/api/hub", headers=auth).json()
    por = {m["key"]: m for m in body["modulos"]}
    assert por["cl_stocks"]["top"][0]["ticker"] == "VAPORES.SN"
    assert por["cl_stocks"]["unidad"] == "numero" and por["cl_stocks"]["universo"] == 30
    assert len(por["us_stocks"]["top"]) == 3 and por["us_stocks"]["unidad"] == "fraccion"
    assert por["etf"]["top"][0]["valor"] >= por["etf"]["top"][1]["valor"]
    pesos = [t["valor"] for t in por["tendencia"]["top"]]
    assert pesos == sorted(pesos, reverse=True)
    for m in body["modulos"]:
        assert m["datos_al"] is not None and m["frescura"] in ("al_dia", "atrasado")
        assert m["programa"]


def test_hub_modulo_corrupto_no_tumba_el_resto(
    client: TestClient, auth: dict[str, str], datos: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from fog_radar.modules.etf import hub as etf_hub

    def roto(settings: Settings) -> None:
        raise ValueError("CSV ilegible")

    monkeypatch.setattr(etf_hub, "resumen_hub", roto)
    por = {m["key"]: m for m in client.get("/api/hub", headers=auth).json()["modulos"]}
    assert por["etf"]["frescura"] == "error" and "CSV ilegible" in por["etf"]["mensaje"]
    assert por["cl_stocks"]["top"]


def test_buscar_por_ticker_y_nombre(client: TestClient, auth: dict[str, str], datos: Path) -> None:
    r = client.get("/api/buscar", params={"q": "vapores"}, headers=auth).json()
    assert r["resultados"][0]["ticker"] == "VAPORES.SN" and r["resultados"][0]["exacto"]
    assert r["resultados"][0]["posicion"] == 1 and r["resultados"][0]["modulo"] == "cl_stocks"

    top_us = client.get("/api/hub", headers=auth).json()["modulos"][1]["top"][0]["ticker"]
    r = client.get("/api/buscar", params={"q": top_us.lower()}, headers=auth).json()
    primero = r["resultados"][0]
    assert primero["ticker"] == top_us and primero["contexto"] == "Top 15" and primero["exacto"]

    # Por nombre (sin tildes ni mayúsculas) y sin resultados.
    assert client.get("/api/buscar", params={"q": "banco"}, headers=auth).json()["resultados"]
    assert client.get("/api/buscar", params={"q": "zzzzzz"}, headers=auth).json()["resultados"] == []


def test_buscar_valida_parametros(client: TestClient, auth: dict[str, str]) -> None:
    assert client.get("/api/buscar", params={"q": ""}, headers=auth).status_code == 422
    assert client.get("/api/buscar", params={"q": "x" * 41}, headers=auth).status_code == 422
    assert client.get("/api/buscar", params={"q": "a", "limite": 999}, headers=auth).status_code == 422
    assert client.get("/api/buscar", params={"q": "a"}).status_code == 401


def test_orden_de_resultados() -> None:
    assert servicio._base_ticker("VAPORES.SN") == "vapores"
    assert servicio._normalizar("  Compañía   Sud ") == "compania sud"
