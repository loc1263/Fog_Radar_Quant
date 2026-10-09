"""Historial del screener EEUU en ``data/us_stocks/history.db`` (ver ``core.histdb``).

Índice derivado de las carpetas ``runs/<run_id>/`` (``result.json``,
``candidates.json``, ``prices.json``), que siguen siendo la fuente de verdad y
lo que sirve ``/rankings/latest*``. Solo se guardan las corridas terminadas
(``status: done``).

- ``candidates``: cada candidato evaluado en cada corrida, con su posición en el
  orden de momentum 12-1 (``rank``; el top 15 son los ``rank <= top_n``), el score
  de salud y sus pilares.
- ``prices``: cierres únicos por (ticker, fecha), deduplicados entre corridas
  (cada corrida trae ~290 días de los mismos tickers). Base del backtesting (B2).

    uv run python -m fog_radar.jobs us_stocks --backfill   # reconstruye desde runs/
"""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path
from typing import Any

from fog_radar.core import histdb
from fog_radar.core.errors import ApiError
from fog_radar.core.logging import get_logger
from fog_radar.modules.us_stocks import storage
from fog_radar.settings import Settings

log = get_logger("us.history")

PILARES = ("valuation", "profitability", "financial_health", "shareholder_return", "technical")

SCHEMA_V1 = """
CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT UNIQUE NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL,
    min_market_cap REAL,
    universe_total INTEGER,
    candidates_total INTEGER,
    evaluated_total INTEGER,
    top_n INTEGER NOT NULL,
    stored_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE candidates (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER NOT NULL,
    in_top INTEGER NOT NULL,
    name TEXT,
    sector TEXT,
    market_cap REAL,
    price REAL,
    momentum_12_1 REAL,
    return_1d REAL,
    return_5d REAL,
    return_3m REAL,
    return_6m REAL,
    return_1y REAL,
    return_ytd REAL,
    rsi_14 REAL,
    sma_50 REAL,
    sma_200 REAL,
    score REAL,
    score_label TEXT,
    valuation REAL,
    profitability REAL,
    financial_health REAL,
    shareholder_return REAL,
    technical REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_candidates_ticker ON candidates(ticker);

CREATE TABLE prices (
    ticker TEXT NOT NULL,
    date TEXT NOT NULL,
    close REAL NOT NULL,
    PRIMARY KEY (ticker, date)
) WITHOUT ROWID;
"""

MIGRACIONES = (SCHEMA_V1,)

_CANDIDATE_COLS = (
    "run_pk",
    "ticker",
    "rank",
    "in_top",
    "name",
    "sector",
    "market_cap",
    "price",
    "momentum_12_1",
    "return_1d",
    "return_5d",
    "return_3m",
    "return_6m",
    "return_1y",
    "return_ytd",
    "rsi_14",
    "sma_50",
    "sma_200",
    "score",
    "score_label",
    *PILARES,
)
_RETORNOS = ("return_1d", "return_5d", "return_3m", "return_6m", "return_1y", "return_ytd")


def db_path(settings: Settings) -> Path:
    return settings.module_data_dir("us_stocks") / histdb.NOMBRE


def _leer(run_dir: Path, nombre: str) -> Any:
    path = run_dir / nombre
    return json.loads(path.read_text(encoding="utf-8")) if path.is_file() else None


# --- Escritura -------------------------------------------------------------------


