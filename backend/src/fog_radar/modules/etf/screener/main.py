"""Screener de ETFs (port de ``Screener_Data_ETF_Script/src/main.py``).

Descarga desde Yahoo Finance precio, AUM, rendimientos por período y
métricas de riesgo de cada ticker de ``List_All_ETF.csv`` y escribe
``etf_perf_data_<timestamp>.csv`` (+ xlsx, ``etf_choice_*``,
``List_All_ETF_valid_*`` e ``incidencias_*``) en ``output_dir``.

Diferencias con el original (solo de integración, no de lógica):

- Se importa como paquete (imports relativos). En ETF_Choice corría en un
  subproceso porque su ``config.py`` chocaba de nombre con el de
  ``analysis/``; como paquete ya no hay choque.
- Las rutas se reciben por parámetro (``input_csv``, ``output_dir``) en vez
  de constantes globales y ``os.chdir``.
- ``print`` → logger ``etf.screener``.
"""

from __future__ import annotations

import os
import time
from pathlib import Path

from fog_radar.core.logging import get_logger

from .config import INPUT_CSV
from .etf_discovery.discovery import AutoETFDiscovery
from .file_handlers.writer import CSVWriter
from .utils import incidencias
from .utils.helpers import get_timestamp, print_section_header

log = get_logger("etf.screener")


def procesar_etfs(output_dir: Path, input_csv: Path = INPUT_CSV) -> Path | None:
    """Procesa todos los ETFs del CSV y guarda los resultados.

    Devuelve la ruta del ``etf_perf_data_<timestamp>.csv`` generado, o
    ``None`` si Yahoo no validó ningún ETF.
    """
    os.makedirs(output_dir, exist_ok=True)
    incidencias.reset()
    discovery = AutoETFDiscovery()

    # Fase 1: Lectura de tickers
    print_section_header("FASE 1: LECTURA DE TICKERS DESDE CSV")
    start = time.time()
    all_tickers = discovery.get_etf_tickers_from_csv(str(input_csv))

    lectura_time = time.time() - start
    log.info(f"Tiempo de lectura: {lectura_time:.2f} segundos")

    # Fase 2: Validación y enriquecimiento
    print_section_header("FASE 2: VALIDACIÓN Y ENRIQUECIMIENTO")
    start = time.time()
    df = discovery.validate_all(all_tickers)
    validation_time = time.time() - start

    log.info(f"Tiempo de validación: {validation_time / 60:.1f} minutos")
    log.info(f"Tiempo total: {(lectura_time + validation_time) / 60:.1f} minutos")

    # Validar que se encontraron ETFs
    if df.empty:
        log.warning("No se validó ningún ETF")
        return None

    # Guardar resultados y recopilar mensajes
    timestamp = get_timestamp()
    saved_files = []

    # Guardar performance data (CSV y XLSX) y etf_choice
    result = CSVWriter.save_performance_data(df, timestamp, str(output_dir))
    saved_files.append(f"Performance Data (CSV): {result['csv']}")
    saved_files.append(f"Performance Data (XLSX): {result['xlsx']}")
    saved_files.append(f"ETF Choice (CSV): {result['choice_csv']}")
    saved_files.append(f"ETF Choice (XLSX): {result['choice_xlsx']}")
    saved_files.append(f"  --> {result['found_count']} ETFs con rendimiento creciente")

    valid_file = CSVWriter.save_valid_tickers(df, timestamp, str(output_dir), str(input_csv))
    if valid_file:
        saved_files.append(f"Tickers Válidos: {valid_file}")
    else:
        saved_files.append("Tickers Válidos: Todos los ETFs son válidos (no se creó archivo)")

    # Incidencias detectadas durante la corrida (rate limit / info incompleta
    # de Yahoo -- ver utils/incidencias.py). No se crea archivo si no hubo
    # ninguna.
    incidencias_path = incidencias.guardar(str(output_dir), timestamp)
    if incidencias_path:
        saved_files.append(
            f"Incidencias (rate limit / info incompleta) [{incidencias.cantidad()}]: {incidencias_path}"
        )
    else:
        saved_files.append("Incidencias: ninguna detectada en esta corrida")

    print_section_header("ARCHIVOS GUARDADOS")
    for file_info in saved_files:
        log.info(f"[OK] {file_info}")

    return Path(result["csv"])
