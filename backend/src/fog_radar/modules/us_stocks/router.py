"""Endpoints del módulo Acciones EEUU: ``/api/us/*``.

- Lectura de la última corrida batch (ranking, candidatos, precios).
- Evaluación en vivo de un ticker (única llamada a Yahoo dentro de un
  request; con caché de 5 min y rate limit).
- Portafolio del modelo único (clusters + Black-Litterman): cómputo
  puro sobre ``prices.json`` + ``candidates.json`` (nunca descargan datos).
"""

from __future__ import annotations

from pathlib import Path
from threading import Lock
from typing import Annotated, Any

import pandas as pd
from cachetools import TTLCache
from fastapi import Depends, Query
from fastapi import Path as PathParam

from fog_radar.core import yahoo
from fog_radar.core.errors import ApiError
from fog_radar.core.logging import get_logger
from fog_radar.core.ratelimit import limite_concurrente, rate_limit
from fog_radar.core.schemas import HistDiff
from fog_radar.modules._base import make_module_router
from fog_radar.modules.us_stocks import history, portfolio, storage, ticker_eval
from fog_radar.modules.us_stocks.schemas import (
    TICKER_PATTERN,
    PortfolioLimits,
    PortfolioRequest,
    PortfolioResult,
    PriceMatrix,
    RunResponse,
    RunsList,
    TickerEvaluation,
    UsHistCorrida,
    UsHistPermanencia,
    UsHistTickerPunto,
)
from fog_radar.settings import Settings, get_settings

log = get_logger("us.api")

router = make_module_router("us_stocks", "Acciones EEUU")

SettingsDep = Annotated[Settings, Depends(get_settings)]

# Caché de la evaluación en vivo (US-BACK-01): mismo ticker en < 5 min no vuelve a Yahoo.
EVAL_TTL_SECONDS = 300
_eval_cache: TTLCache[str, dict[str, Any]] = TTLCache(maxsize=256, ttl=EVAL_TTL_SECONDS)
_eval_lock = Lock()


def clear_evaluation_cache() -> None:
    """Solo para tests."""
    with _eval_lock:
        _eval_cache.clear()


# --- Corridas -------------------------------------------------------------------


@router.get("/rankings/latest", response_model=RunResponse, summary="Top 15 por momentum (última corrida)")
def latest_ranking(settings: SettingsDep) -> Any:
    return storage.read_run_file(storage.latest_finished_run(settings), "result.json")


@router.get(
    "/rankings/latest/candidates",
    response_model=RunResponse,
    summary="Todos los candidatos evaluados (última corrida)",
)
def latest_candidates(settings: SettingsDep) -> Any:
    return storage.read_run_file(storage.latest_finished_run(settings), "candidates.json")


@router.get(
    "/rankings/latest/prices", response_model=PriceMatrix, summary="Matriz de cierres (última corrida)"
)
def latest_prices(
    settings: SettingsDep,
    days: Annotated[int | None, Query(ge=2, le=400, description="Solo los últimos N días.")] = None,
    tickers: Annotated[list[str] | None, Query(max_length=150)] = None,
) -> Any:
    matrix = storage.read_run_file(storage.latest_finished_run(settings), "prices.json")
    wanted = [t.strip().upper() for t in tickers] if tickers else None
    return storage.price_subset(matrix, days=days, tickers=wanted)


@router.get("/rankings/runs", response_model=RunsList, summary="Corridas disponibles (más reciente primero)")
def list_runs(settings: SettingsDep) -> RunsList:
    return RunsList(runs=[d.name for d in storage.list_runs(settings)])


# --- Historial (history.db) --------------------------------------------------------

RunId = Annotated[str | None, Query(pattern=r"^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z$")]


@router.get("/history/runs", response_model=list[UsHistCorrida], summary="Corridas guardadas en el historial")
def history_runs(settings: SettingsDep) -> Any:
    return history.corridas(settings)


@router.get(
    "/history/ticker/{ticker}",
    response_model=list[UsHistTickerPunto],
    summary="Score, momentum y posición de un ticker en cada corrida",
)
def history_ticker(ticker: Annotated[str, PathParam(pattern=TICKER_PATTERN)], settings: SettingsDep) -> Any:
    return history.historia_ticker(settings, ticker.strip().upper())


@router.get("/history/diff", response_model=HistDiff, summary="Qué entró y salió del top entre dos corridas")
def history_diff(settings: SettingsDep, desde: RunId = None, hasta: RunId = None) -> Any:
    return history.diff(settings, desde, hasta)


