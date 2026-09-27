"""Utilidades para encontrar la "última corrida" y coordinar acceso a datos.

Cada módulo guarda sus salidas distinto (CSV con timestamp, carpetas por
corrida, JSON). Estas funciones cubren los casos comunes para que los
routers no repitan la lógica de "buscar lo más reciente".
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Literal

from filelock import FileLock

SortKey = Literal["name", "mtime"]


def _sorted(paths: list[Path], key: SortKey) -> list[Path]:
    if key == "mtime":
        return sorted(paths, key=lambda p: p.stat().st_mtime, reverse=True)
    # Los nombres con timestamp (YYYYMMDD_HHMMSS, YYYYMMDD_NN) ordenan bien alfabéticamente.
    return sorted(paths, key=lambda p: p.name, reverse=True)


def list_files(directory: Path, pattern: str, *, key: SortKey = "name") -> list[Path]:
    """Archivos que calzan con ``pattern``, del más reciente al más antiguo."""
    if not directory.is_dir():
        return []
    return _sorted([p for p in directory.glob(pattern) if p.is_file()], key)


def latest_file(directory: Path, pattern: str, *, key: SortKey = "name") -> Path | None:
    files = list_files(directory, pattern, key=key)
    return files[0] if files else None


def list_run_dirs(directory: Path, pattern: str = "*", *, key: SortKey = "name") -> list[Path]:
    """Subcarpetas de corridas (p. ej. ``runs/<run_id>/``), más reciente primero."""
    if not directory.is_dir():
        return []
    return _sorted([p for p in directory.glob(pattern) if p.is_dir() and not p.name.startswith(".")], key)


def latest_run_dir(directory: Path, pattern: str = "*", *, key: SortKey = "name") -> Path | None:
    dirs = list_run_dirs(directory, pattern, key=key)
    return dirs[0] if dirs else None


def read_json(path: Path) -> Any:
    with path.open(encoding="utf-8") as fh:
        return json.load(fh)


def data_lock(directory: Path, *, timeout: float = 30.0) -> FileLock:
    """Lock entre procesos sobre la carpeta de datos de un módulo.

    Lo usan el pipeline (al escribir) y la API (al leer archivos que se
    reemplazan en bloque), como hacía ETF_Choice con ``filelock``.
    """
    directory.mkdir(parents=True, exist_ok=True)
    return FileLock(str(directory / ".lock"), timeout=timeout)
