"""Modelos de la API del módulo EEUU.

Los nombres de campo son los del JSON que escribe el batch original
(``result.json`` / ``candidates.json``), que ya estaban en snake_case.
Todos los bloques de datos admiten valores ausentes: corridas viejas o
tickers sin fundamentales dejan objetos vacíos.
"""

from __future__ import annotations

import re
from datetime import datetime
from typing import Annotated, Literal

from pydantic import AfterValidator, BaseModel, Field

from fog_radar.core.schemas import Out

TICKER_PATTERN = r"^[A-Za-z0-9.\-^=]{1,15}$"
_TICKER_RE = re.compile(TICKER_PATTERN)


def _ticker(v: str) -> str:
    v = v.strip().upper()
    if not _TICKER_RE.match(v):
        raise ValueError(f"Ticker inválido: {v!r}")
    return v


Ticker = Annotated[str, AfterValidator(_ticker)]


# --- Bloques de datos por ticker --------------------------------------------


class Macd(Out):
    macd: float
    signal: float
    histogram: float


class Technical(Out):
    sma_50: float | None = None
    sma_200: float | None = None
    rsi_14: float | None = None
    macd: Macd | None = None
    volume: float | None = None
    average_volume: float | None = None


class Valuation(Out):
    per_trailing: float | None = None
    per_forward: float | None = None
    peg_ratio: float | None = None
    ev_to_ebitda: float | None = None
    price_to_book: float | None = None
    eps_trailing: float | None = None
    eps_forward: float | None = None


class Profitability(Out):
    roe: float | None = None
    roa: float | None = None
    net_margin: float | None = None


class FinancialHealth(Out):
    debt_to_equity: float | None = None
    current_ratio: float | None = None
    free_cash_flow: float | None = None


class ShareholderReturn(Out):
    dividend_yield: float | None = Field(default=None, description="Fracción (0.012 = 1,2 %).")


class ScoreCategories(Out):
    valuation: float | None = None
    profitability: float | None = None
    financial_health: float | None = None
    shareholder_return: float | None = None
    technical: float | None = None


class Score(Out):
    overall: float | None = None
    label: str = "Sin datos"
    categories: ScoreCategories = Field(default_factory=ScoreCategories)


class CandidateRow(Out):
    ticker: str
    name: str | None = None
    sector: str | None = None
    market_cap: float | None = None
    price: float
    momentum_12_1: float
    return_1d: float | None = None
    return_5d: float | None = None
    return_3m: float | None = None
    return_6m: float | None = None
    return_1y: float | None = None
    return_ytd: float | None = None
    technical: Technical = Field(default_factory=Technical)
    valuation: Valuation = Field(default_factory=Valuation)
    profitability: Profitability = Field(default_factory=Profitability)
    financial_health: FinancialHealth = Field(default_factory=FinancialHealth)
    shareholder_return: ShareholderReturn = Field(default_factory=ShareholderReturn)
    score: Score = Field(default_factory=Score)


# --- Corridas ------------------------------------------------------------------


class RunMetadata(Out):
    run_id: str
    status: Literal["done", "error"]
    error: str | None = None
    started_at: datetime
    finished_at: datetime
    min_market_cap: float
    pause_seconds: float
    universe_total: int
    candidates_total: int
    top_n: int


class RunResponse(RunMetadata):
    rows: list[CandidateRow]


class PriceMatrix(Out):
    dates: list[str]
    prices: dict[str, list[float | None]]


class RunsList(Out):
    runs: list[str]


# --- Historial (history.db) ------------------------------------------------------


class UsHistCorrida(Out):
    run_id: str
    started_at: datetime
    finished_at: datetime | None = None
    universe_total: int | None = None
    candidates_total: int | None = None
    evaluated_total: int | None = None
    top_n: int
    top1_ticker: str | None = None
    top1_score: float | None = None


class UsHistPilares(Out):
    valuation: float | None = None
    profitability: float | None = None
    financial_health: float | None = None
    shareholder_return: float | None = None
    technical: float | None = None


class UsHistTickerPunto(Out):
    run_id: str
    started_at: datetime
    evaluated_total: int | None = None
    top_n: int
    rank: int = Field(description="Posición por momentum 12-1 entre los candidatos evaluados.")
    in_top: bool
    name: str | None = None
    price: float | None = None
    momentum_12_1: float | None = None
    score: float | None = None
    score_label: str | None = None
    pilares: UsHistPilares


class UsHistPermanencia(Out):
    ticker: str
    nombre: str | None = None
    veces_en_top: int
    corridas: int
    racha_actual: int
    rank_ultima: int | None = None
    score_ultima: float | None = None


# --- Evaluación en vivo --------------------------------------------------------


class TickerEvaluation(Out):
    ticker: str
    name: str
    sector: str
    industry: str
    currency: str
    price: float | None
    market_cap: float | None
    fetched_at: datetime
    valuation: Valuation
    profitability: Profitability
    financial_health: FinancialHealth
    shareholder_return: ShareholderReturn
    technical: Technical
    score: Score
    cached: bool = Field(default=False, description="True si salió de la caché de 5 minutos.")


# --- Portafolio ------------------------------------------------------------------


class PortfolioRequest(BaseModel):
    tickers: list[Ticker] = Field(max_length=150, description="Universo ya filtrado (etapa 1).")
    n_assets: int = Field(default=10, ge=2, le=20, description="Activos de la cartera final (clusters).")
    max_weight: float = Field(default=0.25, ge=0.05, le=1.0, description="Peso máximo por activo.")
    risk_free_rate: float | None = Field(
        default=None, ge=-0.05, le=0.5, description="Tasa libre de riesgo anual; null = valor por defecto."
    )
    views: dict[str, float] | None = Field(
        default=None,
        description="Retorno anual esperado por ticker (fracción) que reemplaza la view automática.",
    )


class PortfolioMetrics(Out):
    expected_return: float
    volatility: float
    sharpe: float


class PortfolioPick(Out):
    ticker: str
    cluster: int = Field(description="Cluster de correlación (1..n) en orden de score.")
    score: float | None = None


class PortfolioResult(Out):
    run_id: str
    universe: list[str]
    selection: list[PortfolioPick]
    weights: dict[str, float]
    max_weight: float
    metrics: PortfolioMetrics
    prior_returns: dict[str, float]
    views: dict[str, float]
    posterior_returns: dict[str, float]


class PortfolioLimits(Out):
    """Límites y valores por defecto del modelo (para validar en el cliente)."""

    min_tickers: int
    min_assets: int
    max_assets: int
    default_assets: int
    default_max_weight: float
    default_risk_free_rate: float
