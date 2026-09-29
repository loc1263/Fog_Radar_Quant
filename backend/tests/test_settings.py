from __future__ import annotations

from pathlib import Path

import pytest
from pydantic import SecretStr, ValidationError

from fog_radar.settings import BACKEND_DIR, Settings


def test_origins_accept_comma_separated(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ALLOWED_ORIGINS", "https://a.cl, https://b.cl")
    s = Settings(_env_file=None)
    assert s.allowed_origins == ["https://a.cl", "https://b.cl"]


def test_relative_data_dir_resolves_from_backend() -> None:
    s = Settings(_env_file=None, data_dir=Path("../data"))
    assert s.data_dir == (BACKEND_DIR / "../data").resolve()
    assert s.module_data_dir("etf") == s.data_dir / "etf"


CLAVE_OK = "k3J9-vQ2_xT7pL0mZ8rW4yN6bH1dF5sA"


def test_prod_requires_long_api_key() -> None:
    with pytest.raises(ValidationError, match="API_KEY"):
        Settings(_env_file=None, env="prod", allowed_origins=["https://quant.example.cl"])
    with pytest.raises(ValidationError, match="API_KEY"):
        Settings(_env_file=None, env="prod", api_key=SecretStr("corta"), allowed_origins=["https://x.cl"])
    # Placeholder de .env.example y claves de baja entropía.
    for mala in ("cambiar-por-una-clave-larga-aleatoria", "x" * 40):
        with pytest.raises(ValidationError, match="API_KEY"):
            Settings(_env_file=None, env="prod", api_key=SecretStr(mala), allowed_origins=["https://x.cl"])


def test_prod_rejects_localhost_origins() -> None:
    with pytest.raises(ValidationError, match="localhost"):
        Settings(_env_file=None, env="prod", api_key=SecretStr(CLAVE_OK))


def test_prod_ok() -> None:
    s = Settings(_env_file=None, env="prod", api_key=SecretStr(CLAVE_OK), allowed_origins=["https://x.cl"])
    assert s.is_prod
