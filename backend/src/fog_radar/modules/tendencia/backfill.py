"""Carga en ``history.db`` las corridas de ``output/`` que todavía no están
(port de ``Stock_Tendencia/backend/backfill_history.py``).

    uv run python -m fog_radar.jobs tendencia --backfill

Reconstruye el payload de cada carpeta ``output/<YYYYMMDD>_<NN>/`` con
``dashboard_data.build_dashboard_payload`` y lo guarda con
``history_writer.store_run`` (idempotente por ``run_dir``). No pisa el
``dashboard_data.json`` archivado en cada carpeta.
"""

import glob
import os

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.reporting.dashboard_data import build_dashboard_payload
from fog_radar.modules.tendencia.storage import history_writer
from fog_radar.modules.tendencia.storage.db import history_db_path


def _is_run_dir(path):
    return os.path.isdir(path) and config.is_run_dir_name(os.path.basename(path))


def main():
    run_dirs = sorted(d for d in glob.glob(os.path.join(config.OUTPUT_DIR, "*")) if _is_run_dir(d))
    if not run_dirs:
        return f"No hay carpetas de corrida en {config.OUTPUT_DIR}."

    ok, skipped = 0, []
    for run_dir in run_dirs:
        name = os.path.basename(run_dir)
        try:
            payload = build_dashboard_payload(run_dir)
        except Exception as e:
            print(f"[omitida] {name}: {e}")
            skipped.append(name)
            continue
        # Conserva la fecha de generación original de la corrida si está archivada.
        archived = os.path.join(run_dir, "dashboard_data.json")
        if os.path.exists(archived):
            import json

            with open(archived, encoding="utf-8") as f:
                payload["generated_at"] = json.load(f).get("generated_at", payload["generated_at"])
        history_writer.store_run(payload, run_path=run_dir)
        print(f"[cargada] {name} ({len(payload['analyses'])} análisis)")
        ok += 1

    resumen = f"backfill: {ok} corridas cargadas en {os.path.basename(history_db_path())}"
    if skipped:
        resumen += f", {len(skipped)} omitidas ({', '.join(skipped)})"
    return resumen
