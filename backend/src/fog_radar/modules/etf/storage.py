"""Archivos del módulo ETF y ejecución del análisis sobre ellos.

Disposición en disco (mismo formato de archivo que ``ETF_Choice/WEB/backend``)::

    data/etf/current/    etf_perf_data_<yyyymmdd>.csv + outputs de la corrida vigente
                         (= backend/data/ del original)
    data/etf/hist/       corridas archivadas con sufijo _<yyyymmdd>[_n]
                         (= backend/data_hist/ del original)
    data/etf/screener/   salidas crudas del screener (etf_perf_data_<ts>.csv, xlsx,
                         etf_choice_*, List_All_ETF_valid_*, incidencias_*)
                         y screener/data_old/ con las de más de 30 días

La API solo lee ``current`` (y ``hist`` para la rotación). El pipeline escribe en
una carpeta de *staging* y recién si el análisis terminó sin errores archiva la
corrida vigente e instala la nueva, todo bajo el mismo lock que usan los GET.
"""

from __future__ import annotations

import os
import re
import shutil
import uuid
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import date, datetime
from pathlib import Path
from typing import Any

import pandas as pd
from filelock import FileLock, Timeout

from fog_radar.core.errors import ApiError, NoDataError
from fog_radar.core.logging import get_logger
from fog_radar.modules.etf.analysis import (
    archivado,
    etf_categoria_analysis,
    etf_momentum_analysis,
    etf_ranking_analysis,
    etf_russell3000_momentum,
)
from fog_radar.settings import Settings

log = get_logger("etf.storage")

PATRON_PERF = "etf_perf_data_*.csv"
TOP15 = "etf_top15_recomendados.csv"
RANKING = "etf_ranking_completo.csv"
EXCLUIDOS = "etf_excluidos_outliers.csv"
TOP15_SIMPLE = "etf_top15_simple.csv"
MOMENTUM_TOP15 = "etf_momentum_top15_score_comp.csv"
CATEGORIAS = "etf_categorias_ranking.csv"
CAP_PORTFOLIO = "etf_russell3000_portfolio.csv"
CAP_BUCKETS = "etf_russell3000_resumen_buckets.csv"
HISTORIAL = "historial_corridas.csv"

# Segundos de espera por el lock antes de responder 503 (igual que el original).
LOCK_TIMEOUT = 15
SIN_CORRIDA = "Todavía no hay una corrida procesada. Ejecuta: python -m fog_radar.jobs etf"
_FECHA = re.compile(r"(\d{8})")


class BusyError(ApiError):
    def __init__(self) -> None:
        super().__init__(
            503, "busy", "El backend está actualizando los datos de ETFs; reintenta en unos segundos."
        )


# --- Rutas ---------------------------------------------------------------------
def module_dir(settings: Settings) -> Path:
    return settings.module_data_dir("etf")


def current_dir(settings: Settings) -> Path:
    return module_dir(settings) / "current"


def hist_dir(settings: Settings) -> Path:
    return module_dir(settings) / "hist"


def screener_dir(settings: Settings) -> Path:
    return module_dir(settings) / "screener"


def _lock_path(settings: Settings) -> Path:
    d = current_dir(settings)
    d.mkdir(parents=True, exist_ok=True)
    return d / ".datos.lock"


@contextmanager
def bloqueo_datos(settings: Settings, timeout: float | None = None) -> Iterator[None]:
    """Serializa el acceso a ``current`` entre la API y el pipeline (lock de SO:
    si el proceso muere, se libera solo). ``Timeout`` → 503 ``busy``."""
    try:
        with FileLock(str(_lock_path(settings)), timeout=LOCK_TIMEOUT if timeout is None else timeout):
            yield
    except Timeout as exc:
        raise BusyError() from exc


# --- Lectura -------------------------------------------------------------------
def csv_actual(directory: Path) -> Path | None:
    candidatos = sorted(directory.glob(PATRON_PERF)) if directory.is_dir() else []
    return candidatos[-1] if candidatos else None


def fecha_de(nombre: str | int | None) -> date | None:
    """``etf_perf_data_20260924.csv`` / ``20260924`` → ``date(2026, 9, 24)``."""
    if nombre is None:
        return None
    m = _FECHA.search(str(nombre))
    if not m:
        return None
    try:
        return datetime.strptime(m.group(1), "%Y%m%d").date()
    except ValueError:
        return None


