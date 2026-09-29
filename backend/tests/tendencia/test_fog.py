"""Integración con Fog Radar: endpoints nuevos (/latest, archivos de la corrida),
pipeline por CLI (python -m fog_radar.jobs tendencia) y backfill."""

from __future__ import annotations

import importlib
import os
import sqlite3
from collections.abc import Iterator
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from fog_radar.core.jobs import JobRegistry
from fog_radar.jobs import __main__ as cli
from fog_radar.modules.tendencia import paths
from fog_radar.settings import Settings

from .helpers import FIXTURES, RUN, copiar_arbol, instalar_corrida


@pytest.fixture
def datos(settings: Settings) -> Path:
    root = settings.module_data_dir("tendencia")
    instalar_corrida(root)
    return root


@pytest.fixture
def raiz_temporal(settings: Settings, _tendencia_root: Path) -> Iterator[Path]:
    """`config` apuntando a data/tendencia de los Settings del test."""
    from fog_radar.modules.tendencia import config

    root = settings.module_data_dir("tendencia")
    paths.set_root(root)
    importlib.reload(config)
    yield root
    paths.set_root(_tendencia_root)
    importlib.reload(config)


def test_sin_corridas_503(client: TestClient, auth: dict[str, str]) -> None:
    r = client.get("/api/tendencia/latest", headers=auth)
    assert r.status_code == 503
    assert r.json()["code"] == "no_data"
    # Sin history.db las listas vienen vacías (como el original con una base vacía).
    assert client.get("/api/tendencia/runs", headers=auth).json() == []
    assert client.get("/api/tendencia/analyses", headers=auth).json() == []


def test_latest(client: TestClient, auth: dict[str, str], datos: Path) -> None:
    body = client.get("/api/tendencia/latest", headers=auth).json()
    assert body["run_dir"] == RUN
    assert [a["key"] for a in body["analyses"]] == ["", "_us"]
    principal = body["analyses"][0]
    assert principal["label"] == "Asia + EEUU"
    assert len(principal["holdings"]) == 15
    assert [o["n"] for o in principal["portfolio_options"]] == [5, 10]
    assert principal["momentum_semanal"][0]["momentum_1s_pct"] is not None
    assert body["parametros"]["tamanos_portafolio"] == [5, 10]
    assert {"nombre": "top15_seleccion.csv", "tipo": "text/csv; charset=utf-8"}.items() <= {
        k: v for k, v in next(a for a in body["archivos"] if a["nombre"] == "top15_seleccion.csv").items()
    }.items()


def test_latest_ignora_corrida_sin_dashboard(client: TestClient, auth: dict[str, str], datos: Path) -> None:
    (datos / "output" / "20991231_01").mkdir()  # corrida en curso: sin dashboard_data.json
    assert client.get("/api/tendencia/latest", headers=auth).json()["run_dir"] == RUN


def test_corrida_antigua_sin_bloques_nuevos(client: TestClient, auth: dict[str, str], datos: Path) -> None:
    import json

    viejo = datos / "output" / "20260921_01"
    viejo.mkdir()
    payload = json.loads((datos / "output" / RUN / "dashboard_data.json").read_text(encoding="utf-8"))
    for a in payload["analyses"]:
        a.pop("momentum_semanal")
        a.pop("portfolio_options")
        for h in a["holdings"]:
            h.pop("senal_tecnica_3m")
    (viejo / "dashboard_data.json").write_text(json.dumps(payload), encoding="utf-8")
    body = client.get("/api/tendencia/runs/20260921_01/dashboard", headers=auth).json()
    assert body["analyses"][0]["portfolio_options"] == []
    assert body["analyses"][0]["holdings"][0]["senal_tecnica_3m"] is None


def test_archivos_de_la_corrida(client: TestClient, auth: dict[str, str], datos: Path) -> None:
    (datos / "output" / RUN / "informe_portafolio_20260926.pdf").write_bytes(b"%PDF-1.4 prueba")
    base = f"/api/tendencia/runs/{RUN}/files"
    nombres = [a["nombre"] for a in client.get(base, headers=auth).json()]
    assert "informe_portafolio_20260926.pdf" in nombres
    r = client.get(f"{base}/informe_portafolio_20260926.pdf", headers=auth)
    assert r.status_code == 200
    assert r.headers["content-type"] == "application/pdf"
    assert r.content.startswith(b"%PDF")
    for malo in ("..%2F..%2Fhistory.db", "history.db", ".oculto.csv", "no_existe.csv"):
        assert client.get(f"{base}/{malo}", headers=auth).status_code == 404, malo
    assert client.get("/api/tendencia/runs/20260926_99/files", headers=auth).status_code == 404


