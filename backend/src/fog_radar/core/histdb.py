"""Bases históricas SQLite de los módulos (``data/<modulo>/history.db``).

Patrón común (ver ``INFORME_SQLITE.md`` y ADR 0003): la base es un **índice
derivado** de los archivos de cada corrida, no su reemplazo.

- Los archivos de la corrida siguen siendo la fuente de verdad (y lo que sirve
  la "última corrida"): la paridad con los originales no depende de la base.
- El pipeline escribe **al final**, en una sola transacción e idempotente
  (si la corrida ya existe, se borra y se reinserta). Si la escritura falla se
  registra un WARNING y la corrida **no** falla (``guardar_sin_fallar``).
- La API abre la base en solo lectura (``conectar_lectura``) y devuelve ``[]``
  si todavía no existe.
- Un *backfill* por módulo reconstruye la base desde los archivos.

Las bases usan WAL (los GET no se bloquean mientras el pipeline escribe) y
versión de esquema en ``PRAGMA user_version`` (``migrar``).
"""

from __future__ import annotations

import sqlite3
from collections.abc import Callable, Iterator, Mapping, Sequence
from contextlib import contextmanager
from pathlib import Path
from typing import Any

from fog_radar.core.errors import ApiError
from fog_radar.core.logging import get_logger

log = get_logger("core.histdb")

NOMBRE = "history.db"
BUSY_TIMEOUT_MS = 30_000


@contextmanager
def conectar_escritura(path: Path | str) -> Iterator[sqlite3.Connection]:
    """Conexión de escritura: claves foráneas activas (para los ``ON DELETE
    CASCADE``), WAL y ``busy_timeout``. Commit al salir del ``with``; rollback
    si hubo excepción."""
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(path, timeout=BUSY_TIMEOUT_MS / 1000)
    conn.row_factory = sqlite3.Row
    try:
        conn.execute("PRAGMA foreign_keys = ON")
        conn.execute(f"PRAGMA busy_timeout = {BUSY_TIMEOUT_MS}")
        conn.execute("PRAGMA journal_mode = WAL")
        yield conn
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()


@contextmanager
def conectar_lectura(path: Path | str) -> Iterator[sqlite3.Connection | None]:
    """Conexión de solo lectura (``mode=ro``); ``None`` si la base no existe."""
    path = Path(path)
    if not path.is_file():
        yield None
        return
    conn = sqlite3.connect(f"{path.resolve().as_uri()}?mode=ro", uri=True, timeout=5)
    conn.row_factory = sqlite3.Row
    try:
        yield conn
    finally:
        conn.close()


def filas(path: Path | str, sql: str, params: Sequence[Any] | Mapping[str, Any] = ()) -> list[dict[str, Any]]:
    """Ejecuta una consulta de lectura. ``[]`` si la base no existe; 503 si está
    corrupta o el esquema no calza (p. ej. una base vieja sin migrar)."""
    with conectar_lectura(path) as conn:
        if conn is None:
            return []
        try:
            return [
                dict(r)
                for r in conn.execute(
                    sql, params if isinstance(params, Mapping) else tuple(params)
                ).fetchall()
            ]
        except sqlite3.DatabaseError as exc:
            raise ApiError(503, "invalid_data", f"{Path(path).name} no se pudo leer: {exc}") from exc


def version(conn: sqlite3.Connection) -> int:
    return int(conn.execute("PRAGMA user_version").fetchone()[0])


