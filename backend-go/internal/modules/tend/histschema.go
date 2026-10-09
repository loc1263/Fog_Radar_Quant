package tend

// Esquema de history.db. Nunca se edita una migración
// publicada; un cambio es una migración nueva al final de Migraciones.

const schemaV1 = `
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
`

// v2: un ticker una sola vez por análisis y los pesos de Markowitz del universo completo.
const schemaV2 = `
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
`

// Migraciones de history.db, en orden.
var Migraciones = []string{schemaV1, schemaV2}