def test_backfill_y_historial(
    client: TestClient, auth: dict[str, str], settings: Settings, datos: Path, raiz_temporal: Path
) -> None:
    from fog_radar.modules.tendencia import pipeline

    resumen = pipeline.run(settings, backfill=True)
    assert resumen.startswith("backfill: 1 corridas cargadas")
    runs = client.get("/api/tendencia/runs", params={"analysis": "_us"}, headers=auth).json()
    assert [r["run_dir"] for r in runs] == [RUN]
    # generated_at conserva la fecha original de la corrida.
    assert runs[0]["generated_at"].startswith("2026-09-26T21:01")
    leaders = client.get("/api/tendencia/history/momentum-leaders", params={"top": 3}, headers=auth).json()
    assert len(leaders) == 3


def test_pipeline_sin_yahoo(settings: Settings, raiz_temporal: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Pipeline completo con --skip-fetch: el descubrimiento y el armado del dataset se
    reemplazan (usan Yahoo / los CSV crudos); el resto corre de verdad, PDF incluido."""
    copiar_arbol(FIXTURES / "data", raiz_temporal)
    from fog_radar.modules.tendencia import pipeline
    from fog_radar.modules.tendencia.ingestion import build_dataset
    from fog_radar.modules.tendencia.universe import discovery

    monkeypatch.setattr(discovery, "main", lambda force=False: None)
    monkeypatch.setattr(build_dataset, "main", lambda: None)

    resumen = pipeline.run(settings, skip_fetch=True)
    assert resumen.startswith("corrida ")
    assert "Asia + EEUU: Sharpe" in resumen and "solo EEUU: Sharpe" in resumen
    run_dir = next((raiz_temporal / "output").iterdir())
    assert (run_dir / "dashboard_data.json").is_file()
    assert list(run_dir.glob("informe_portafolio_*.pdf"))
    with sqlite3.connect(raiz_temporal / "history.db") as conn:
        assert conn.execute("SELECT COUNT(*) FROM runs").fetchone()[0] == 1


def test_cli_opciones(settings: Settings, datos: Path, raiz_temporal: Path) -> None:
    from fog_radar import settings as settings_mod

    settings_mod.get_settings.cache_clear()
    os.environ["DATA_DIR"] = str(settings.data_dir)
    os.environ["ENV"] = "test"
    try:
        assert cli.main(["tendencia", "--backfill"]) == 0
        assert cli.main(["tendencia", "--solo-analisis"]) == 2  # opción de ETF
    finally:
        del os.environ["DATA_DIR"], os.environ["ENV"]
        settings_mod.get_settings.cache_clear()
    rec = JobRegistry(settings.jobs_dir).latest("tendencia")
    assert rec is not None and rec.status == "success"
    assert rec.message and rec.message.startswith("backfill")


def test_excluidos_por_api(client: TestClient, auth: dict[str, str], datos: Path) -> None:
    (datos / "processed" / "excluded_tickers.txt").write_text(
        "SPCX\tSpace Exploration Technologies Corp.\thistoria insuficiente (16 semanas)\n"
        "7203.T\tToyota\tfalta FX para JPY\n",
        encoding="utf-8",
    )
    filas = client.get("/api/tendencia/excluidos", headers=auth).json()
    assert filas[1] == {"ticker": "7203.T", "nombre": "Toyota", "motivo": "falta FX para JPY"}
    assert len(client.get("/api/tendencia/latest", headers=auth).json()["excluidos"]) == 2


def test_validacion_de_configuracion(raiz_temporal: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    from fog_radar.modules.tendencia import config

    assert config.validar_configuracion() == []
    monkeypatch.setattr(config, "MAX_WEIGHT_PER_ASSET", 0.05)
    problemas = config.validar_configuracion()
    assert any("no pueden sumar 100 %" in p for p in problemas)
    monkeypatch.setattr(config, "MAX_WEIGHT_PER_ASSET", 0.15)
    monkeypatch.setattr(config, "PORTFOLIO_SIZE_OPTIONS", (5, 15))
    assert any("portafolio reducido de 15" in p for p in config.validar_configuracion())


def test_pipeline_rechaza_configuracion_no_factible(
    settings: Settings, raiz_temporal: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from fog_radar.modules.tendencia import config, pipeline

    monkeypatch.setattr(config, "validar_configuracion", lambda: ["[x] problema"])
    monkeypatch.setattr(importlib, "reload", lambda m: m)
    with pytest.raises(ValueError, match="no factible"):
        pipeline.run(settings)
