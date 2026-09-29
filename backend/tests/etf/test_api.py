"""API del módulo ETF (``/api/etf``) sobre la corrida real del 24-09-2026."""

from __future__ import annotations

from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from fog_radar.core.jobs import JobRegistry
from fog_radar.core.ratelimit import reset_rate_limits
from fog_radar.modules.etf import storage
from fog_radar.settings import Settings

TABLAS = {
    "top15": 15,
    "top15-simple": 15,
    "top15-vanguard": 15,
    "ranking-completo": 683,
    "excluidos": 8,
    "momentum/top15": 15,
}


def test_sin_datos_503(client: TestClient, auth: dict[str, str]) -> None:
    for path in ("resumen", "top15", "rotacion", "russell3000", "pdf"):
        r = client.get(f"/api/etf/{path}", headers=auth)
        assert r.status_code == 503, path
        assert r.json()["code"] == "no_data"
    # El historial vacío no es un error.
    assert client.get("/api/etf/historial", headers=auth).json() == {"filas": []}


def test_requiere_api_key(client: TestClient, etf_corrida: Path) -> None:
    assert client.get("/api/etf/top15").status_code == 401


@pytest.mark.parametrize(("path", "n"), TABLAS.items())
def test_tablas(client: TestClient, auth: dict[str, str], etf_corrida: Path, path: str, n: int) -> None:
    r = client.get(f"/api/etf/{path}", headers=auth)
    assert r.status_code == 200
    body = r.json()
    assert body["fecha"] == "2026-09-24"
    assert len(body["filas"]) == n


