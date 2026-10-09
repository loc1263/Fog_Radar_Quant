"""Historial de corridas de ETFs en ``data/etf/history.db`` (ver ``core.histdb``).

Índice derivado de los CSV de cada corrida: la vigente en ``current/`` y las
archivadas en ``hist/`` con sufijo ``_<yyyymmdd>[_n]``. Los CSV siguen siendo la
fuente de verdad (y lo que sirven ``/top15``, ``/rotacion``, etc.).

Una corrida se identifica por ``run_key`` = ``<yyyymmdd>`` o ``<yyyymmdd>_<n>``:
el mismo sufijo con que ``archivado.archivar_corrida`` la archivará (o ya la
archivó). Para la vigente, ``n`` = corridas archivadas de esa fecha + 1, así la
clave no cambia al archivarla; re-analizarla (``--solo-analisis``) la reemplaza.

- ``runs``: una fila por corrida (lo mismo que ``historial_corridas.csv``).
- ``ranking``: el universo depurado completo, con score, sub-scores, métricas y
  ``en_top15``.
- ``momentum``: el ranking de Momentum (``Score_comp``).
- ``excluidos`` y ``categorias``.

    uv run python -m fog_radar.jobs etf --backfill   # reconstruye desde current/ + hist/
"""

from __future__ import annotations

import re
import sqlite3
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any

import pandas as pd

from fog_radar.core import histdb
from fog_radar.core.errors import ApiError
from fog_radar.core.logging import get_logger
from fog_radar.modules.etf import storage
from fog_radar.modules.etf.analysis.archivado import extraer_fecha_y_secuencia
from fog_radar.modules.etf.analysis.config import TOP_N
from fog_radar.settings import Settings

log = get_logger("etf.history")

SCHEMA_V1 = """
CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_key TEXT UNIQUE NOT NULL,
    fecha TEXT NOT NULL,
    secuencia INTEGER NOT NULL,
    run_at TEXT,
    universo_bruto INTEGER,
    universo_depurado INTEGER NOT NULL,
    n_outliers INTEGER,
    sharpe_promedio REAL,
    beta_promedio REAL,
    top1_ticker TEXT,
    top1_score REAL,
    stored_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_runs_fecha ON runs(fecha, secuencia);

CREATE TABLE ranking (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER NOT NULL,
    en_top15 INTEGER NOT NULL,
    nombre TEXT,
    category TEXT,
    proveedor TEXT,
    precio REAL,
    aum REAL,
    expense_ratio REAL,
    perf_5d REAL,
    perf_1m REAL,
    perf_6m REAL,
    perf_ytd REAL,
    perf_1y REAL,
    sharpe REAL,
    treynor REAL,
    beta REAL,
    s_sharpe REAL,
    s_treynor REAL,
    s_1y REAL,
    s_6m REAL,
    s_consistencia REAL,
    s_tendencia REAL,
    s_baja_dispersion REAL,
    s_bajo_beta REAL,
    s_cerca_maximo REAL,
    score REAL,
    score_simple REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_ranking_ticker ON ranking(ticker);

CREATE TABLE momentum (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER NOT NULL,
    nombre TEXT,
    category TEXT,
    m_norm REAL,
    q_norm REAL,
    consist_norm REAL,
    score_comp REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_momentum_ticker ON momentum(ticker);

CREATE TABLE excluidos (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    nombre TEXT,
    category TEXT,
    ventana TEXT,
    z_score REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_excluidos_ticker ON excluidos(ticker);

CREATE TABLE categorias (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    n INTEGER,
    sharpe_promedio REAL,
    treynor_mediana REAL,
    beta_promedio REAL,
    perf_1y_promedio REAL,
    perf_6m_promedio REAL,
    score_promedio REAL,
    pct_positivos_1y REAL,
    tickers TEXT,
    PRIMARY KEY (run_pk, category)
);
"""

MIGRACIONES = (SCHEMA_V1,)

