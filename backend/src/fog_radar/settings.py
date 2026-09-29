"""Configuración central (pydantic-settings).

Se lee de variables de entorno y de ``backend/.env`` (no versionado; ver
``backend/.env.example``). Todas las rutas de datos salen de acá: ningún
módulo arma rutas a mano.
"""

from __future__ import annotations

from functools import lru_cache
from pathlib import Path
from typing import Annotated, Literal
from zoneinfo import ZoneInfo

from pydantic import Field, SecretStr, field_validator, model_validator
from pydantic_settings import BaseSettings, NoDecode, SettingsConfigDict

BACKEND_DIR = Path(__file__).resolve().parents[2]
REPO_ROOT = BACKEND_DIR.parent

ModuleKey = Literal["etf", "us_stocks", "cl_stocks", "tendencia"]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=BACKEND_DIR / ".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    env: Literal["dev", "test", "prod"] = "dev"
    app_name: str = "Fog Radar Quant API"
    log_level: str = "INFO"

    # --- Seguridad ---
    api_key: SecretStr | None = None
    allowed_origins: Annotated[list[str], NoDecode] = Field(default_factory=lambda: ["http://localhost:5173"])
    # Red de seguridad global por usuario final (ver core/ratelimit.py).
    rate_limit_default: str = "300/minute"
    rate_limit_in_tests: bool = False  # los tests de rate limit lo activan

    # --- Datos ---
    data_dir: Path = REPO_ROOT / "data"
    # Zona horaria de los timestamps en nombres de archivo (los pipelines corren para Chile).
    timezone: str = "America/Santiago"

    # --- Corridas batch (fog_radar.jobs.runner) ---
    # Pausa entre un pipeline y el siguiente de una secuencia (respiro para Yahoo Finance).
    pipeline_pausa_s: int = 60
    # Reintentos de un pipeline que falla en una corrida programada (US-DATA-04) y espera entre intentos.
    pipeline_reintentos: int = 1
    pipeline_espera_reintento_s: int = 600
    # Cuánto espera una corrida programada si otra secuencia tiene el lock (p. ej. un ETF que se alargó).
    pipeline_lock_espera_s: int = 6 * 3600
    # Webhook (Slack/Discord/Teams) que recibe un aviso cuando un pipeline falla (US-DEVOPS-04).
    alert_webhook_url: SecretStr | None = None

    # --- Fuentes externas ---
    fred_api_key: SecretStr | None = None

    @field_validator("allowed_origins", mode="before")
    @classmethod
    def _split_origins(cls, v: object) -> object:
        """Acepta ``"a,b"`` además de una lista."""
        if isinstance(v, str):
            return [o.strip() for o in v.split(",") if o.strip()]
        return v

    @field_validator("data_dir", mode="after")
    @classmethod
    def _resolve_data_dir(cls, v: Path) -> Path:
        # Rutas relativas se interpretan desde backend/ (igual que .env).
        return v if v.is_absolute() else (BACKEND_DIR / v).resolve()

    @model_validator(mode="after")
    def _prod_requires_key(self) -> Settings:
        if self.env == "prod":
            if self.api_key is None or len(self.api_key.get_secret_value()) < 24:
                raise ValueError("En prod API_KEY es obligatoria y debe tener al menos 24 caracteres.")
            if any("localhost" in o or "127.0.0.1" in o for o in self.allowed_origins):
                raise ValueError("En prod ALLOWED_ORIGINS no puede incluir localhost.")
        return self

    # --- Helpers ---
    @property
    def is_prod(self) -> bool:
        return self.env == "prod"

    def module_data_dir(self, module: ModuleKey) -> Path:
        """Carpeta de datos de un módulo (``data/<modulo>``)."""
        return self.data_dir / module

    @property
    def tzinfo(self) -> ZoneInfo:
        return ZoneInfo(self.timezone)

    @property
    def jobs_dir(self) -> Path:
        return self.data_dir / "_jobs"


@lru_cache
def get_settings() -> Settings:
    return Settings()