def test_top15_campos(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    fila = client.get("/api/etf/top15", headers=auth).json()["filas"][0]
    assert fila["ticker"] == "CRAK"
    assert fila["categoria"] == "Equity Energy"
    assert fila["perf_1y"] == pytest.approx(74.34)
    assert fila["score"] == pytest.approx(88.409, abs=1e-3)
    assert fila["s_sharpe"] == 100.0
    assert "Performance 1Y (%)" not in fila


def test_vanguard_solo_vanguard_ordenado(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    filas = client.get("/api/etf/top15-vanguard", headers=auth).json()["filas"]
    assert {f["proveedor"] for f in filas} == {"Vanguard"}
    scores = [f["score"] for f in filas]
    assert scores == sorted(scores, reverse=True)


def test_excluidos_ordenados_por_z_con_signo(
    client: TestClient, auth: dict[str, str], etf_corrida: Path
) -> None:
    filas = client.get("/api/etf/excluidos", headers=auth).json()["filas"]
    zs = [f["z_score"] for f in filas]
    assert zs == sorted(zs, reverse=True)
    assert all(f["ventana_atipica"] for f in filas)


def test_resumen_y_modelo(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    body = client.get("/api/etf/resumen", headers=auth).json()
    assert body["archivo_csv"] == "etf_perf_data_20260924.csv"
    assert body["fecha"] == "2026-09-24"
    assert body["tiene_corrida_anterior"] is True
    assert body["resumen"]["universo_bruto"] == 1189
    assert body["resumen"]["top1_ticker"] == "CRAK"
    modelo = body["modelo"]
    assert modelo["top_n"] == 15
    assert sum(p["peso"] for p in modelo["pesos"]) == pytest.approx(1.0)
    assert sum(p["peso"] for p in modelo["pesos_simple"]) == pytest.approx(1.0)
    assert sum(p["peso"] for p in modelo["pesos_momentum"]) == pytest.approx(1.0)
    assert modelo["z_threshold"] == 6.0


def test_diversificacion(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    filas = client.get("/api/etf/diversificacion", headers=auth).json()["filas"]
    assert sum(f["conteo"] for f in filas) == 15
    assert sum(f["pct"] for f in filas) == pytest.approx(100.0)


def test_categorias_con_lista_de_tickers(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    body = client.get("/api/etf/categorias/ranking", headers=auth).json()
    assert body["min_n_categoria"] == 5
    primera = body["filas"][0]
    assert primera["n"] == len(primera["tickers"]) >= 5
    sharpes = [f["sharpe_promedio"] for f in body["filas"]]
    assert sharpes == sorted(sharpes, reverse=True)


def test_top15_cap(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    body = client.get("/api/etf/russell3000", headers=auth).json()
    assert body["benchmark_ticker"] == "IWV"
    assert len(body["filas"]) == 15
    assert [b["cap_bucket"] for b in body["buckets"]] == ["Large-cap", "Mid-cap", "Small-cap"]
    assert sum(f["peso_portfolio"] for f in body["filas"]) == pytest.approx(100.0)


def test_rotacion_sin_corrida_anterior(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    for f in storage.hist_dir(settings).glob("etf_top15_recomendados*"):
        f.unlink()
    body = client.get("/api/etf/rotacion", headers=auth).json()
    assert body["disponible"] is False
    assert body["mantienen"] == []


def test_historial(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    filas = client.get("/api/etf/historial", headers=auth).json()["filas"]
    assert len(filas) == 19
    assert filas[0]["fecha"] == "2026-07-10"
    assert filas[-1]["fecha"] == "2026-09-24"


def test_pdf(client: TestClient, auth: dict[str, str], etf_corrida: Path) -> None:
    r = client.get("/api/etf/pdf", headers=auth)
    assert r.status_code == 200
    assert r.headers["content-type"] == "application/pdf"
    assert 'filename="etf_informe_24092026.pdf"' in r.headers["content-disposition"]
    assert r.content.startswith(b"%PDF")
    assert (
        r.content.count(b"/Type /Page\n")
        + r.content.count(b"/Type /Page\r")
        + r.content.count(b"/Type /Page ")
        >= 7
    )


def test_run_reanaliza_y_registra_job(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    antes = (etf_corrida / storage.TOP15).read_bytes()
    r = client.post("/api/etf/run", headers=auth)
    assert r.status_code == 200, r.text
    job = r.json()["job"]
    assert job["status"] == "success"
    assert job["trigger"] == "api"
    assert "solo análisis" in job["message"]
    assert (etf_corrida / storage.TOP15).read_bytes() == antes  # mismo input → mismo output
    assert JobRegistry(settings.jobs_dir).latest("etf").status == "success"
    # No archiva: el histórico no gana archivos.
    assert not list(storage.hist_dir(settings).glob("etf_perf_data_20260924*"))


def test_run_en_curso_409(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    registry = JobRegistry(settings.jobs_dir)
    with registry.track("etf"):
        r = client.post("/api/etf/run", headers=auth)
    assert r.status_code == 409
    assert r.json()["code"] == "run_in_progress"


def test_run_respeta_el_lock_global(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    """Una corrida batch (cron o consola) en curso bloquea el re-análisis por API."""
    import threading

    from fog_radar.jobs.runner import lock_pipelines

    tomado, soltar = threading.Event(), threading.Event()

    def batch() -> None:
        with lock_pipelines(settings):
            tomado.set()
            soltar.wait(5)

    t = threading.Thread(target=batch)
    t.start()
    tomado.wait(5)
    try:
        r = client.post("/api/etf/run", headers=auth)
    finally:
        soltar.set()
        t.join()
    assert r.status_code == 409 and r.json()["code"] == "run_in_progress"


def test_run_sin_csv(client: TestClient, auth: dict[str, str]) -> None:
    r = client.post("/api/etf/run", headers=auth)
    assert r.status_code == 503
    assert r.json()["code"] == "no_data"


def test_run_rate_limit(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    settings.rate_limit_in_tests = True
    reset_rate_limits()
    try:
        codes = [client.post("/api/etf/run", headers=auth).status_code for _ in range(4)]
    finally:
        settings.rate_limit_in_tests = False
        reset_rate_limits()
    assert codes[:3] == [200, 200, 200]
    assert codes[3] == 429


def test_lock_ocupado_503(
    client: TestClient, auth: dict[str, str], settings: Settings, etf_corrida: Path
) -> None:
    from filelock import FileLock

    lock = FileLock(str(storage.current_dir(settings) / ".datos.lock"))
    storage_timeout = storage.LOCK_TIMEOUT
    storage.LOCK_TIMEOUT = 0.1
    try:
        import threading

        tomado, soltar = threading.Event(), threading.Event()

        def _otro_hilo() -> None:
            with lock:
                tomado.set()
                soltar.wait(5)

        t = threading.Thread(target=_otro_hilo)
        t.start()
        tomado.wait(5)
        r = client.get("/api/etf/top15", headers=auth)
        soltar.set()
        t.join()
    finally:
        storage.LOCK_TIMEOUT = storage_timeout
    assert r.status_code == 503
    assert r.json()["code"] == "busy"
