"""Pipeline batch del screener EEUU (7 pasos de ``scanner.run_batch``).

    uv run python -m fog_radar.jobs us_stocks              # ~10-15 min (500 tickers)
    uv run python -m fog_radar.jobs us_stocks --backfill   # solo reconstruye history.db

Cada corrida deja ``result.json``, ``candidates.json``, ``prices.json`` y
``run.log`` en ``data/us_stocks/runs/<run_id>/``. Nunca se ejecuta dentro de
un request web.
"""

from __future__ import annotations

import json

from fog_radar.core import histdb
from fog_radar.core.logging import get_logger
from fog_radar.modules.us_stocks import history, scanner, storage
from fog_radar.settings import Settings

log = get_logger("us.pipeline")


def run(settings: Settings, *, backfill: bool = False) -> str:
    if backfill:
        return history.backfill(settings)
    run_dir = scanner.run_batch(output_root=storage.runs_dir(settings))
    result = json.loads((run_dir / "result.json").read_text(encoding="utf-8"))
    if result.get("status") == "error":
        raise RuntimeError(f"La corrida {run_dir.name} falló: {result.get('error')}")
    histdb.guardar_sin_fallar("us_stocks", lambda: history.guardar_corrida(settings, run_dir))
    top = [r["ticker"] for r in result.get("rows", [])[:3]]
    return (
        f"{run_dir.name} · {result.get('candidates_total', 0)} candidatos de "
        f"{result.get('universe_total', 0)} · top 3: {', '.join(top) or '—'}"
    )