def _guardar(conn: sqlite3.Connection, run_dir: Path) -> bool:
    """Guarda la corrida; ``False`` si no está terminada (sin ``result.json`` o con error)."""
    result = _leer(run_dir, "result.json")
    if result is None or result.get("status") != "done":
        return False
    candidatos = (_leer(run_dir, "candidates.json") or result).get("rows", [])
    top_n = int(result.get("top_n") or 15)

    histdb.reemplazar_corrida(conn, "runs", "run_id", result["run_id"])
    run_pk = conn.execute(
        """INSERT INTO runs (run_id, started_at, finished_at, status, min_market_cap, universe_total,
                             candidates_total, evaluated_total, top_n)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)""",
        (
            result["run_id"],
            result.get("started_at"),
            result.get("finished_at"),
            result["status"],
            result.get("min_market_cap"),
            result.get("universe_total"),
            result.get("candidates_total"),
            len(candidatos),
            top_n,
        ),
    ).lastrowid

    filas = []
    for i, r in enumerate(candidatos, start=1):
        tecnico = r.get("technical") or {}
        score = r.get("score") or {}
        categorias = score.get("categories") or {}
        filas.append(
            (
                run_pk,
                r["ticker"],
                i,
                int(i <= top_n),
                r.get("name"),
                r.get("sector"),
                r.get("market_cap"),
                r.get("price"),
                r.get("momentum_12_1"),
                *(r.get(c) for c in _RETORNOS),
                tecnico.get("rsi_14"),
                tecnico.get("sma_50"),
                tecnico.get("sma_200"),
                score.get("overall"),
                score.get("label"),
                *(categorias.get(p) for p in PILARES),
            )
        )
    histdb.insertar(conn, "candidates", _CANDIDATE_COLS, filas)

    matriz = _leer(run_dir, "prices.json") or {}
    fechas: list[str] = matriz.get("dates", [])
    cierres = [
        (ticker, fecha, close)
        for ticker, serie in (matriz.get("prices") or {}).items()
        for fecha, close in zip(fechas, serie, strict=False)
        if close is not None
    ]
    # La corrida más nueva manda (Yahoo ajusta cierres viejos por dividendos/splits).
    conn.executemany(
        "INSERT INTO prices (ticker, date, close) VALUES (?, ?, ?) "
        "ON CONFLICT(ticker, date) DO UPDATE SET close = excluded.close",
        cierres,
    )
    return True


def guardar_corrida(settings: Settings, run_dir: Path) -> None:
    with histdb.conectar_escritura(db_path(settings)) as conn:
        histdb.migrar(conn, MIGRACIONES)
        if _guardar(conn, run_dir):
            log.info("Corrida %s guardada en history.db", run_dir.name)


def backfill(settings: Settings) -> str:
    """Carga en ``history.db`` todas las corridas terminadas de ``runs/`` (idempotente),
    de la más antigua a la más nueva (así los cierres quedan con el último ajuste)."""
    carpetas = sorted(storage.list_runs(settings), key=lambda p: p.name)
    if not carpetas:
        return f"backfill: no hay corridas en {storage.runs_dir(settings)}"
    ok, omitidas = 0, []
    with histdb.conectar_escritura(db_path(settings)) as conn:
        histdb.migrar(conn, MIGRACIONES)
        for run_dir in carpetas:
            try:
                conn.execute("SAVEPOINT corrida")
                guardada = _guardar(conn, run_dir)
                conn.execute("RELEASE corrida")
            except Exception as exc:
                conn.execute("ROLLBACK TO corrida")
                conn.execute("RELEASE corrida")
                log.warning("backfill: se omite %s (%s: %s)", run_dir.name, type(exc).__name__, exc)
                guardada = False
            if guardada:
                ok += 1
            else:
                omitidas.append(run_dir.name)
    resumen = f"backfill: {ok} corridas cargadas en history.db"
    if omitidas:
        resumen += f", {len(omitidas)} omitidas (sin terminar o con error: {', '.join(omitidas)})"
    return resumen


# --- Lectura ---------------------------------------------------------------------


def corridas(settings: Settings) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        """
        SELECT r.run_id, r.started_at, r.finished_at, r.universe_total, r.candidates_total,
               r.evaluated_total, r.top_n, c.ticker AS top1_ticker, c.score AS top1_score
        FROM runs r
        LEFT JOIN candidates c ON c.run_pk = r.id AND c.rank = 1
        ORDER BY r.run_id ASC
        """,
    )


