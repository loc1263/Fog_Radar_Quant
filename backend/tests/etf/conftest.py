"""Fixtures del módulo ETF: una corrida real de ETF_Choice (24-09-2026).

``fixtures/input``: el ``etf_perf_data_20260924.csv`` y el historial de esa corrida.
``fixtures/hist``: lo mínimo del histórico que usa el análisis (perf más antiguo
para la consistencia de Momentum; Top 15 archivados y la corrida del 22-09 para la
rotación). ``fixtures/expected``: salidas del proyecto original. Los CSV grandes
van comprimidos (``.gz``) y se descomprimen al copiarlos.
"""

from __future__ import annotations

import gzip
import shutil
from pathlib import Path

import pytest

from fog_radar.modules.etf import storage
from fog_radar.settings import Settings

FIXTURES = Path(__file__).with_name("fixtures")


def copiar_fixtures(origen: Path, destino: Path) -> None:
    destino.mkdir(parents=True, exist_ok=True)
    for f in origen.iterdir():
        copiar_fixture(f, destino)


def copiar_fixture(f: Path, destino: Path) -> None:
    if f.suffix == ".gz":
        with gzip.open(f, "rb") as src, (destino / f.stem).open("wb") as dst:
            shutil.copyfileobj(src, dst)
    else:
        shutil.copy2(f, destino / f.name)


def expected(nombre: str) -> Path:
    """Ruta de una salida esperada del original (descomprimida si hace falta)."""
    plano = FIXTURES / "expected" / nombre
    if plano.exists():
        return plano
    return FIXTURES / "expected" / f"{nombre}.gz"


def leer_bytes(path: Path) -> bytes:
    if path.suffix == ".gz":
        with gzip.open(path, "rb") as fh:
            return fh.read()
    return path.read_bytes()


@pytest.fixture
def etf_hist(settings: Settings) -> Path:
    d = storage.hist_dir(settings)
    copiar_fixtures(FIXTURES / "hist", d)
    return d


@pytest.fixture
def etf_corrida(settings: Settings, etf_hist: Path) -> Path:
    """``data/etf/current`` con la corrida del 24-09 ya analizada (outputs del original)."""
    d = storage.current_dir(settings)
    copiar_fixtures(FIXTURES / "input", d)
    for f in (FIXTURES / "expected").iterdir():
        if f.suffix in (".csv", ".gz"):
            copiar_fixture(f, d)
    return d
