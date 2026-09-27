"""Lectura y escritura de los archivos de datos del módulo Chile.

Mismo formato que el proyecto original (``data/cl_stocks/``):

- ``indicadores_acciones_<YYYYMMDD_HHMMSS>.csv`` (+ ``.xlsx``): salida del extractor.
- ``ranking_acciones_<YYYYMMDD_HHMMSS>.csv`` (+ ``.xlsx``): salida del scoring.

Así se pueden copiar los CSV históricos del proyecto original sin convertir.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass
from datetime import datetime, tzinfo
from pathlib import Path

import pandas as pd

from fog_radar.core.errors import ApiError, NoDataError
from fog_radar.core.runs import latest_file
from fog_radar.modules.cl_stocks.scoring import COLUMNAS_IDENTIDAD, COLUMNAS_SCORE_PILARES
from fog_radar.settings import Settings

PATRON_INDICADORES = "indicadores_acciones_*.csv"
PATRON_RANKING = "ranking_acciones_*.csv"
_TS = re.compile(r"_(\d{8}_\d{6})\.csv$")

COLUMNAS_REQUERIDAS_RANKING: tuple[str, ...] = (
    "Ranking",
    *COLUMNAS_IDENTIDAD,
    "Score Compuesto",
    *COLUMNAS_SCORE_PILARES,
    "Métricas Disponibles",
    "Métricas Totales",
    "Grupo Financiero Duplicado",
)


def data_dir(settings: Settings) -> Path:
    return settings.module_data_dir("cl_stocks")


def timestamp_de(path: Path, tz: tzinfo | None = None) -> datetime | None:
    """Fecha/hora codificada en el nombre del archivo (hora local de ``tz``)."""
    m = _TS.search(path.name)
    if not m:
        return None
    return datetime.strptime(m.group(1), "%Y%m%d_%H%M%S").replace(tzinfo=tz)


@dataclass(frozen=True)
class Loaded:
    path: Path
    df: pd.DataFrame
    generado_en: datetime | None


class InvalidDataError(ApiError):
    """El archivo existe pero no tiene el formato esperado."""

    def __init__(self, detail: str) -> None:
        super().__init__(503, "invalid_data", detail)


def _cargar(settings: Settings, patron: str, requeridas: tuple[str, ...], que: str) -> Loaded:
    path = latest_file(data_dir(settings), patron)
    if path is None:
        raise NoDataError(
            f"Todavía no hay {que} del IPSA. Ejecuta el pipeline: python -m fog_radar.jobs cl_stocks"
        )
    try:
        df = pd.read_csv(path)
    except Exception as exc:  # CSV vacío o corrupto
        raise InvalidDataError(f"No se pudo leer {path.name}: {type(exc).__name__}") from exc
    faltantes = [c for c in requeridas if c not in df.columns]
    if faltantes:
        raise InvalidDataError(f"{path.name} no tiene las columnas: {', '.join(faltantes)}")
    return Loaded(path, df, timestamp_de(path, settings.tzinfo))


def cargar_ranking(settings: Settings) -> Loaded:
    return _cargar(settings, PATRON_RANKING, COLUMNAS_REQUERIDAS_RANKING, "un ranking")


def cargar_indicadores(settings: Settings) -> Loaded:
    return _cargar(settings, PATRON_INDICADORES, COLUMNAS_IDENTIDAD, "indicadores")


def _escribir_atomico(df: pd.DataFrame, destino: Path) -> None:
    """Escribe en un temporal y renombra: la API nunca lee un archivo a medias."""
    tmp = destino.with_name(f".{destino.name}.tmp")
    if destino.suffix == ".csv":
        df.to_csv(tmp, index=False)
    else:
        df.to_excel(tmp, index=False, engine="openpyxl")
    os.replace(tmp, destino)


def guardar(df: pd.DataFrame, directory: Path, prefijo: str, timestamp: str) -> Path:
    """Guarda ``<prefijo>_<timestamp>.xlsx`` y ``.csv`` (el CSV al final: es el que lee la API)."""
    directory.mkdir(parents=True, exist_ok=True)
    base = directory / f"{prefijo}_{timestamp}"
    _escribir_atomico(df, base.with_suffix(".xlsx"))
    _escribir_atomico(df, base.with_suffix(".csv"))
    return base.with_suffix(".csv")