def migrar(conn: sqlite3.Connection, migraciones: Sequence[str]) -> int:
    """Aplica las migraciones pendientes según ``PRAGMA user_version``.

    ``migraciones[i]`` lleva la base de la versión ``i`` a la ``i + 1``; cada una
    corre en su propia transacción junto con el cambio de versión. Nunca se
    editan migraciones ya publicadas: un cambio de esquema es una migración
    nueva al final de la lista. Devuelve la versión final."""
    actual = version(conn)
    if actual > len(migraciones):
        raise RuntimeError(
            f"La base está en la versión {actual} y este código conoce hasta la {len(migraciones)}."
        )
    conn.commit()
    for i in range(actual, len(migraciones)):
        try:
            conn.executescript(f"BEGIN;\n{migraciones[i]}\nPRAGMA user_version = {i + 1};\nCOMMIT;")
        except Exception:
            conn.rollback()
            raise
        log.info("history.db migrada a la versión %d", i + 1)
    return len(migraciones)


def reemplazar_corrida(conn: sqlite3.Connection, tabla: str, columna: str, clave: str) -> None:
    """Borra la corrida ``clave`` (y sus filas hijas, vía ``ON DELETE CASCADE``)
    antes de reinsertarla: guardar dos veces la misma corrida no duplica nada."""
    conn.execute(f"DELETE FROM {tabla} WHERE {columna} = ?", (clave,))


def insertar(
    conn: sqlite3.Connection, tabla: str, columnas: Sequence[str], valores: list[tuple[Any, ...]]
) -> None:
    if not valores:
        return
    sql = f"INSERT INTO {tabla} ({', '.join(columnas)}) VALUES ({', '.join('?' * len(columnas))})"
    conn.executemany(sql, valores)


def guardar_sin_fallar[T](modulo: str, fn: Callable[[], T]) -> T | None:
    """Corre la escritura en la base histórica sin hacer fallar la corrida: los
    archivos ya quedaron instalados y la base se puede reconstruir con el backfill."""
    try:
        return fn()
    except Exception as exc:
        log.warning(
            "[%s] la corrida no se guardó en history.db (%s: %s). Se recupera con "
            "python -m fog_radar.jobs %s --backfill",
            modulo,
            type(exc).__name__,
            exc,
            modulo,
        )
        return None


def diff_corridas(
    desde: list[dict[str, Any]], hasta: list[dict[str, Any]], top_n: int | None
) -> list[dict[str, Any]]:
    """Compara dos rankings (filas con ``ticker``, ``nombre``, ``rank``, ``score``).

    Con ``top_n`` el estado se mide contra el top (``nuevo``/``salio``/``sigue``/
    ``fuera``); sin él, contra la presencia en el ranking. Orden: los del top de
    ``hasta`` primero (por rank), luego los que salieron, luego el resto."""
    a = {f["ticker"]: f for f in desde}
    b = {f["ticker"]: f for f in hasta}

    def en(f: dict[str, Any] | None) -> bool:
        if f is None or f.get("rank") is None:
            return f is not None and top_n is None
        return top_n is None or f["rank"] <= top_n

    salida: list[dict[str, Any]] = []
    for t in a.keys() | b.keys():
        fa, fb = a.get(t), b.get(t)
        ena, enb = en(fa), en(fb)
        estado = "sigue" if ena and enb else "nuevo" if enb else "salio" if ena else "fuera"
        ra, rb = (fa or {}).get("rank"), (fb or {}).get("rank")
        sa, sb = (fa or {}).get("score"), (fb or {}).get("score")
        salida.append(
            {
                "ticker": t,
                "nombre": (fb or fa or {}).get("nombre"),
                "rank_desde": ra,
                "rank_hasta": rb,
                "score_desde": sa,
                "score_hasta": sb,
                "delta_rank": ra - rb if ra is not None and rb is not None else None,
                "delta_score": round(sb - sa, 6) if sa is not None and sb is not None else None,
                "estado": estado,
            }
        )
    orden = {"sigue": 0, "nuevo": 0, "salio": 1, "fuera": 2}
    salida.sort(
        key=lambda f: (
            orden[f["estado"]],
            f["rank_hasta"] if f["estado"] != "salio" and f["rank_hasta"] is not None else 10**9,
            f["rank_desde"] if f["rank_desde"] is not None else 10**9,
            f["ticker"],
        )
    )
    return salida