# Columna del CSV -> columna de la tabla.
RANKING_COLS: dict[str, str] = {
    "Nombre": "nombre",
    "Category": "category",
    "Proveedor": "proveedor",
    "Precio": "precio",
    "AUM": "aum",
    "Expense Ratio": "expense_ratio",
    "Performance 5D (%)": "perf_5d",
    "Performance 1M (%)": "perf_1m",
    "Performance 6M (%)": "perf_6m",
    "Performance YTD (%)": "perf_ytd",
    "Performance 1Y (%)": "perf_1y",
    "Sharpe Ratio": "sharpe",
    "Treynor Ratio": "treynor",
    "Beta": "beta",
    "s_sharpe": "s_sharpe",
    "s_treynor": "s_treynor",
    "s_1y": "s_1y",
    "s_6m": "s_6m",
    "s_consistencia": "s_consistencia",
    "s_tendencia": "s_tendencia",
    "s_baja_dispersion": "s_baja_dispersion",
    "s_bajo_beta": "s_bajo_beta",
    "s_cerca_maximo": "s_cerca_maximo",
    "Score": "score",
    "Score_Simple": "score_simple",
}
MOMENTUM_COLS: dict[str, str] = {
    "Nombre": "nombre",
    "Category": "category",
    "M_norm": "m_norm",
    "Q_norm": "q_norm",
    "Consist_norm": "consist_norm",
    "Score_comp": "score_comp",
}
EXCLUIDOS_COLS: dict[str, str] = {
    "Nombre": "nombre",
    "Category": "category",
    "Ventana_Atipica": "ventana",
    "Z_Score": "z_score",
}
CATEGORIAS_COLS: dict[str, str] = {
    "Category": "category",
    "N": "n",
    "Sharpe_Promedio": "sharpe_promedio",
    "Treynor_Mediana": "treynor_mediana",
    "Beta_Promedio": "beta_promedio",
    "Perf_1Y_Promedio": "perf_1y_promedio",
    "Perf_6M_Promedio": "perf_6m_promedio",
    "Score_Promedio": "score_promedio",
    "Pct_Positivos_1Y": "pct_positivos_1y",
    "Tickers": "tickers",
}

_SUFIJO = re.compile(r"_(\d{8}(?:_\d+)?)\.csv$")


def db_path(settings: Settings) -> Path:
    return storage.module_dir(settings) / histdb.NOMBRE


def _v(value: Any) -> Any:
    if value is None:
        return None
    try:
        if pd.isna(value):
            return None
    except (TypeError, ValueError):
        pass
    return value.item() if hasattr(value, "item") else value


# --- Corridas en disco -------------------------------------------------------------


@dataclass(frozen=True)
class CorridaEnDisco:
    """Rutas de los CSV de una corrida (``None`` si ese archivo no existe)."""

    run_key: str
    fecha: str
    secuencia: int
    ranking: Path
    perf: Path | None
    excluidos: Path | None
    momentum: Path | None
    categorias: Path | None


def _o_none(path: Path) -> Path | None:
    return path if path.is_file() else None


def corridas_archivadas(settings: Settings) -> list[CorridaEnDisco]:
    """Corridas de ``hist/`` que tienen ``etf_ranking_completo_<sufijo>.csv``, en orden."""
    hist = storage.hist_dir(settings)
    if not hist.is_dir():
        return []
    salida = []
    for ranking in hist.glob("etf_ranking_completo_*.csv"):
        m = _SUFIJO.search(ranking.name)
        if not m:
            continue
        sufijo = m.group(1)
        fecha, secuencia = extraer_fecha_y_secuencia(ranking.name)
        salida.append(
            CorridaEnDisco(
                run_key=sufijo,
                fecha=fecha,
                secuencia=secuencia,
                ranking=ranking,
                perf=_o_none(hist / f"etf_perf_data_{sufijo}.csv"),
                excluidos=_o_none(hist / f"etf_excluidos_outliers_{sufijo}.csv"),
                momentum=_o_none(hist / f"etf_momentum_ranking_completo_{sufijo}.csv"),
                categorias=_o_none(hist / f"etf_categorias_ranking_{sufijo}.csv"),
            )
        )
    return sorted(salida, key=lambda c: (c.fecha, c.secuencia))


