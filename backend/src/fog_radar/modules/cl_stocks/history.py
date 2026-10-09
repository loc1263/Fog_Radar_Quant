"""Historial de rankings del IPSA en ``data/cl_stocks/history.db`` (ver ``core.histdb``).

Índice derivado de los CSV ``ranking_acciones_<ts>.csv`` (+ ``indicadores_acciones_<ts>.csv``):
los CSV siguen siendo la fuente de verdad y lo que sirve ``/ranking``. Una corrida
se identifica por el timestamp del nombre del archivo (``YYYYMMDD_HHMMSS``).

    uv run python -m fog_radar.jobs cl_stocks --backfill   # reconstruye desde los CSV
"""

from __future__ import annotations

import sqlite3
from pathlib import Path
from typing import Any

import pandas as pd

from fog_radar.core import histdb
from fog_radar.core.errors import ApiError
from fog_radar.core.logging import get_logger
from fog_radar.modules.cl_stocks import storage
from fog_radar.modules.cl_stocks.columnas import COLUMNAS
from fog_radar.settings import Settings

log = get_logger("cl.history")

METRICAS_NUMERICAS: tuple[str, ...] = tuple(c.key for c in COLUMNAS if c.unidad != "texto")

# Columna del CSV de ranking -> columna de la tabla ``ranking``.
PILARES_SQL: dict[str, str] = {
    "Score Valor": "score_valor",
    "Score Calidad": "score_calidad",
    "Score Crecimiento": "score_crecimiento",
    "Score Salud Financiera": "score_salud_financiera",
    "Score Dividendo": "score_dividendo",
    "Score Momentum": "score_momentum",
}

SCHEMA_V1 = """
CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT UNIQUE NOT NULL,
    run_at TEXT NOT NULL,
    archivo TEXT NOT NULL,
    n_tickers INTEGER NOT NULL,
    stored_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE ranking (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER,
    nombre TEXT,
    sector TEXT,
    industria TEXT,
    score_compuesto REAL,
    score_valor REAL,
    score_calidad REAL,
    score_crecimiento REAL,
    score_salud_financiera REAL,
    score_dividendo REAL,
    score_momentum REAL,
    metricas_disponibles INTEGER,
    grupo_duplicado INTEGER,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_ranking_ticker ON ranking(ticker);

-- Formato largo: agregar una métrica al extractor no requiere migrar.
CREATE TABLE indicadores (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    metrica TEXT NOT NULL,
    valor REAL,
    PRIMARY KEY (run_pk, ticker, metrica)
);
CREATE INDEX idx_indicadores_ticker ON indicadores(ticker, metrica);
"""

MIGRACIONES = (SCHEMA_V1,)

_RANKING_COLS = (
    "run_pk",
    "ticker",
    "rank",
    "nombre",
    "sector",
    "industria",
    "score_compuesto",
    *PILARES_SQL.values(),
    "metricas_disponibles",
    "grupo_duplicado",
)


def db_path(settings: Settings) -> Path:
    return storage.data_dir(settings) / histdb.NOMBRE


def _v(value: Any) -> Any:
    if value is None:
        return None
    try:
        if pd.isna(value):
            return None
    except (TypeError, ValueError):
        pass
    return value.item() if hasattr(value, "item") else value


def _num(value: Any) -> float | None:
    v = _v(value)
    if v is None:
        return None
    try:
        return float(v)
    except (TypeError, ValueError):
        return None


def _int(value: Any) -> int | None:
    v = _num(value)
    return None if v is None else int(v)


def _ts(path: Path) -> str:
    """``ranking_acciones_20260929_103000.csv`` → ``20260929_103000``."""
    return path.stem.removeprefix("ranking_acciones_")


# --- Escritura -------------------------------------------------------------------


def _guardar(conn: sqlite3.Connection, settings: Settings, ranking_csv: Path) -> int:
    ranking = pd.read_csv(ranking_csv)
    ts = _ts(ranking_csv)
    run_at = storage.timestamp_de(ranking_csv, settings.tzinfo)
    histdb.reemplazar_corrida(conn, "runs", "ts", ts)
    run_pk = conn.execute(
        "INSERT INTO runs (ts, run_at, archivo, n_tickers) VALUES (?, ?, ?, ?)",
        (ts, run_at.isoformat() if run_at else ts, ranking_csv.name, len(ranking)),
    ).lastrowid
    assert run_pk is not None

    histdb.insertar(
        conn,
        "ranking",
        _RANKING_COLS,
        [
            (
                run_pk,
                str(r["Símbolo"]),
                _int(r.get("Ranking")),
                _v(r.get("Nombre corto")),
                _v(r.get("Sector")),
                _v(r.get("Industria")),
                _num(r.get("Score Compuesto")),
                *(_num(r.get(c)) for c in PILARES_SQL),
                _int(r.get("Métricas Disponibles")),
                _int(r.get("Grupo Financiero Duplicado")),
            )
            for r in ranking.to_dict(orient="records")
        ],
    )

    indicadores_csv = ranking_csv.with_name(f"indicadores_acciones_{ts}.csv")
    if indicadores_csv.is_file():
        ind = pd.read_csv(indicadores_csv)
        presentes = [c for c in COLUMNAS if c.unidad != "texto" and c.csv in ind.columns]
        valores = [
            (run_pk, str(r["Símbolo"]), c.key, _num(r.get(c.csv)))
            for r in ind.to_dict(orient="records")
            for c in presentes
            if _num(r.get(c.csv)) is not None
        ]
        histdb.insertar(conn, "indicadores", ("run_pk", "ticker", "metrica", "valor"), valores)
    return run_pk


