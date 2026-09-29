"""
Guarda el payload que arma reporting.dashboard_data.build_dashboard_payload()
en la base histórica (ver storage.db para el esquema).

store_run() es idempotente por run_dir: si esa corrida ya estaba en la
base (por ejemplo, volviste a correr step07_export_dashboard_data.py
sobre la misma corrida), borra sus filas anteriores antes de reinsertar
—vía ON DELETE CASCADE desde runs—, así nunca queda data duplicada ni
vieja mezclada con nueva.
"""

import json
import re

from fog_radar.modules.tendencia.storage.db import get_connection, init_db

_RUN_DATE_RE = re.compile(r"^(\d{4})(\d{2})(\d{2})_(\d{2})$")

# Columnas de cada tabla hija de `analyses` (ver storage.db.SCHEMA), en el
# mismo nombre que la clave del payload — salvo `momentum_rank`, que en el
# payload se llama `rank` (palabra reservada en SQLite). Agregar una
# columna nueva es agregarla acá y en el esquema.
HOLDING_COLUMNS = (
    "ticker",
    "nombre",
    "pais",
    "sector",
    "peso_final",
    "precio_actual_usd",
    "senal_tecnica",
    "ma_10s",
    "ma_40s",
    "momentum_26s_pct",
    "retorno_esperado_anual_pct",
    "volatilidad_anual_pct",
    "precio_proyectado_6m_usd",
    "precio_min_90pct_6m_usd",
    "precio_max_90pct_6m_usd",
    "retorno_esperado_6m_pct",
)
SECTOR_COLUMNS = (
    "sector",
    "n_acciones",
    "peso_total",
    "retorno_6m_prom",
    "senal_predominante",
    "tickers",
)
MOMENTUM_COLUMNS = (
    "momentum_rank",
    "ticker",
    "nombre",
    "pais",
    "sector",
    "precio_actual_usd",
    "momentum_1m_pct",
    "momentum_3m_pct",
    "momentum_6m_pct",
    "momentum_12m_pct",
    "pct_meses_positivos_12m",
    "momentum_score_pct",
)
_PAYLOAD_KEY = {"momentum_rank": "rank"}


def _run_date_from_dir(run_dir):
    """'20260921_01' -> '2026-09-21' (para poder ordenar/filtrar por fecha
    con comparación de strings ISO). None si el nombre no matchea el
    patrón esperado."""
    m = _RUN_DATE_RE.match(run_dir)
    if not m:
        return None
    return f"{m.group(1)}-{m.group(2)}-{m.group(3)}"


def _insert_rows(conn, table, columns, analysis_id, rows):
    """INSERT de las filas de una tabla hija de `analyses`. Las claves
    que falten en una fila se guardan como NULL."""
    cols = ("analysis_id", *columns)
    sql = f"INSERT INTO {table} ({', '.join(cols)}) VALUES ({', '.join('?' * len(cols))})"
    conn.executemany(
        sql, [(analysis_id, *(row.get(_PAYLOAD_KEY.get(c, c)) for c in columns)) for row in rows]
    )


def store_run(payload, db_path=None):
    """
    payload: el dict que devuelve dashboard_data.build_dashboard_payload()
    (mismo formato que dashboard_data.json: generated_at, run_dir,
    universe_source, market_regime_notes, analyses[]). Devuelve el id de
    runs.id de la corrida guardada.
    """
    init_db(db_path)
    run_dir = payload["run_dir"]
    run_date = _run_date_from_dir(run_dir) or run_dir[:8]

    with get_connection(db_path) as conn:
        existing = conn.execute("SELECT id FROM runs WHERE run_dir = ?", (run_dir,)).fetchone()
        if existing:
            conn.execute("DELETE FROM runs WHERE id = ?", (existing["id"],))

        cur = conn.execute(
            """INSERT INTO runs (run_dir, run_date, generated_at, universe_source, market_regime_notes)
               VALUES (?, ?, ?, ?, ?)""",
            (
                run_dir,
                run_date,
                payload.get("generated_at"),
                payload.get("universe_source"),
                json.dumps(payload.get("market_regime_notes") or [], ensure_ascii=False),
            ),
        )
        run_id = cur.lastrowid

        for analysis in payload.get("analyses", []):
            perf = analysis.get("performance", {})
            universo = perf.get("universo_completo", {})
            portafolio = perf.get("portafolio_15_finales", {})

            cur = conn.execute(
                """INSERT INTO analyses (
                       run_id, analysis_key, label, universe_size, n_selected,
                       retorno_universo_anual, volatilidad_universo_anual, sharpe_universo,
                       retorno_portafolio_anual, volatilidad_portafolio_anual, sharpe_portafolio,
                       tasa_libre_riesgo_anual
                   ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
                (
                    run_id,
                    analysis.get("key", ""),
                    analysis.get("label"),
                    analysis.get("universe_size"),
                    analysis.get("n_selected"),
                    universo.get("retorno_esperado_anual"),
                    universo.get("volatilidad_anual"),
                    universo.get("sharpe_ratio"),
                    portafolio.get("retorno_esperado_anual"),
                    portafolio.get("volatilidad_anual"),
                    portafolio.get("sharpe_ratio"),
                    perf.get("tasa_libre_riesgo_anual"),
                ),
            )
            analysis_id = cur.lastrowid

            _insert_rows(conn, "holdings", HOLDING_COLUMNS, analysis_id, analysis.get("holdings", []))
            _insert_rows(conn, "sectors", SECTOR_COLUMNS, analysis_id, analysis.get("sectors", []))
            _insert_rows(conn, "momentum", MOMENTUM_COLUMNS, analysis_id, analysis.get("momentum", []))

    return run_id
