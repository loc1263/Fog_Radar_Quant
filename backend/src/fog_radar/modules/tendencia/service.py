"""Consultas de la API de Tendencia: última corrida (JSON archivado) e historial
(SQL de ``api/app.py`` del original, sobre ``history.db``)."""

from __future__ import annotations

import json
import sqlite3
from contextlib import AbstractContextManager
from typing import Any

from fog_radar.core import histdb
from fog_radar.core.errors import ApiError
from fog_radar.modules.tendencia import run_files as storage
from fog_radar.modules.tendencia.schemas import TdDashboard, TdParametros
from fog_radar.settings import Settings


def parametros() -> TdParametros:
    """Constantes del modelo. Se leen de ``config`` sin cargar el universo: son
    literales del módulo, así que basta con importarlo una vez."""
    from fog_radar.modules.tendencia import config as c

    return TdParametros(
        top_n=c.TOP_N_SELECTION,
        top_n_us=c.TOP_N_SELECTION_US,
        max_peso_por_activo=c.MAX_WEIGHT_PER_ASSET,
        l2_gamma=c.L2_GAMMA,
        semanas_minimas=c.MIN_WEEKS_REQUIRED,
        horizonte_semanas=c.FORECAST_HORIZON_WEEKS,
        ma_corta=c.SHORT_MA_WEEKS,
        ma_larga=c.LONG_MA_WEEKS,
        ma_corta_3m=c.SHORT_MA_WEEKS_3M,
        ma_larga_3m=c.LONG_MA_WEEKS_3M,
        top_n_momentum=c.TOP_N_MOMENTUM,
        tamanos_portafolio=list(c.PORTFOLIO_SIZE_OPTIONS),
    )


def dashboard(settings: Settings, run_dir: str | None = None) -> TdDashboard:
    d = storage.latest_run(settings) if run_dir is None else storage.run_dir(settings, run_dir)
    payload = storage.read_dashboard(d)
    return TdDashboard.model_validate(
        {
            **payload,
            "archivos": storage.archivos(d),
            "parametros": parametros(),
            # Solo aplica a la última corrida: el archivo se reescribe en cada armado.
            "excluidos": storage.excluidos(settings) if run_dir is None else [],
        }
    )


# --- history.db ----------------------------------------------------------------


def _conn(settings: Settings) -> AbstractContextManager[sqlite3.Connection | None]:
    """Conexión de solo lectura; ``None`` si la base todavía no existe."""
    return histdb.conectar_lectura(storage.history_db(settings))


def _rows(settings: Settings, sql: str, params: tuple[Any, ...] | list[Any]) -> list[dict[str, Any]]:
    return histdb.filas(storage.history_db(settings), sql, params)


def analyses(settings: Settings) -> list[dict[str, Any]]:
    return _rows(settings, "SELECT DISTINCT analysis_key, label FROM analyses ORDER BY analysis_key", ())


def runs(
    settings: Settings, analysis: str, date_from: str | None, date_to: str | None, limit: int
) -> list[dict[str, Any]]:
    sql = """
        SELECT r.run_dir, r.run_date, r.generated_at, r.universe_source,
               a.analysis_key, a.label, a.universe_size, a.n_selected,
               a.retorno_universo_anual, a.volatilidad_universo_anual, a.sharpe_universo,
               a.retorno_portafolio_anual, a.volatilidad_portafolio_anual, a.sharpe_portafolio,
               a.tasa_libre_riesgo_anual
        FROM runs r
        JOIN analyses a ON a.run_id = r.id
        WHERE a.analysis_key = ?
    """
    params: list[Any] = [analysis]
    if date_from:
        sql += " AND r.run_date >= ?"
        params.append(date_from)
    if date_to:
        sql += " AND r.run_date <= ?"
        params.append(date_to)
    sql += " ORDER BY r.run_date ASC, r.generated_at ASC LIMIT ?"
    params.append(limit)
    return _rows(settings, sql, params)