def corrida_vigente(settings: Settings) -> CorridaEnDisco | None:
    """La corrida de ``current/``, con la clave que tendrá al archivarse."""
    cur = storage.current_dir(settings)
    ranking = cur / storage.RANKING
    perf = storage.csv_actual(cur)
    if not ranking.is_file() or perf is None:
        return None
    fecha = storage.fecha_de(perf.name)
    if fecha is None:
        return None
    tag = fecha.strftime("%Y%m%d")
    archivadas = sum(1 for c in corridas_archivadas(settings) if c.fecha == tag)
    secuencia = archivadas + 1
    return CorridaEnDisco(
        run_key=tag if secuencia == 1 else f"{tag}_{secuencia}",
        fecha=tag,
        secuencia=secuencia,
        ranking=ranking,
        perf=perf,
        excluidos=_o_none(cur / storage.EXCLUIDOS),
        momentum=_o_none(cur / "etf_momentum_ranking_completo.csv"),
        categorias=_o_none(cur / storage.CATEGORIAS),
    )


@dataclass
class _Leida:
    corrida: CorridaEnDisco
    run_at: datetime
    ranking: pd.DataFrame
    universo_bruto: int | None
    excluidos: pd.DataFrame | None
    momentum: pd.DataFrame | None
    categorias: pd.DataFrame | None


def _leer(settings: Settings, c: CorridaEnDisco) -> _Leida:
    def csv(path: Path | None) -> pd.DataFrame | None:
        return pd.read_csv(path) if path is not None else None

    bruto = None
    if c.perf is not None:
        perf = pd.read_csv(c.perf, encoding="utf-8-sig", usecols=["TipoInstrumento"])
        bruto = int((perf["TipoInstrumento"] == "ETF").sum())
    return _Leida(
        corrida=c,
        run_at=datetime.fromtimestamp(c.ranking.stat().st_mtime, tz=settings.tzinfo),
        ranking=pd.read_csv(c.ranking),
        universo_bruto=bruto,
        excluidos=csv(c.excluidos),
        momentum=csv(c.momentum),
        categorias=csv(c.categorias),
    )


# --- Escritura -------------------------------------------------------------------


def _filas(df: pd.DataFrame | None, mapa: dict[str, str]) -> list[tuple[Any, ...]]:
    if df is None or df.empty:
        return []
    return [tuple(_v(r.get(c)) for c in mapa) for r in df.to_dict(orient="records")]


