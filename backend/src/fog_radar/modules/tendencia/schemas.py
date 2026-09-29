"""Modelos de la API del módulo Tendencia (``/api/tendencia``).

Los nombres de campo son los del ``dashboard_data.json`` y de ``history.db`` del
original (ya estaban en snake_case y en español). Prefijo ``Td`` porque el
OpenAPI es uno solo para el sitio. Las corridas antiguas no traen algunos
bloques (momentum semanal, portafolios reducidos, señal 3m): esos campos tienen
valor por defecto.
"""

from __future__ import annotations

from datetime import date

from pydantic import ConfigDict

from fog_radar.core.schemas import FechaLocal, Out


class TdMetricas(Out):
    retorno_esperado_anual: float
    volatilidad_anual: float
    sharpe_ratio: float


class TdPerformance(Out):
    universo_completo: TdMetricas
    portafolio_15_finales: TdMetricas
    tasa_libre_riesgo_anual: float
    tickers_seleccionados: list[str] = []


class TdPerformanceReducida(TdMetricas):
    n_acciones: int | None = None
    tickers_seleccionados: list[str] = []


class TdHolding(Out):
    ticker: str
    peso_final: float
    nombre: str | None = None
    pais: str | None = None
    sector: str | None = None
    precio_actual_usd: float | None = None
    senal_tecnica: str | None = None
    ma_10s: float | None = None
    ma_40s: float | None = None
    senal_tecnica_3m: str | None = None
    ma_4s: float | None = None
    ma_13s: float | None = None
    momentum_26s_pct: float | None = None
    retorno_esperado_anual_pct: float | None = None
    volatilidad_anual_pct: float | None = None
    precio_proyectado_6m_usd: float | None = None
    precio_min_90pct_6m_usd: float | None = None
    precio_max_90pct_6m_usd: float | None = None
    retorno_esperado_6m_pct: float | None = None


class TdSector(Out):
    sector: str
    n_acciones: int
    peso_total: float
    retorno_6m_prom: float | None = None
    senal_predominante: str | None = None
    tickers: str = ""
    """Tickers del sector separados por coma (formato del original)."""


class TdMomentumMensual(Out):
    rank: int
    ticker: str
    nombre: str | None = None
    pais: str | None = None
    sector: str | None = None
    precio_actual_usd: float | None = None
    momentum_1m_pct: float | None = None
    momentum_3m_pct: float | None = None
    momentum_6m_pct: float | None = None
    momentum_12m_pct: float | None = None
    pct_meses_positivos_12m: float | None = None
    momentum_score_pct: float | None = None


class TdMomentumSemanal(Out):
    rank: int
    ticker: str
    nombre: str | None = None
    pais: str | None = None
    sector: str | None = None
    precio_actual_usd: float | None = None
    momentum_1s_pct: float | None = None
    momentum_2s_pct: float | None = None
    momentum_3s_pct: float | None = None
    momentum_4s_pct: float | None = None
    pct_semanas_positivas_4s: float | None = None
    momentum_score_pct: float | None = None


class TdPortafolioReducido(Out):
    n: int
    performance: TdPerformanceReducida
    holdings: list[TdHolding]


class TdAnalisis(Out):
    key: str
    """"" = Asia + EEUU (principal), "_us" = solo EEUU."""
    label: str
    universe_size: int
    n_selected: int
    performance: TdPerformance
    holdings: list[TdHolding]
    sectors: list[TdSector]
    momentum: list[TdMomentumMensual] = []
    momentum_semanal: list[TdMomentumSemanal] = []
    portfolio_options: list[TdPortafolioReducido] = []


class TdArchivo(Out):
    nombre: str
    tipo: str
    bytes: int


class TdParametros(Out):
    """Parámetros del modelo (de ``config``), para los textos de metodología."""

    top_n: int
    top_n_us: int
    max_peso_por_activo: float
    l2_gamma: float
    semanas_minimas: int
    horizonte_semanas: int
    ma_corta: int
    ma_larga: int
    ma_corta_3m: int
    ma_larga_3m: int
    top_n_momentum: int
    tamanos_portafolio: list[int]


class TdExcluido(Out):
    """Ticker del universo que quedó fuera del análisis (falta de historia, de FX…)."""

    ticker: str
    nombre: str
    motivo: str


class TdDashboard(Out):
    """Una corrida completa (mismo contenido que ``dashboard_data.json``)."""

    generated_at: FechaLocal
    run_dir: str
    universe_source: str | None = None
    market_regime_notes: list[str] = []
    analyses: list[TdAnalisis]
    archivos: list[TdArchivo] = []
    """Archivos descargables de la corrida (PDF, CSV…), ver ``/runs/{run_dir}/files``."""
    parametros: TdParametros | None = None
    excluidos: list[TdExcluido] = []
    """Tickers excluidos en el último armado del dataset (TD-DATA-1 / TD-OPS-2)."""


# --- Historial (history.db) -----------------------------------------------------


class TdAnalisisDisponible(Out):
    analysis_key: str
    label: str


class TdRunPerformance(Out):
    run_dir: str
    run_date: date
    generated_at: FechaLocal
    universe_source: str | None = None
    analysis_key: str
    label: str
    universe_size: int | None = None
    n_selected: int | None = None
    retorno_universo_anual: float | None = None
    volatilidad_universo_anual: float | None = None
    sharpe_universo: float | None = None
    retorno_portafolio_anual: float | None = None
    volatilidad_portafolio_anual: float | None = None
    sharpe_portafolio: float | None = None
    tasa_libre_riesgo_anual: float | None = None


class TdTickerNombre(Out):
    ticker: str
    nombre: str | None = None


class TdTickerHistoria(Out):
    run_date: date
    run_dir: str
    peso_final: float | None = None
    precio_actual_usd: float | None = None
    senal_tecnica: str | None = None
    momentum_26s_pct: float | None = None
    retorno_esperado_6m_pct: float | None = None
    precio_proyectado_6m_usd: float | None = None
    precio_min_90pct_6m_usd: float | None = None
    precio_max_90pct_6m_usd: float | None = None


class TdSectorHistoria(Out):
    run_date: date
    run_dir: str
    sector: str | None = None
    peso_total: float | None = None
    n_acciones: int | None = None
    retorno_6m_prom: float | None = None


class TdMomentumLider(Out):
    run_date: date
    run_dir: str
    ticker: str | None = None
    momentum_rank: int | None = None
    momentum_score_pct: float | None = None


class TdRunDetalle(Out):
    """Corrida reconstruida desde ``history.db`` (endpoint ``/runs/{run_dir}``)."""

    model_config = ConfigDict(extra="allow", json_schema_serialization_defaults_required=True)

    id: int
    run_dir: str
    run_date: date
    generated_at: FechaLocal
    universe_source: str | None = None
    market_regime_notes: list[str] = []
    stored_at: str | None = None
    analyses: list[dict[str, object]]
