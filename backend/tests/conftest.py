from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path

import pytest
from fastapi.testclient import TestClient
from pydantic import SecretStr

from fog_radar.main import create_app
from fog_radar.settings import Settings

TEST_KEY = "test-key-0123456789abcdef"


@pytest.fixture
def settings(tmp_path: Path) -> Settings:
    return Settings(_env_file=None, env="test", api_key=SecretStr(TEST_KEY), data_dir=tmp_path / "data")


@pytest.fixture
def client(settings: Settings) -> Iterator[TestClient]:
    with TestClient(create_app(settings), raise_server_exceptions=False) as c:
        yield c


@pytest.fixture
def auth() -> dict[str, str]:
    return {"X-API-Key": TEST_KEY}