def _guardar(conn: sqlite3.Connection, d: _Leida) -> None:
    c, ranking = d.corrida, d.ranking
    top1 = ranking.iloc[0] if len(ranking) else None
    histdb.reemplazar_corrida(conn, "runs", "run_key", c.run_key)
    run_pk = conn.execute(
        """INSERT INTO runs (run_key, fecha, secuencia, run_at, universo_bruto, universo_depurado, n_outliers,
                             sharpe_promedio, beta_promedio, top1_ticker, top1_score)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
        (
            c.run_key,
            f"{c.fecha[:4]}-{c.fecha[4:6]}-{c.fecha[6:]}",
            c.secuencia,
            d.run_at.isoformat(),
            d.universo_bruto,
            len(ranking),
            None if d.excluidos is None else len(d.excluidos),
            _v(ranking["Sharpe Ratio"].mean()) if "Sharpe Ratio" in ranking else None,
            _v(ranking["Beta"].mean()) if "Beta" in ranking else None,
            None if top1 is None else str(top1["Ticker"]),
            None if top1 is None else _v(top1.get("Score")),
        ),
    ).lastrowid

    rank_cols = ("run_pk", "ticker", "rank", "en_top15", *RANKING_COLS.values())
    histdb.insertar(
        conn,
        "ranking",
        rank_cols,
        [
            (run_pk, str(r["Ticker"]), i, int(i <= TOP_N), *(_v(r.get(col)) for col in RANKING_COLS))
            for i, r in enumerate(ranking.to_dict(orient="records"), start=1)
        ],
    )
    if d.momentum is not None:
        histdb.insertar(
            conn,
            "momentum",
            ("run_pk", "ticker", "rank", *MOMENTUM_COLS.values()),
            [
                (run_pk, str(r["Ticker"]), i, *(_v(r.get(col)) for col in MOMENTUM_COLS))
                for i, r in enumerate(d.momentum.to_dict(orient="records"), start=1)
            ],
        )
    if d.excluidos is not None and not d.excluidos.empty:
        histdb.insertar(
            conn,
            "excluidos",
            ("run_pk", "ticker", *EXCLUIDOS_COLS.values()),
            [
                (run_pk, str(r["Ticker"]), *(_v(r.get(col)) for col in EXCLUIDOS_COLS))
                for r in d.excluidos.to_dict(orient="records")
            ],
        )
    histdb.insertar(
        conn,
        "categorias",
        ("run_pk", *CATEGORIAS_COLS.values()),
        [(run_pk, *f) for f in _filas(d.categorias, CATEGORIAS_COLS)],
    )


def guardar_vigente(settings: Settings) -> str | None:
    """Guarda (o reemplaza) la corrida de ``current/``. Lee los CSV bajo el lock de
    datos y escribe la base afuera de él (los GET de ``current`` no esperan)."""
    with storage.bloqueo_datos(settings, timeout=60):
        c = corrida_vigente(settings)
        leida = _leer(settings, c) if c is not None else None
    if leida is None:
        return None
    with histdb.conectar_escritura(db_path(settings)) as conn:
        histdb.migrar(conn, MIGRACIONES)
        _guardar(conn, leida)
    log.info("Corrida %s guardada en history.db", leida.corrida.run_key)
    return leida.corrida.run_key


def backfill(settings: Settings) -> str:
    """Carga en ``history.db`` las corridas de ``hist/`` y la vigente (idempotente)."""
    with storage.bloqueo_datos(settings, timeout=60):
        corridas = corridas_archivadas(settings)
        vigente = corrida_vigente(settings)
        if vigente is not None:
            corridas.append(vigente)
        leidas: list[_Leida] = []
        omitidas: list[str] = []
        for c in corridas:
            try:
                leidas.append(_leer(settings, c))
            except Exception as exc:
                log.warning("backfill: se omite %s (%s: %s)", c.run_key, type(exc).__name__, exc)
                omitidas.append(c.run_key)
    if not corridas:
        return "backfill: no hay corridas de ETF en current/ ni en hist/"
    ok = 0
    with histdb.conectar_escritura(db_path(settings)) as conn:
        histdb.migrar(conn, MIGRACIONES)
        for d in leidas:
            try:
                conn.execute("SAVEPOINT corrida")
                _guardar(conn, d)
                conn.execute("RELEASE corrida")
                ok += 1
            except Exception as exc:
                conn.execute("ROLLBACK TO corrida")
                conn.execute("RELEASE corrida")
                log.warning("backfill: se omite %s (%s: %s)", d.corrida.run_key, type(exc).__name__, exc)
                omitidas.append(d.corrida.run_key)
    resumen = f"backfill: {ok} corridas cargadas en history.db"
    if omitidas:
        resumen += f", {len(omitidas)} omitidas ({', '.join(omitidas)})"
    return resumen


# --- Lectura ---------------------------------------------------------------------

_ORDEN = "r.fecha ASC, r.secuencia ASC"


def corridas(settings: Settings) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        f"""
        SELECT run_key, fecha, secuencia, run_at, universo_bruto, universo_depurado, n_outliers,
               sharpe_promedio, beta_promedio, top1_ticker, top1_score
        FROM runs r ORDER BY {_ORDEN}
        """,
    )


def historia_ticker(settings: Settings, ticker: str) -> list[dict[str, Any]]:
    """Cada corrida en que el ETF estuvo en el universo depurado, en el ranking de
    Momentum o excluido por outlier."""
    filas = histdb.filas(
        db_path(settings),
        f"""
        SELECT r.run_key, r.fecha, r.universo_depurado,
               k.rank, k.en_top15, k.score, k.score_simple, k.precio, k.sharpe, k.treynor, k.beta,
               k.perf_1m, k.perf_6m, k.perf_1y,
               m.rank AS momentum_rank, m.score_comp AS momentum_score,
               x.ventana AS excluido_ventana, x.z_score AS excluido_z,
               COALESCE(k.nombre, m.nombre, x.nombre) AS nombre,
               COALESCE(k.category, m.category, x.category) AS category
        FROM runs r
        LEFT JOIN ranking k ON k.run_pk = r.id AND k.ticker = :t
        LEFT JOIN momentum m ON m.run_pk = r.id AND m.ticker = :t
        LEFT JOIN excluidos x ON x.run_pk = r.id AND x.ticker = :t
        WHERE k.ticker IS NOT NULL OR m.ticker IS NOT NULL OR x.ticker IS NOT NULL
        ORDER BY {_ORDEN}
        """,
        {"t": ticker},
    )
    if not filas:
        raise ApiError(404, "not_found", f"'{ticker}' no aparece en el historial de ETFs.")
    return [{**f, "en_top15": bool(f["en_top15"]) if f["en_top15"] is not None else False} for f in filas]


def historia_categorias(settings: Settings) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        f"""
        SELECT r.run_key, r.fecha, c.category, c.n, c.score_promedio, c.sharpe_promedio,
               c.perf_1y_promedio,
               (SELECT COUNT(*) FROM ranking k
                 WHERE k.run_pk = r.id AND k.en_top15 = 1 AND k.category = c.category) AS en_top15
        FROM categorias c JOIN runs r ON r.id = c.run_pk
        ORDER BY {_ORDEN}, c.score_promedio DESC
        """,
    )


def permanencia_top(settings: Settings, ultimas: int) -> list[dict[str, Any]]:
    """Veces en el Top 15 y racha actual en las últimas ``ultimas`` corridas."""
    filas = histdb.filas(
        db_path(settings),
        """
        WITH ult AS (SELECT id, run_key, fecha, secuencia FROM runs
                     ORDER BY fecha DESC, secuencia DESC LIMIT ?)
        SELECT u.run_key, k.ticker, k.nombre, k.rank, k.score
        FROM ranking k JOIN ult u ON u.id = k.run_pk
        WHERE k.en_top15 = 1
        ORDER BY u.fecha DESC, u.secuencia DESC
        """,
        (ultimas,),
    )
    runs = [
        c["run_key"]
        for c in histdb.filas(
            db_path(settings),
            "SELECT run_key FROM runs ORDER BY fecha DESC, secuencia DESC LIMIT ?",
            (ultimas,),
        )
    ]
    if not runs:
        return []
    por_ticker: dict[str, dict[str, Any]] = {}
    for f in filas:
        d = por_ticker.setdefault(f["ticker"], {"nombre": f["nombre"], "en": {}, "ultimo": f})
        d["en"][f["run_key"]] = f
    salida = []
    for ticker, d in por_ticker.items():
        racha = 0
        for k in runs:
            if k not in d["en"]:
                break
            racha += 1
        ultima = d["en"].get(runs[0])
        salida.append(
            {
                "ticker": ticker,
                "nombre": d["nombre"],
                "veces_en_top": len(d["en"]),
                "corridas": len(runs),
                "racha_actual": racha,
                "rank_ultima": ultima["rank"] if ultima else None,
                "score_ultima": ultima["score"] if ultima else None,
            }
        )
    salida.sort(key=lambda d: (-d["veces_en_top"], -d["racha_actual"], d["ticker"]))
    return salida


def _ranking_de(settings: Settings, run_key: str) -> list[dict[str, Any]]:
    return histdb.filas(
        db_path(settings),
        """
        SELECT k.ticker, k.nombre, k.rank, k.score
        FROM ranking k JOIN runs r ON r.id = k.run_pk
        WHERE r.run_key = ?
        """,
        (run_key,),
    )


def diff(settings: Settings, desde: str | None, hasta: str | None) -> dict[str, Any]:
    """Rotación del Top 15 entre dos corridas cualesquiera (por defecto, las dos últimas)."""
    claves = [c["run_key"] for c in corridas(settings)]
    if desde is None and hasta is None:
        if len(claves) < 2:
            raise ApiError(404, "not_found", "Hacen falta al menos dos corridas en el historial.")
        desde, hasta = claves[-2], claves[-1]
    elif desde is None or hasta is None:
        raise ApiError(422, "validation_error", "Indica 'desde' y 'hasta', o ninguno de los dos.")
    for k in (desde, hasta):
        if k not in claves:
            raise ApiError(404, "not_found", f"No existe la corrida '{k}' en el historial.")
    return {
        "desde": desde,
        "hasta": hasta,
        "top_n": TOP_N,
        "filas": histdb.diff_corridas(_ranking_de(settings, desde), _ranking_de(settings, hasta), TOP_N),
    }