def historia_ticker(settings: Settings, ticker: str) -> list[dict[str, Any]]:
    filas = histdb.filas(
        db_path(settings),
        f"""
        SELECT r.run_id, r.started_at, r.evaluated_total, r.top_n,
               c.rank, c.in_top, c.name, c.price, c.momentum_12_1, c.score, c.score_label,
               {", ".join(f"c.{p}" for p in PILARES)}
        FROM candidates c
        JOIN runs r ON r.id = c.run_pk
        WHERE c.ticker = ?
        ORDER BY r.run_id ASC
        """,
        (ticker,),
    )
    if not filas:
        raise ApiError(404, "not_found", f"'{ticker}' no aparece entre los candidatos del historial.")
    return [
        {
            **{k: v for k, v in f.items() if k not in PILARES},
            "in_top": bool(f["in_top"]),
            "pilares": {p: f[p] for p in PILARES},
        }
        for f in filas
    ]


def permanencia_top(settings: Settings, ultimas: int) -> list[dict[str, Any]]:
    """Cuántas de las últimas ``ultimas`` corridas estuvo cada ticker en el top, y
    su racha actual (corridas seguidas en el top hasta la última)."""
    filas = histdb.filas(
        db_path(settings),
        """
        WITH ult AS (SELECT id, run_id FROM runs ORDER BY run_id DESC LIMIT ?)
        SELECT u.run_id, c.ticker, c.name, c.rank, c.in_top, c.score
        FROM candidates c JOIN ult u ON u.id = c.run_pk
        ORDER BY u.run_id DESC
        """,
        (ultimas,),
    )
    if not filas:
        return []
    runs = sorted({f["run_id"] for f in filas}, reverse=True)
    por_ticker: dict[str, dict[str, Any]] = {}
    for f in filas:
        d = por_ticker.setdefault(
            f["ticker"],
            {"ticker": f["ticker"], "nombre": f["name"], "veces_en_top": 0, "en_top": set(), "ultimo": None},
        )
        if f["in_top"]:
            d["veces_en_top"] += 1
            d["en_top"].add(f["run_id"])
        if d["ultimo"] is None:
            d["ultimo"] = f
    salida = []
    for d in por_ticker.values():
        if not d["veces_en_top"]:
            continue
        racha = 0
        for run_id in runs:
            if run_id not in d["en_top"]:
                break
            racha += 1
        ultimo = d["ultimo"]
        salida.append(
            {
                "ticker": d["ticker"],
                "nombre": d["nombre"],
                "veces_en_top": d["veces_en_top"],
                "corridas": len(runs),
                "racha_actual": racha,
                "rank_ultima": ultimo["rank"] if ultimo["run_id"] == runs[0] else None,
                "score_ultima": ultimo["score"] if ultimo["run_id"] == runs[0] else None,
            }
        )
    salida.sort(key=lambda d: (-d["veces_en_top"], -d["racha_actual"], d["ticker"]))
    return salida


def _ranking_de(settings: Settings, run_id: str) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        """
        SELECT c.ticker, c.name AS nombre, c.rank, c.score
        FROM candidates c JOIN runs r ON r.id = c.run_pk
        WHERE r.run_id = ?
        """,
        (run_id,),
    )


def diff(settings: Settings, desde: str | None, hasta: str | None) -> dict[str, Any]:
    """Qué entró, salió o cambió en el top entre dos corridas (por defecto, las dos últimas)."""
    runs = {c["run_id"]: c for c in corridas(settings)}
    ids = sorted(runs)
    if desde is None and hasta is None:
        if len(ids) < 2:
            raise ApiError(404, "not_found", "Hacen falta al menos dos corridas en el historial.")
        desde, hasta = ids[-2], ids[-1]
    elif desde is None or hasta is None:
        raise ApiError(422, "validation_error", "Indica 'desde' y 'hasta', o ninguno de los dos.")
    for run_id in (desde, hasta):
        if run_id not in runs:
            raise ApiError(404, "not_found", f"No existe la corrida '{run_id}' en el historial.")
    top_n = int(runs[hasta]["top_n"])
    return {
        "desde": desde,
        "hasta": hasta,
        "top_n": top_n,
        "filas": histdb.diff_corridas(_ranking_de(settings, desde), _ranking_de(settings, hasta), top_n),
    }
