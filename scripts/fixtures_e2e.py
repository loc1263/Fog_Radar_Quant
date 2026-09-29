"""Instala en data/ los fixtures de los tests del backend para correr el sitio (E2E, CI, demo).

    cd backend && uv run python ../scripts/fixtures_e2e.py [--data-dir ../data]

Copia las corridas reales usadas en los tests de paridad de cada módulo y, para
Tendencia, arma history.db con el backfill. Descomprime los ``.gz``. No borra
nada: si un archivo ya existe, se reemplaza.
"""

from __future__ import annotations

import argparse
import gzip
import shutil
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
TESTS = REPO / "backend" / "tests"


def copiar(origen: Path, destino: Path, patron: str = "*") -> None:
    for f in origen.rglob(patron):
        if f.is_dir():
            continue
        dst = destino / f.relative_to(origen)
        dst.parent.mkdir(parents=True, exist_ok=True)
        if f.suffix == ".gz":
            with gzip.open(f, "rb") as src, dst.with_suffix("").open("wb") as out:
                shutil.copyfileobj(src, out)
        else:
            shutil.copy2(f, dst)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--data-dir", type=Path, default=REPO / "data")
    data: Path = parser.parse_args().data_dir.resolve()

    copiar(TESTS / "cl_stocks" / "fixtures", data / "cl_stocks", "*_20260925_183129.csv")
    copiar(TESTS / "us_stocks" / "fixtures" / "runs", data / "us_stocks" / "runs")
    etf = TESTS / "etf" / "fixtures"
    copiar(etf / "hist", data / "etf" / "hist")
    copiar(etf / "input", data / "etf" / "current")
    for f in (etf / "expected").iterdir():
        if f.suffix in (".csv", ".gz"):
            copiar(f.parent, data / "etf" / "current", f.name)
    td = TESTS / "tendencia" / "fixtures"
    copiar(td / "data", data / "tendencia")
    copiar(td / "expected", data / "tendencia" / "output")

    # history.db de Tendencia a partir de la corrida copiada.
    env = {"DATA_DIR": str(data), "ENV": "dev"}
    import os

    subprocess.run(
        [sys.executable, "-m", "fog_radar.jobs", "tendencia", "--backfill"],
        check=True,
        env={**os.environ, **env},
    )
    print(f"Fixtures instalados en {data}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