def guardar_corrida(settings: Settings, ranking_csv: Path) -> None:
    """Guarda (o reemplaza) la corrida del CSV de ranking en ``history.db``."""
    with histdb.conectar_escritura(db_path(settings)) as conn:
        histdb.migrar(conn, MIGRACIONES)
        _guardar(conn, settings, ranking_csv)
    log.info("Corrida %s guardada en history.db", _ts(ranking_csv))


def backfill(settings: Settings) -> str:
    """Carga en ``history.db`` todos los ``ranking_acciones_*.csv`` (idempotente)."""
    archivos = sorted(storage.data_dir(settings).glob(storage.PATRON_RANKING))
    if not archivos:
        return f"backfill: no hay {storage.PATRON_RANKING} en {storage.data_dir(settings)}"
    ok, omitidos = 0, []
    with histdb.conectar_escritura(db_path(settings)) as conn:
        histdb.migrar(conn, MIGRACIONES)
        for path in archivos:
            try:
                conn.execute("SAVEPOINT corrida")
                _guardar(conn, settings, path)
                conn.execute("RELEASE corrida")
                ok += 1
            except Exception as exc:
                conn.execute("ROLLBACK TO corrida")
                conn.execute("RELEASE corrida")
                log.warning("backfill: se omite %s (%s: %s)", path.name, type(exc).__name__, exc)
                omitidos.append(path.name)
    resumen = f"backfill: {ok} corridas cargadas en history.db"
    if omitidos:
        resumen += f", {len(omitidos)} omitidas ({', '.join(omitidos)})"
    return resumen


# --- Lectura ---------------------------------------------------------------------


def corridas(settings: Settings) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        """
        SELECT r.ts, r.run_at, r.archivo, r.n_tickers,
               k.ticker AS top1_ticker, k.score_compuesto AS top1_score
        FROM runs r
        LEFT JOIN ranking k ON k.run_pk = r.id AND k.rank = 1
        ORDER BY r.ts ASC
        """,
    )


def historia_ticker(settings: Settings, ticker: str) -> list[dict[str, Any]]:
    filas = histdb.filas(
        db_path(settings),
        f"""
        SELECT r.ts, r.run_at, r.n_tickers, k.rank, k.nombre, k.score_compuesto,
               {", ".join(f"k.{c}" for c in PILARES_SQL.values())}, k.metricas_disponibles
        FROM ranking k
        JOIN runs r ON r.id = k.run_pk
        WHERE k.ticker = ?
        ORDER BY r.ts ASC
        """,
        (ticker,),
    )
    if not filas:
        raise ApiError(404, "not_found", f"'{ticker}' no aparece en el historial del IPSA.")
    return filas


def historia_indicador(settings: Settings, ticker: str, metrica: str) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        """
        SELECT r.ts, r.run_at, i.valor
        FROM indicadores i
        JOIN runs r ON r.id = i.run_pk
        WHERE i.ticker = ? AND i.metrica = ?
        ORDER BY r.ts ASC
        """,
        (ticker, metrica),
    )


def _ranking_de(settings: Settings, ts: str) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        """
        SELECT k.ticker, k.nombre, k.rank, k.score_compuesto AS score
        FROM ranking k JOIN runs r ON r.id = k.run_pk
        WHERE r.ts = ?
        """,
        (ts,),
    )


def diff(settings: Settings, desde: str | None, hasta: str | None) -> dict[str, Any]:
    """Cambios de posición entre dos corridas (por defecto, las dos últimas)."""
    tss = [c["ts"] for c in corridas(settings)]
    if desde is None and hasta is None:
        if len(tss) < 2:
            raise ApiError(404, "not_found", "Hacen falta al menos dos corridas en el historial.")
        desde, hasta = tss[-2], tss[-1]
    elif desde is None or hasta is None:
        raise ApiError(422, "validation_error", "Indica 'desde' y 'hasta', o ninguno de los dos.")
    for ts in (desde, hasta):
        if ts not in tss:
            raise ApiError(404, "not_found", f"No existe la corrida '{ts}' en el historial.")
    return {
        "desde": desde,
        "hasta": hasta,
        "top_n": None,
        "filas": histdb.diff_corridas(_ranking_de(settings, desde), _ranking_de(settings, hasta), None),
    }
