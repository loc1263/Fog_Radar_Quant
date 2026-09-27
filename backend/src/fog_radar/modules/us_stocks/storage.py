"""Corridas del screener EEUU en disco: ``data/us_stocks/runs/<run_id>/``.

Mismo formato que ``Stock_Choice_EEUU/backend/data/runs`` (``result.json``,
``candidates.json``, ``prices.json``, ``run.log``): las corridas del
proyecto original se copian tal cual.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

from fog_radar.core.errors import NoDataError
from fog_radar.core.runs import list_run_dirs, read_json
from fog_radar.settings import Settings

NO_RUNS = "Todavía no hay corridas del screener EEUU. Ejecuta: python -m fog_radar.jobs us_stocks"


def runs_dir(settings: Settings) -> Path:
    return settings.module_data_dir("us_stocks") / "runs"


def list_runs(settings: Settings) -> list[Path]:
    """Carpetas de corridas, la más reciente primero (el nombre es un timestamp UTC)."""
    return list_run_dirs(runs_dir(settings))


def latest_finished_run(settings: Settings) -> Path:
    """La corrida más reciente que ya escribió ``result.json``.

    Una corrida en curso crea su carpeta (y ``run.log``) antes de terminar:
    se ignora y se sirve la última terminada.
    """
    for run_dir in list_runs(settings):
        if (run_dir / "result.json").is_file():
            return run_dir
    raise NoDataError(NO_RUNS)


def read_run_file(run_dir: Path, name: str) -> Any:
    path = run_dir / name
    if not path.is_file():
        raise NoDataError(f"La corrida {run_dir.name} no tiene {name}.")
    return read_json(path)


def price_subset(
    matrix: dict[str, Any], *, days: int | None = None, tickers: list[str] | None = None
) -> dict[str, Any]:
    """Recorta la matriz de cierres a los últimos ``days`` días y/o a ``tickers``."""
    dates: list[str] = matrix.get("dates", [])
    prices: dict[str, list[float | None]] = matrix.get("prices", {})
    if tickers is not None:
        wanted = set(tickers)
        prices = {t: v for t, v in prices.items() if t in wanted}
    if days is not None and days < len(dates):
        dates = dates[-days:]
        prices = {t: v[-days:] for t, v in prices.items()}
    return {"dates": dates, "prices": prices}
