"""Fixtures de datos de Tendencia (corrida real 20260926_01 del proyecto original)."""

from __future__ import annotations

import gzip
import shutil
from pathlib import Path

FIXTURES = Path(__file__).with_name("fixtures")
RUN = "20260926_01"


def copiar_arbol(origen: Path, destino: Path) -> None:
    """Copia ``origen`` en ``destino`` descomprimiendo los ``.gz``."""
    for f in origen.rglob("*"):
        if f.is_dir():
            continue
        rel = f.relative_to(origen)
        dst = destino / rel
        dst.parent.mkdir(parents=True, exist_ok=True)
        if f.suffix == ".gz":
            with gzip.open(f, "rb") as src, dst.with_suffix("").open("wb") as out:
                shutil.copyfileobj(src, out)
        else:
            shutil.copy2(f, dst)


def instalar_corrida(root: Path) -> Path:
    """``root`` = data/tendencia con los insumos y la corrida esperada en output/."""
    copiar_arbol(FIXTURES / "data", root)
    run_dir = root / "output" / RUN
    copiar_arbol(FIXTURES / "expected" / RUN, run_dir)
    return run_dir
