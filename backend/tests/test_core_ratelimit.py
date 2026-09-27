from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path

import pytest
from fastapi import APIRouter, Depends
from fastapi.testclient import TestClient
from pydantic import SecretStr
from starlette.requests import Request

from fog_radar.core.ratelimit import client_key, rate_limit, reset_rate_limits
from fog_radar.main import create_app
from fog_radar.settings import Settings


def _req(headers: dict[str, str]) -> Request:
    raw = [(k.lower().encode(), v.encode()) for k, v in headers.items()]
    return Request({"type": "http", "headers": raw, "client": ("10.0.0.5", 1234)})


def test_client_key_uses_forwarded_for_from_bff() -> None:
    assert client_key(_req({"X-Forwarded-For": "200.1.2.3, 10.0.0.5"})) == "200.1.2.3"


def test_client_key_falls_back_to_remote_address() -> None:
    assert client_key(_req({})) == "10.0.0.5"


@pytest.fixture
def limited_client(tmp_path: Path) -> Iterator[TestClient]:
    reset_rate_limits()
    s = Settings(
        _env_file=None,
        env="test",
        rate_limit_in_tests=True,
        rate_limit_default="3/minute",
        api_key=SecretStr("k" * 24),
        data_dir=tmp_path,
    )
    app = create_app(s)
    extra = APIRouter()

    @extra.post("/api/caro", dependencies=[Depends(rate_limit("1/minute", scope="caro"))])
    def _caro() -> dict[str, bool]:
        return {"ok": True}

    app.include_router(extra)
    with TestClient(app) as c:
        yield c
    reset_rate_limits()


def test_default_limit_per_end_user(limited_client: TestClient) -> None:
    codes = [limited_client.get("/api/health").status_code for _ in range(4)]
    assert codes == [200, 200, 200, 429]
    r = limited_client.get("/api/health")
    assert r.json()["code"] == "rate_limited"
    # Otro usuario final (IP reenviada por el BFF) tiene su propio contador.
    assert limited_client.get("/api/health", headers={"X-Forwarded-For": "200.1.2.3"}).status_code == 200


def test_strict_limit_on_expensive_endpoint(limited_client: TestClient) -> None:
    assert limited_client.post("/api/caro").status_code == 200
    r = limited_client.post("/api/caro")
    assert r.status_code == 429 and "1/minute" in r.json()["detail"]
