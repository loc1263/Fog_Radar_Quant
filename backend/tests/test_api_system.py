from __future__ import annotations

from fastapi import APIRouter
from fastapi.testclient import TestClient
from pydantic import SecretStr

from fog_radar.core.jobs import JobRegistry
from fog_radar.main import create_app
from fog_radar.settings import Settings

MODULE_PREFIXES = ["cl", "us", "etf", "tendencia"]


def test_health_is_public(client: TestClient) -> None:
    r = client.get("/api/health")
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "ok" and body["env"] == "test"
    assert "X-Process-Time-ms" in r.headers


def test_protected_endpoints_require_key(client: TestClient, auth: dict[str, str]) -> None:
    for path in ["/api/status", "/api/jobs", *[f"/api/{p}/status" for p in MODULE_PREFIXES]]:
        r = client.get(path)
        assert r.status_code == 401, path
        assert r.json() == {"detail": "API key inválida o ausente.", "code": "unauthorized"}
        assert client.get(path, headers={"X-API-Key": "incorrecta"}).status_code == 401
        assert client.get(path, headers=auth).status_code == 200, path


def test_status_lists_four_modules(client: TestClient, auth: dict[str, str]) -> None:
    body = client.get("/api/status", headers=auth).json()
    keys = [m["key"] for m in body["modules"]]
    assert keys == ["cl_stocks", "us_stocks", "etf", "tendencia"]
    assert {m["key"]: m["migrated"] for m in body["modules"]} == {
        "cl_stocks": True,  # Fase 3
        "us_stocks": True,  # Fase 4
        "etf": False,
        "tendencia": False,
    }
    assert all(m["data_available"] is False for m in body["modules"])
    assert body["modules"][0]["api_prefix"] == "/api/cl"


def test_module_status_reflects_data_and_jobs(
    client: TestClient, auth: dict[str, str], settings: Settings
) -> None:
    d = settings.module_data_dir("etf")
    d.mkdir(parents=True)
    (d / "etf_perf_data_20260927.csv").write_text("a,b\n1,2\n")
    with JobRegistry(settings.jobs_dir).track("etf"):
        pass
    body = client.get("/api/etf/status", headers=auth).json()
    assert body["data_available"] is True
    assert body["latest_job"]["status"] == "success"
    assert body["last_success"]["id"] == body["latest_job"]["id"]


def test_jobs_endpoints(client: TestClient, auth: dict[str, str]) -> None:
    latest = client.get("/api/jobs", headers=auth).json()
    assert set(latest) == {"cl_stocks", "us_stocks", "etf", "tendencia"}
    assert client.get("/api/jobs/etf", headers=auth).json() == []
    r = client.get("/api/jobs/noexiste", headers=auth)
    assert r.status_code == 404 and r.json()["code"] == "unknown_module"
    r = client.get("/api/jobs/etf?limit=0", headers=auth)
    assert r.status_code == 422 and r.json()["code"] == "validation_error"


def test_unknown_route_uses_error_format(client: TestClient) -> None:
    r = client.get("/api/no-existe")
    assert r.status_code == 404
    assert r.json()["code"] == "not_found"


def _app_with_boom(settings: Settings) -> TestClient:
    app = create_app(settings)
    boom = APIRouter()

    @boom.get("/api/boom")
    def _boom() -> None:
        raise RuntimeError("detalle interno")

    app.include_router(boom)
    return TestClient(app, raise_server_exceptions=False)


def test_unhandled_error_exposes_detail_outside_prod(settings: Settings) -> None:
    r = _app_with_boom(settings).get("/api/boom")
    assert r.status_code == 500
    assert r.json() == {"detail": "RuntimeError: detalle interno", "code": "internal_error"}


def test_prod_hides_internal_errors_and_docs(tmp_path) -> None:
    prod = Settings(
        _env_file=None,
        env="prod",
        api_key=SecretStr("k" * 32),
        allowed_origins=["https://quant.example.cl"],
        data_dir=tmp_path,
    )
    c = _app_with_boom(prod)
    r = c.get("/api/boom")
    assert r.json() == {"detail": "Error interno del servidor.", "code": "internal_error"}
    assert c.get("/docs").status_code == 404
    assert c.get("/openapi.json").status_code == 404


def test_docs_available_in_dev(client: TestClient) -> None:
    assert client.get("/docs").status_code == 200
    paths = client.get("/openapi.json").json()["paths"]
    assert {f"/api/{p}/status" for p in MODULE_PREFIXES} <= set(paths)


def test_cors_only_allowed_origin(client: TestClient) -> None:
    ok = client.options(
        "/api/health",
        headers={"Origin": "http://localhost:5173", "Access-Control-Request-Method": "GET"},
    )
    assert ok.headers.get("access-control-allow-origin") == "http://localhost:5173"
    bad = client.options(
        "/api/health",
        headers={"Origin": "https://malo.example", "Access-Control-Request-Method": "GET"},
    )
    assert "access-control-allow-origin" not in bad.headers


def test_dev_without_key_is_open(tmp_path) -> None:
    s = Settings(_env_file=None, env="dev", api_key=None, data_dir=tmp_path)
    with TestClient(create_app(s)) as c:
        assert c.get("/api/status").status_code == 200
