"""
Fixtures compartidas. Como settings.py crea directorios (data/, output/)
y carga el universo al importarse, y varios tests necesitan un
`config.BASE_DIR` aislado (para no leer/escribir en el proyecto real),
estas fixtures usan `importlib.reload` sobre un `BASE_DIR` temporal
monkeypatcheado, replicando el mismo patrón que usa
`tendencia.pipeline` en producción para refrescar el universo tras
`universe.discovery`.
"""

import importlib
from collections.abc import Iterator
from pathlib import Path

import pytest

from fog_radar.modules.tendencia import paths


@pytest.fixture(autouse=True, scope="session")
def _tendencia_root(tmp_path_factory: pytest.TempPathFactory) -> Iterator[Path]:
    """Fog Radar: `config` crea carpetas y lee el universo al importarse. Para que
    ningún test toque data/tendencia del repo, la raíz del módulo apunta a una
    carpeta temporal durante toda la sesión."""
    root = tmp_path_factory.mktemp("tendencia")
    paths.set_root(root)
    from fog_radar.modules.tendencia import config

    importlib.reload(config)
    yield root
    paths.set_root(None)


@pytest.fixture
def fresh_settings(tmp_path, monkeypatch):
    """
    Devuelve el módulo `tendencia.config` recargado con
    `BASE_DIR` apuntando a un directorio temporal vacío (sin snapshot de
    universo), es decir: cae al universo FALLBACK_* determinista. Se
    revierte automáticamente al terminar el test.
    """
    from fog_radar.modules.tendencia import config as settings

    monkeypatch.setattr(settings, "BASE_DIR", str(tmp_path))
    monkeypatch.setattr(settings, "UNIVERSE_DIR", str(tmp_path / "data" / "universe"))
    (tmp_path / "data" / "universe").mkdir(parents=True, exist_ok=True)
    monkeypatch.setattr(settings, "_find_universe_file", lambda: None)

    asia, us, sector = settings._load_universe()
    monkeypatch.setattr(settings, "ASIA_TICKERS", asia)
    monkeypatch.setattr(settings, "US_TICKERS", us)
    monkeypatch.setattr(settings, "SECTOR_MAP", sector)
    monkeypatch.setattr(settings, "ALL_TICKERS", {**asia, **us})

    yield settings