@router.get(
    "/history/top",
    response_model=list[UsHistPermanencia],
    summary="Permanencia en el top (últimas N corridas)",
)
def history_top(settings: SettingsDep, ultimas: Annotated[int, Query(ge=2, le=260)] = 20) -> Any:
    return history.permanencia_top(settings, ultimas)


# --- Evaluación en vivo ------------------------------------------------------------


@router.get(
    "/tickers/{ticker}/evaluate",
    response_model=TickerEvaluation,
    summary="Evalúa un ticker en vivo (Yahoo Finance)",
    dependencies=[
        Depends(rate_limit("20/minute", scope="us-evaluate")),
        Depends(limite_concurrente(4, scope="us-evaluate")),
    ],
)
def evaluate(ticker: Annotated[str, PathParam(pattern=TICKER_PATTERN)]) -> Any:
    key = ticker.strip().upper()
    with _eval_lock:
        cached = _eval_cache.get(key)
    if cached is not None:
        return {**cached, "cached": True}
    try:
        with yahoo.interactivo():
            result = ticker_eval.evaluate_ticker(key)
    except ticker_eval.TickerNotFoundError as exc:
        raise ApiError(404, "ticker_not_found", str(exc)) from exc
    except Exception as exc:
        log.warning("Error consultando Yahoo Finance para %s: %s", key, exc)
        raise ApiError(502, "upstream_error", "Error consultando Yahoo Finance. Intenta más tarde.") from exc
    with _eval_lock:
        _eval_cache[key] = result
    return result


# --- Portafolio --------------------------------------------------------------------


def _candidates_by_ticker(run_dir: Path) -> dict[str, dict[str, Any]]:
    rows = storage.read_run_file(run_dir, "candidates.json").get("rows", [])
    return {row["ticker"]: row for row in rows}


def _market_caps_and_scores(
    run_dir: Path, tickers: list[str]
) -> tuple[dict[str, float | None], dict[str, float | None]]:
    """Market cap y score de salud de cada ticker, tal como quedaron en ``candidates.json``."""
    candidates = _candidates_by_ticker(run_dir)
    caps = {t: candidates.get(t, {}).get("market_cap") for t in tickers}
    scores = {t: (candidates.get(t, {}).get("score") or {}).get("overall") for t in tickers}
    return caps, scores


def _returns(run_dir: Path, tickers: list[str]) -> pd.DataFrame:
    return portfolio.daily_returns(portfolio.load_price_matrix(run_dir, tickers))


def _rf(value: float | None) -> float:
    return portfolio.DEFAULT_RISK_FREE_RATE if value is None else value


def _portfolio_error(exc: portfolio.PortfolioDataError) -> ApiError:
    return ApiError(400, "portfolio_data", str(exc))


@router.get("/portfolio/limits", response_model=PortfolioLimits, summary="Límites del modelo de portafolio")
def portfolio_limits() -> PortfolioLimits:
    return PortfolioLimits(
        min_tickers=portfolio.MIN_TICKERS,
        min_assets=portfolio.MIN_N_ASSETS,
        max_assets=portfolio.MAX_N_ASSETS,
        default_assets=portfolio.DEFAULT_N_ASSETS,
        default_max_weight=portfolio.DEFAULT_MAX_WEIGHT,
        default_risk_free_rate=portfolio.DEFAULT_RISK_FREE_RATE,
    )


@router.post(
    "/portfolio/optimize",
    response_model=PortfolioResult,
    summary="Portafolio del modelo único: clusters de correlación + Black-Litterman",
    dependencies=[
        Depends(rate_limit("30/minute", scope="us-optimize")),
        Depends(limite_concurrente(4, scope="us-optimize")),
    ],
)
def optimize(request: PortfolioRequest, settings: SettingsDep) -> Any:
    run_dir = storage.latest_finished_run(settings)
    tickers = sorted(set(request.tickers))
    try:
        returns = _returns(run_dir, tickers)
        caps, scores = _market_caps_and_scores(run_dir, tickers)
        views = {k.strip().upper(): v for k, v in (request.views or {}).items()}
        result = portfolio.build_portfolio(
            returns,
            caps,
            scores,
            _rf(request.risk_free_rate),
            views or None,
            n_assets=request.n_assets,
            max_weight=request.max_weight,
        )
    except portfolio.PortfolioDataError as exc:
        raise _portfolio_error(exc) from exc
    return {"run_id": run_dir.name, "universe": tickers, **result}
