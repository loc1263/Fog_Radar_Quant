"""
Capa de almacenamiento histórico (SQLite) de las corridas del pipeline.

Cada corrida que arma reporting.dashboard_data.build_dashboard_payload()
(una por ejecución de step07_export_dashboard_data.py, o el paso 8/8
dentro de run_pipeline.py) se guarda acá ADEMÁS de en los JSON/CSV de
siempre — esto no los reemplaza (esos archivos siguen siendo la fuente
de verdad de cada corrida individual, y lo que lee el frontend para "la
última corrida"), es un índice consultable de TODAS las corridas para
poder analizar tendencias en el tiempo (ver storage.history_writer para
cómo se llena y api.app para cómo se consulta desde el frontend).

Un solo archivo SQLite en data/history.db (ver settings.DATA_DIR), sin
dependencias externas (sqlite3 es de la librería estándar). Pensado para
un solo escritor (el pipeline, corrida por corrida, nunca en paralelo) y
lectores locales (la API FastAPI en api.app) — no hace falta un motor
cliente/servidor para este caso de uso.

Por qué SQLite y no Postgres: es un pipeline de un solo proceso corriendo
en tu máquina local, sin escrituras concurrentes ni acceso remoto — un
archivo que se puede respaldar copiándolo, sin ningún servicio que
mantener corriendo, te da exactamente el mismo SQL (joins, agregaciones)
que necesitás para "analizar un año de corridas".
"""

import os
from contextlib import contextmanager

from fog_radar.core import histdb
from fog_radar.modules.tendencia import config


def history_db_path():
    """data/tendencia/history.db — se calcula al usarse (la raíz puede cambiar, ver paths.py)."""
    return os.path.join(config.DATA_DIR, "history.db")


# `momentum_rank` (no `rank`, palabra reservada en SQLite moderno por las
# funciones ventana) es la posición 1..N en el ranking de momentum de esa
# corrida/análisis — ver modeling.momentum.
SCHEMA_V1 = """
CREATE TABLE IF NOT EXISTS runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_dir TEXT UNIQUE NOT NULL,
    run_date TEXT NOT NULL,
    generated_at TEXT NOT NULL,
    universe_source TEXT,
    market_regime_notes TEXT,
    stored_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_runs_date ON runs(run_date);

CREATE TABLE IF NOT EXISTS analyses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    analysis_key TEXT NOT NULL,
    label TEXT NOT NULL,
    universe_size INTEGER,
    n_selected INTEGER,
    retorno_universo_anual REAL,
    volatilidad_universo_anual REAL,
    sharpe_universo REAL,
    retorno_portafolio_anual REAL,
    volatilidad_portafolio_anual REAL,
    sharpe_portafolio REAL,
    tasa_libre_riesgo_anual REAL,
    UNIQUE(run_id, analysis_key)
);
CREATE INDEX IF NOT EXISTS idx_analyses_run ON analyses(run_id);
CREATE INDEX IF NOT EXISTS idx_analyses_key ON analyses(analysis_key);

CREATE TABLE IF NOT EXISTS holdings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    nombre TEXT,
    pais TEXT,
    sector TEXT,
    peso_final REAL,
    precio_actual_usd REAL,
    senal_tecnica TEXT,
    ma_10s REAL,
    ma_40s REAL,
    momentum_26s_pct REAL,
    retorno_esperado_anual_pct REAL,
    volatilidad_anual_pct REAL,
    precio_proyectado_6m_usd REAL,
    precio_min_90pct_6m_usd REAL,
    precio_max_90pct_6m_usd REAL,
    retorno_esperado_6m_pct REAL
);
CREATE INDEX IF NOT EXISTS idx_holdings_analysis ON holdings(analysis_id);
CREATE INDEX IF NOT EXISTS idx_holdings_ticker ON holdings(ticker);

CREATE TABLE IF NOT EXISTS sectors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    sector TEXT,
    n_acciones INTEGER,
    peso_total REAL,
    retorno_6m_prom REAL,
    senal_predominante TEXT,
    tickers TEXT
);
CREATE INDEX IF NOT EXISTS idx_sectors_analysis ON sectors(analysis_id);

CREATE TABLE IF NOT EXISTS momentum (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    momentum_rank INTEGER,
    ticker TEXT,
    nombre TEXT,
    pais TEXT,
    sector TEXT,
    precio_actual_usd REAL,
    momentum_1m_pct REAL,
    momentum_3m_pct REAL,
    momentum_6m_pct REAL,
    momentum_12m_pct REAL,
    pct_meses_positivos_12m REAL,
    momentum_score_pct REAL
);
CREATE INDEX IF NOT EXISTS idx_momentum_analysis ON momentum(analysis_id);
CREATE INDEX IF NOT EXISTS idx_momentum_ticker ON momentum(ticker);
"""


# v2 (INFORME_SQLITE §3.1): un ticker una sola vez por análisis, y los pesos de
# Markowitz del universo completo (``markowitz_universe_weights<sufijo>.csv``)
# para ver cuán cerca del top estuvo cada ticker y por qué salió.
SCHEMA_V2 = """
CREATE UNIQUE INDEX IF NOT EXISTS ux_holdings_analysis_ticker ON holdings(analysis_id, ticker);

CREATE TABLE IF NOT EXISTS universe_weights (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    analysis_id INTEGER NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
    universe_rank INTEGER NOT NULL,
    ticker TEXT NOT NULL,
    nombre TEXT,
    pais TEXT,
    peso_markowitz REAL,
    retorno_esperado_anual REAL,
    UNIQUE(analysis_id, ticker)
);
CREATE INDEX IF NOT EXISTS idx_universe_weights_ticker ON universe_weights(ticker);
"""

# Migraciones en orden (ver core.histdb.migrar). La v1 es el esquema original con
# ``IF NOT EXISTS``: aplicarla a una base creada antes del versionado no cambia nada.
MIGRACIONES = (SCHEMA_V1, SCHEMA_V2)


@contextmanager
def get_connection(db_path=None):
    """Conexión de escritura (claves foráneas, WAL; ver ``core.histdb``). Filas
    accesibles por nombre de columna; commit al salir del ``with`` o rollback si
    hubo excepción."""
    with histdb.conectar_escritura(db_path or history_db_path()) as conn:
        yield conn


def init_db(db_path=None):
    """Crea o migra el esquema a la última versión. Idempotente."""
    with get_connection(db_path) as conn:
        histdb.migrar(conn, MIGRACIONES)