def run_detalle(settings: Settings, run_dir: str) -> dict[str, Any]:
    with _conn(settings) as conn:
        run = conn.execute("SELECT * FROM runs WHERE run_dir = ?", (run_dir,)).fetchone() if conn else None
        if conn is None or run is None:
            raise ApiError(404, "not_found", f"No existe la corrida '{run_dir}' en la base.")
        analyses_rows = conn.execute("SELECT * FROM analyses WHERE run_id = ?", (run["id"],)).fetchall()
        detalle = []
        for a in analyses_rows:
            holdings = conn.execute(
                "SELECT * FROM holdings WHERE analysis_id = ? ORDER BY peso_final DESC", (a["id"],)
            ).fetchall()
            sectors = conn.execute(
                "SELECT * FROM sectors WHERE analysis_id = ? ORDER BY peso_total DESC", (a["id"],)
            ).fetchall()
            momentum = conn.execute(
                "SELECT * FROM momentum WHERE analysis_id = ? ORDER BY momentum_rank ASC", (a["id"],)
            ).fetchall()
            detalle.append(
                {
                    **dict(a),
                    "holdings": [dict(h) for h in holdings],
                    "sectors": [dict(s) for s in sectors],
                    "momentum": [dict(m) for m in momentum],
                }
            )
    result = dict(run)
    result["market_regime_notes"] = json.loads(result.get("market_regime_notes") or "[]")
    result["analyses"] = detalle
    return result


def tickers(settings: Settings, analysis: str) -> list[dict[str, Any]]:
    return _rows(
        settings,
        """
        SELECT DISTINCT h.ticker, h.nombre
        FROM holdings h
        JOIN analyses a ON a.id = h.analysis_id
        WHERE a.analysis_key = ?
        ORDER BY h.ticker
        """,
        (analysis,),
    )


def ticker_history(settings: Settings, ticker: str, analysis: str) -> list[dict[str, Any]]:
    rows = _rows(
        settings,
        """
        SELECT r.run_date, r.run_dir, h.peso_final, h.precio_actual_usd, h.senal_tecnica,
               h.momentum_26s_pct, h.retorno_esperado_6m_pct, h.precio_proyectado_6m_usd,
               h.precio_min_90pct_6m_usd, h.precio_max_90pct_6m_usd
        FROM holdings h
        JOIN analyses a ON a.id = h.analysis_id
        JOIN runs r ON r.id = a.run_id
        WHERE a.analysis_key = ? AND h.ticker = ?
        ORDER BY r.run_date ASC
        """,
        (analysis, ticker),
    )
    if not rows:
        raise ApiError(
            404,
            "not_found",
            f"'{ticker}' nunca apareció seleccionado en el análisis '{analysis or 'principal'}'.",
        )
    return rows


def sector_history(settings: Settings, analysis: str) -> list[dict[str, Any]]:
    return _rows(
        settings,
        """
        SELECT r.run_date, r.run_dir, s.sector, s.peso_total, s.n_acciones, s.retorno_6m_prom
        FROM sectors s
        JOIN analyses a ON a.id = s.analysis_id
        JOIN runs r ON r.id = a.run_id
        WHERE a.analysis_key = ?
        ORDER BY r.run_date ASC
        """,
        (analysis,),
    )


def momentum_leaders(settings: Settings, analysis: str, top: int) -> list[dict[str, Any]]:
    return _rows(
        settings,
        """
        SELECT r.run_date, r.run_dir, m.ticker, m.momentum_rank, m.momentum_score_pct
        FROM momentum m
        JOIN analyses a ON a.id = m.analysis_id
        JOIN runs r ON r.id = a.run_id
        WHERE a.analysis_key = ? AND m.momentum_rank <= ?
        ORDER BY r.run_date ASC, m.momentum_rank ASC
        """,
        (analysis, top),
    )


def universe_history(settings: Settings, ticker: str, analysis: str) -> list[dict[str, Any]]:
    """Posición de un ticker en el universo completo (pesos de Markowitz) en cada
    corrida, esté o no entre los seleccionados (``universe_weights``, desde la v2)."""
    rows = _rows(
        settings,
        """
        SELECT r.run_date, r.run_dir, u.universe_rank, a.universe_size, u.peso_markowitz,
               u.retorno_esperado_anual,
               EXISTS (
                   SELECT 1 FROM holdings h WHERE h.analysis_id = a.id AND h.ticker = u.ticker
               ) AS seleccionado
        FROM universe_weights u
        JOIN analyses a ON a.id = u.analysis_id
        JOIN runs r ON r.id = a.run_id
        WHERE a.analysis_key = ? AND u.ticker = ?
        ORDER BY r.run_date ASC, r.run_dir ASC
        """,
        (analysis, ticker),
    )
    if not rows:
        raise ApiError(
            404,
            "not_found",
            f"'{ticker}' no tiene pesos del universo guardados en el análisis '{analysis or 'principal'}'.",
        )
    return rows
