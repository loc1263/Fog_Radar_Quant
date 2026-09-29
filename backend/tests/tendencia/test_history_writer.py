"""
Tests de storage.history_writer.store_run() — sin red, sobre una base
sqlite temporal (tmp_path), nunca la real (data/history.db).
"""

import sqlite3

from fog_radar.modules.tendencia.storage import history_writer
from fog_radar.modules.tendencia.storage.db import get_connection, init_db


def _sample_payload(run_dir="20260921_01", sharpe=2.5):
    return {
        "generated_at": "2026-09-21T22:28:57.004745",
        "run_dir": run_dir,
        "universe_source": "dynamic_yfinance_screener",
        "market_regime_notes": ["VIX en 14.9: volatilidad baja."],
        "analyses": [
            {
                "key": "",
                "label": "Asia + EEUU",
                "universe_size": 77,
                "n_selected": 1,
                "performance": {
                    "universo_completo": {
                        "retorno_esperado_anual": 0.47,
                        "volatilidad_anual": 0.17,
                        "sharpe_ratio": 2.57,
                    },
                    "portafolio_15_finales": {
                        "retorno_esperado_anual": 0.48,
                        "volatilidad_anual": 0.17,
                        "sharpe_ratio": sharpe,
                    },
                    "tasa_libre_riesgo_anual": 0.0387,
                    "tickers_seleccionados": ["NVDA"],
                },
                "holdings": [
                    {
                        "ticker": "NVDA",
                        "peso_final": 0.06,
                        "nombre": "NVIDIA Corporation",
                        "pais": "EEUU",
                        "sector": "Technology",
                        "precio_actual_usd": 222.27,
                        "senal_tecnica": "Alcista",
                        "ma_10s": 218.05,
                        "ma_40s": 199.88,
                        "momentum_26s_pct": 32.99,
                        "retorno_esperado_anual_pct": 58.56,
                        "volatilidad_anual_pct": 48.0,
                        "precio_proyectado_6m_usd": 281.2,
                        "precio_min_90pct_6m_usd": 160.89,
                        "precio_max_90pct_6m_usd": 491.49,
                        "retorno_esperado_6m_pct": 26.51,
                    },
                ],
                "sectors": [
                    {
                        "sector": "Technology",
                        "n_acciones": 1,
                        "peso_total": 0.06,
                        "retorno_6m_prom": 26.51,
                        "senal_predominante": "Alcista",
                        "tickers": "NVDA",
                    },
                ],
                "momentum": [
                    {
                        "rank": 1,
                        "ticker": "NVDA",
                        "nombre": "NVIDIA Corporation",
                        "pais": "EEUU",
                        "sector": "Technology",
                        "precio_actual_usd": 222.27,
                        "momentum_1m_pct": 4.0,
                        "momentum_3m_pct": 10.0,
                        "momentum_6m_pct": 30.0,
                        "momentum_12m_pct": 60.0,
                        "pct_meses_positivos_12m": 75.0,
                        "momentum_score_pct": 40.0,
                    },
                ],
            },
        ],
    }


def test_store_run_inserts_run_analysis_holding_sector_and_momentum(tmp_path):
    db_path = str(tmp_path / "history.db")
    payload = _sample_payload()

    run_id = history_writer.store_run(payload, db_path=db_path)

    with get_connection(db_path) as conn:
        run = conn.execute("SELECT * FROM runs WHERE id = ?", (run_id,)).fetchone()
        assert run["run_dir"] == "20260921_01"
        assert run["run_date"] == "2026-09-21"
        assert run["universe_source"] == "dynamic_yfinance_screener"

        analysis = conn.execute("SELECT * FROM analyses WHERE run_id = ?", (run_id,)).fetchone()
        assert analysis["analysis_key"] == ""
        assert analysis["universe_size"] == 77
        assert analysis["sharpe_portafolio"] == 2.5

        holding = conn.execute("SELECT * FROM holdings WHERE analysis_id = ?", (analysis["id"],)).fetchone()
        assert holding["ticker"] == "NVDA"
        assert holding["peso_final"] == 0.06

        sector = conn.execute("SELECT * FROM sectors WHERE analysis_id = ?", (analysis["id"],)).fetchone()
        assert sector["sector"] == "Technology"

        momentum = conn.execute("SELECT * FROM momentum WHERE analysis_id = ?", (analysis["id"],)).fetchone()
        assert momentum["ticker"] == "NVDA"
        assert momentum["momentum_rank"] == 1


def test_store_run_is_idempotent_by_run_dir(tmp_path):
    db_path = str(tmp_path / "history.db")
    history_writer.store_run(_sample_payload(sharpe=2.5), db_path=db_path)

    # Reinsertar la MISMA corrida (mismo run_dir) con un valor distinto no
    # debe duplicar filas — debe reemplazar la anterior.
    history_writer.store_run(_sample_payload(sharpe=3.1), db_path=db_path)

    with get_connection(db_path) as conn:
        assert conn.execute("SELECT COUNT(*) FROM runs").fetchone()[0] == 1
        assert conn.execute("SELECT COUNT(*) FROM holdings").fetchone()[0] == 1
        sharpe = conn.execute("SELECT sharpe_portafolio FROM analyses").fetchone()[0]
        assert sharpe == 3.1


def test_store_run_two_different_runs_both_kept(tmp_path):
    db_path = str(tmp_path / "history.db")
    history_writer.store_run(_sample_payload(run_dir="20260914_01"), db_path=db_path)
    history_writer.store_run(_sample_payload(run_dir="20260921_01"), db_path=db_path)

    with get_connection(db_path) as conn:
        dates = [r["run_date"] for r in conn.execute("SELECT run_date FROM runs ORDER BY run_date")]
    assert dates == ["2026-09-14", "2026-09-21"]


def test_deleting_a_run_cascades_to_children(tmp_path):
    db_path = str(tmp_path / "history.db")
    history_writer.store_run(_sample_payload(), db_path=db_path)

    with get_connection(db_path) as conn:
        conn.execute("DELETE FROM runs")

    with get_connection(db_path) as conn:
        assert conn.execute("SELECT COUNT(*) FROM analyses").fetchone()[0] == 0
        assert conn.execute("SELECT COUNT(*) FROM holdings").fetchone()[0] == 0
        assert conn.execute("SELECT COUNT(*) FROM sectors").fetchone()[0] == 0
        assert conn.execute("SELECT COUNT(*) FROM momentum").fetchone()[0] == 0


def test_init_db_is_safe_to_call_twice(tmp_path):
    db_path = str(tmp_path / "history.db")
    init_db(db_path)
    init_db(db_path)  # no debe fallar (CREATE TABLE IF NOT EXISTS)
    with get_connection(db_path) as conn:
        assert isinstance(conn, sqlite3.Connection)