def leer(settings: Settings, nombre: str) -> pd.DataFrame | None:
    """CSV de la corrida vigente, o ``None`` si no existe (llamar dentro del lock)."""
    ruta = current_dir(settings) / nombre
    if not ruta.is_file():
        return None
    try:
        return pd.read_csv(ruta)
    except Exception as exc:
        raise ApiError(503, "invalid_data", f"No se pudo leer {nombre}: {type(exc).__name__}") from exc


def leer_requerido(settings: Settings, nombre: str, detalle: str = SIN_CORRIDA) -> pd.DataFrame:
    df = leer(settings, nombre)
    if df is None:
        raise NoDataError(detalle)
    return df


def generado_en(settings: Settings, nombre: str = TOP15) -> datetime | None:
    ruta = current_dir(settings) / nombre
    if not ruta.is_file():
        return None
    return datetime.fromtimestamp(ruta.stat().st_mtime, tz=settings.tzinfo)


def hay_corrida_anterior(settings: Settings) -> bool:
    d = hist_dir(settings)
    return d.is_dir() and any(d.glob("etf_top15_recomendados*.csv"))


# --- Análisis (port de main.correr_pipeline del original) ----------------------
def correr_analisis(data_dir: Path, data_hist_dir: Path) -> dict[str, Any]:
    """Corre los 4 análisis sobre el ``etf_perf_data_*.csv`` de ``data_dir`` y escribe
    los outputs en esa misma carpeta. Momentum, Categoría y Top 15 Cap. no bloquean el
    resto si fallan (mismo criterio que el original)."""
    ruta_csv = csv_actual(data_dir)
    if ruta_csv is None:
        raise FileNotFoundError(f"No hay ningún {PATRON_PERF} en {data_dir}.")
    ruta = str(ruta_csv)

    resultado = etf_ranking_analysis.run(ruta)
    avisos: list[str] = []
    try:
        etf_momentum_analysis.run(ruta, data_hist_dir=str(data_hist_dir))
    except Exception as exc:
        log.warning("Análisis de Momentum falló, se omite: %s", exc)
        avisos.append("momentum")
    try:
        etf_categoria_analysis.run(str(data_dir / RANKING))
    except Exception as exc:
        log.warning("Análisis de Categoría falló, se omite: %s", exc)
        avisos.append("categorías")
    try:
        etf_russell3000_momentum.run(ruta)
    except Exception as exc:
        log.warning("Análisis Top 15 Cap. falló, se omite: %s", exc)
        avisos.append("top 15 cap.")
    resultado["avisos"] = avisos
    return resultado


def preparar_staging(settings: Settings, origen_csv: Path, fecha_tag: str) -> Path:
    """Carpeta temporal (en el mismo disco que ``current``, para poder renombrar) con el
    CSV nuevo ya con su nombre final y una copia del historial vigente."""
    base = module_dir(settings)
    base.mkdir(parents=True, exist_ok=True)
    # No usar tempfile.mkdtemp: en Windows (Python ≥ 3.12) crea la carpeta con una ACL sin herencia y los
    # archivos instalados desde ahí quedan ilegibles para el usuario (PermissionError en la corrida siguiente).
    staging = base / f".staging_{uuid.uuid4().hex[:8]}"
    staging.mkdir()
    shutil.copy2(origen_csv, staging / f"etf_perf_data_{fecha_tag}.csv")
    with bloqueo_datos(settings, timeout=60):
        historial = current_dir(settings) / HISTORIAL
        if historial.is_file():
            shutil.copy2(historial, staging / HISTORIAL)
    return staging


def instalar_desde_staging(settings: Settings, staging: Path, *, archivar: bool) -> list[str]:
    """Bajo el lock: (opcional) archiva la corrida vigente en ``hist`` y mueve todo lo
    de ``staging`` a ``current``. Devuelve los nombres instalados."""
    destino = current_dir(settings)
    destino.mkdir(parents=True, exist_ok=True)
    instalados: list[str] = []
    with bloqueo_datos(settings, timeout=60):
        if archivar:
            movidos = archivado.archivar_corrida(str(destino), str(hist_dir(settings)))
            for m in movidos:
                log.info("archivado: %s", Path(m).name)
            if not movidos:
                log.info("(no había corrida vigente para archivar)")
        elif (nuevo := csv_actual(staging)) is not None:
            # Re-análisis: si el CSV vigente tuviera otro nombre, no dejar dos.
            for viejo in destino.glob(PATRON_PERF):
                if viejo.name != nuevo.name:
                    viejo.unlink()
        for origen in sorted(staging.iterdir()):
            os.replace(origen, destino / origen.name)
            instalados.append(origen.name)
    return instalados
