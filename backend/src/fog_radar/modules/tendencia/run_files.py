"""Archivos del módulo Tendencia que lee la API (``data/tendencia``) — el "storage" de ADR 0002
(el nombre ``storage`` ya lo usa el paquete del original con la base SQLite).

- ``output/<YYYYMMDD>_<NN>/``: una carpeta por corrida (``dashboard_data.json``,
  CSV/JSON de Markowitz, tendencia y momentum, informe PDF).
- ``history.db``: base SQLite con todas las corridas (vista Historial).

La API nunca importa ``config`` (que carga el universo y crea carpetas): arma
las rutas desde ``Settings``.
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

from fog_radar.core.errors import ApiError, NoDataError
from fog_radar.core.runs import list_run_dirs, read_json
from fog_radar.settings import Settings

RUN_DIR_PATTERN = r"^\d{8}_\d{2}$"
_RUN_DIR_RE = re.compile(RUN_DIR_PATTERN)
DASHBOARD = "dashboard_data.json"
SIN_CORRIDAS = "Todavía no hay corridas de Tendencia. Ejecuta: python -m fog_radar.jobs tendencia"

# Archivos de una corrida que se pueden descargar (nombre plano, sin subcarpetas).
TIPOS_DESCARGABLES = {
    ".pdf": "application/pdf",
    ".csv": "text/csv; charset=utf-8",
    ".json": "application/json",
    ".txt": "text/plain; charset=utf-8",
}


def module_dir(settings: Settings) -> Path:
    return settings.module_data_dir("tendencia")


def output_dir(settings: Settings) -> Path:
    return module_dir(settings) / "output"


def history_db(settings: Settings) -> Path:
    return module_dir(settings) / "history.db"


def run_dirs(settings: Settings) -> list[Path]:
    """Corridas terminadas (con ``dashboard_data.json``), la más reciente primero."""
    return [
        d
        for d in list_run_dirs(output_dir(settings))
        if _RUN_DIR_RE.match(d.name) and (d / DASHBOARD).is_file()
    ]


def run_dir(settings: Settings, name: str) -> Path:
    if not _RUN_DIR_RE.match(name):
        raise ApiError(404, "not_found", f"'{name}' no es un nombre de corrida válido (YYYYMMDD_NN).")
    d = output_dir(settings) / name
    if not (d / DASHBOARD).is_file():
        raise ApiError(404, "not_found", f"No existe la corrida '{name}'.")
    return d


def latest_run(settings: Settings) -> Path:
    dirs = run_dirs(settings)
    if not dirs:
        raise NoDataError(SIN_CORRIDAS)
    return dirs[0]


def read_dashboard(d: Path) -> dict[str, Any]:
    try:
        return read_json(d / DASHBOARD)
    except ValueError as exc:
        raise ApiError(503, "invalid_data", f"{d.name}/{DASHBOARD} está corrupto.") from exc


def archivos(d: Path) -> list[dict[str, Any]]:
    return [
        {"nombre": f.name, "tipo": TIPOS_DESCARGABLES[f.suffix.lower()], "bytes": f.stat().st_size}
        for f in sorted(d.iterdir())
        if f.is_file() and f.suffix.lower() in TIPOS_DESCARGABLES and not f.name.startswith(".")
    ]


def archivo(d: Path, nombre: str) -> Path:
    """Archivo descargable de una corrida; rechaza rutas y extensiones no permitidas."""
    if (
        "/" in nombre
        or "\\" in nombre
        or nombre.startswith(".")
        or Path(nombre).suffix.lower() not in (TIPOS_DESCARGABLES)
    ):
        raise ApiError(404, "not_found", f"Archivo no disponible: {nombre}")
    path = d / nombre
    if not path.is_file():
        raise ApiError(404, "not_found", f"La corrida {d.name} no tiene {nombre}.")
    return path


def excluidos(settings: Settings) -> list[dict[str, str]]:
    """Tickers que el último armado del dataset dejó fuera (TD-OPS-2).

    ``processed/excluded_tickers.txt`` del original: ``ticker<TAB>nombre<TAB>motivo``
    sin encabezado. Vacío si el pipeline no corrió nunca.
    """
    path = module_dir(settings) / "processed" / "excluded_tickers.txt"
    if not path.is_file():
        return []
    filas = []
    for linea in path.read_text(encoding="utf-8").splitlines():
        partes = linea.split("\t")
        if len(partes) >= 3 and partes[0].strip():
            filas.append(
                {"ticker": partes[0].strip(), "nombre": partes[1].strip(), "motivo": partes[2].strip()}
            )
    return filas
