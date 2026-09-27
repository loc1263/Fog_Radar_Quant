"""
Lógica del escaneo cuantitativo, pensada para correr como batch (fuera de
cualquier request HTTP).

El pipeline corre en pasos **secuenciales**, y cada paso hace UNA sola
cosa: o descarga datos (I/O de red a Yahoo Finance) o evalúa/calcula sobre
datos ya descargados (función pura, sin red). Nunca mezcla las dos cosas
en el mismo paso. Eso mantiene cada etapa fácil de leer, testear en
aislamiento y de extender sin tocar las demás (p. ej. cambiar la fuente de
datos de un paso, o agregar un paso nuevo, sin reescribir el resto):

  1. `_fetch_universe`        — descarga el universo de tickers de EEUU
                                 (S&P 500 ∪ Nasdaq-100).
  2. `_fetch_market_caps`     — descarga el market cap (`fast_info`, la
                                 consulta más liviana de yfinance) de cada
                                 ticker del universo.
  3. `_filter_by_market_cap`  — evalúa esos market caps contra el umbral
                                 y devuelve los tickers candidatos.
  4. `_download_price_history`— descarga en una sola llamada en lote el
                                 histórico de precios de los candidatos.
  5. `_compute_momentum_rows` — evalúa ese histórico: momentum 12-1,
                                 performance por plazo (1d/5d/3m/6m/1a/YTD)
                                 e indicadores técnicos (SMA/RSI/MACD) de
                                 cada candidato.
  6. `_fetch_fundamentals`    — descarga `get_info()` (nombre, sector y
                                 datos fundamentales) de cada candidato con
                                 momentum válido.
  7. `_evaluate_candidates`   — evalúa esos fundamentales junto con el
                                 técnico ya calculado y les asigna un score
                                 de salud 0-100 (ver `scoring.py`).

`run_batch()` simplemente encadena estos pasos en orden y persiste el
resultado. Cada corrida escribe todos sus archivos (resultado, candidatos
completos, metadata y log) en una carpeta propia bajo `data/runs/<run_id>/`,
para que el frontend (u otro consumidor) los lea sin necesidad de disparar
nada.
"""

from __future__ import annotations

import json
import logging
import random
import time
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import pandas as pd
import yfinance as yf

from .fundamentals import (
    build_financial_health,
    build_profitability,
    build_shareholder_return,
    build_technical,
    build_valuation,
)
from .scoring import evaluate_score
from .universe import get_us_large_cap_universe

logger = logging.getLogger("fog_radar.us.scanner")

DEFAULT_MIN_MARKET_CAP = 200_000_000_000  # 200B USD
DEFAULT_PAUSE_SECONDS = 0.6
TOP_N = 15
MAX_RETRIES = 2
HISTORY_PERIOD = "14mo"


def _sleep_with_jitter(base_seconds: float) -> None:
    if base_seconds <= 0:
        return
    time.sleep(random.uniform(base_seconds * 0.7, base_seconds * 1.3))


def _retry(fn, ticker: str, what: str):
    """Reintenta `fn()` hasta `MAX_RETRIES` veces con backoff creciente.
    Devuelve `None` (con un warning en el log) si todos los intentos fallan."""
    for attempt in range(MAX_RETRIES + 1):
        try:
            return fn()
        except Exception:
            if attempt < MAX_RETRIES:
                time.sleep(0.8 * (attempt + 1))
            else:
                logger.warning("No se pudo obtener %s de %s", what, ticker, exc_info=True)
    return None


# --------------------------------------------------------------------------
# Paso 1: universo de tickers (descarga)
# --------------------------------------------------------------------------


def _fetch_universe() -> list[str]:
    logger.info("Paso 1/7: obteniendo universo de acciones de EEUU...")
    universe = get_us_large_cap_universe()
    logger.info("Universo: %d tickers", len(universe))
    return universe


# --------------------------------------------------------------------------
# Paso 2: market cap de cada ticker del universo (descarga)
# --------------------------------------------------------------------------


def _fetch_one_market_cap(ticker: str) -> float | None:
    def call() -> float | None:
        fast_info = yf.Ticker(ticker).fast_info
        mc = getattr(fast_info, "market_cap", None)
        if mc is None:
            mc = fast_info["marketCap"]
        return float(mc) if mc else None

    return _retry(call, ticker, "market cap")


def _fetch_market_caps(tickers: list[str], pause_seconds: float) -> dict[str, float | None]:
    logger.info("Paso 2/7: descargando market cap de %d tickers...", len(tickers))
    total = len(tickers)
    market_caps: dict[str, float | None] = {}
    for i, ticker in enumerate(tickers, start=1):
        market_caps[ticker] = _fetch_one_market_cap(ticker)
        # Progreso cada 25 tickers: esta etapa es la más larga del batch
        # (una consulta por cada ticker del universo), así que sin esto el
        # log queda callado varios minutos seguidos.
        if i % 25 == 0 or i == total:
            logger.info("[%d/%d] market caps descargados", i, total)
        _sleep_with_jitter(pause_seconds)
    return market_caps


# --------------------------------------------------------------------------
# Paso 3: filtro por market cap (evaluación pura, sin red)
# --------------------------------------------------------------------------


def _filter_by_market_cap(market_caps: dict[str, float | None], min_market_cap: float) -> list[str]:
    logger.info("Paso 3/7: evaluando %d tickers contra el umbral de market cap...", len(market_caps))
    candidate_tickers = [
        ticker for ticker, mc in market_caps.items() if mc is not None and mc >= min_market_cap
    ]
    for ticker in candidate_tickers:
        logger.info("%s pasa el filtro (market cap %.0f)", ticker, market_caps[ticker])
    logger.info("%d/%d tickers pasan el filtro de market cap.", len(candidate_tickers), len(market_caps))
    return candidate_tickers


# --------------------------------------------------------------------------
# Paso 4: histórico de precios de los candidatos (descarga)
# --------------------------------------------------------------------------


def _download_price_history(candidate_tickers: list[str]) -> pd.DataFrame:
    logger.info("Paso 4/7: descargando histórico de precios de %d candidatos...", len(candidate_tickers))
    return yf.download(
        tickers=candidate_tickers,
        period=HISTORY_PERIOD,
        interval="1d",
        group_by="ticker",
        auto_adjust=True,
        progress=False,
        threads=True,
    )


# --------------------------------------------------------------------------
# Paso 5: momentum, performance y técnico (evaluación pura, sin red)
# --------------------------------------------------------------------------


def _price_at(closes: pd.Series, first_date: pd.Timestamp, target: pd.Timestamp) -> float | None:
    """Precio de cierre vigente en `target` (el último dato disponible en o
    antes de esa fecha). `None` si `target` cae antes del histórico
    disponible o el dato es inválido."""
    if target < first_date:
        return None
    try:
        value = float(closes.asof(target))
    except Exception:
        return None
    return value if pd.notna(value) and value > 0 else None


def _compute_momentum(closes: pd.Series) -> dict[str, float] | None:
    """Momentum 12-1 y performance en varios plazos (1d, 5d, 3m, 6m, 1a,
    YTD), a partir del histórico de precios ya descargado.

    Devuelve `None` si no hay al menos ~12 meses de histórico: sin eso el
    momentum 12-1 (que usa el filtro/ranking) no es fiable. Con esa base
    cubierta, cada plazo de performance se calcula de forma independiente
    y puede quedar en `None` sin invalidar el resto (p. ej. una acción con
    exactamente 12 meses de historia igual puede no tener un precio válido
    unos días antes del arranque de esa ventana).
    """
    closes = closes.dropna()
    if closes.empty:
        return None

    last_date = closes.index[-1]
    first_date = closes.index[0]

    if (last_date - first_date).days < 300:
        return None

    p_now = float(closes.iloc[-1])
    if pd.isna(p_now) or p_now <= 0:
        return None

    p_1mo = _price_at(closes, first_date, last_date - pd.Timedelta(30, unit="D"))
    p_12mo = _price_at(closes, first_date, last_date - pd.Timedelta(365, unit="D"))
    if p_1mo is None or p_12mo is None:
        return None

    def ret(p_then: float | None) -> float | None:
        return (p_now / p_then) - 1.0 if p_then else None

    ytd_target = pd.Timestamp(year=last_date.year, month=1, day=1, tz=last_date.tz)

    return {
        "price": p_now,
        "momentum_12_1": (p_1mo / p_12mo) - 1.0,
        "return_1d": ret(_price_at(closes, first_date, last_date - pd.Timedelta(1, unit="D"))),
        "return_5d": ret(_price_at(closes, first_date, last_date - pd.Timedelta(5, unit="D"))),
        "return_3m": ret(_price_at(closes, first_date, last_date - pd.Timedelta(91, unit="D"))),
        "return_6m": ret(_price_at(closes, first_date, last_date - pd.Timedelta(182, unit="D"))),
        "return_1y": ret(p_12mo),
        "return_ytd": ret(_price_at(closes, first_date, ytd_target)),
    }


def _extract_closes_volumes(
    history: pd.DataFrame, ticker: str, single_ticker: bool
) -> tuple[pd.Series, pd.Series] | None:
    try:
        if single_ticker:
            return history["Close"], history["Volume"]
        return history[ticker]["Close"], history[ticker]["Volume"]
    except Exception:
        return None


def _build_price_matrix(
    evaluated_tickers: list[str],
    history: pd.DataFrame,
    single_ticker: bool,
) -> dict[str, Any]:
    """Serializa el histórico de cierres ya descargado (`history`) de los
    candidatos evaluados a una matriz fechas x tickers, para que
    `app/portfolio.py` pueda calcular covarianza/correlación sin volver a
    pedirle nada a Yahoo Finance. Se persiste como `prices.json` en la
    carpeta de la corrida."""
    dates = history.index
    prices: dict[str, list[float | None]] = {}
    for ticker in evaluated_tickers:
        extracted = _extract_closes_volumes(history, ticker, single_ticker)
        if extracted is None:
            continue
        closes, _ = extracted
        closes = closes.reindex(dates)
        prices[ticker] = [None if pd.isna(v) else float(v) for v in closes]
    return {
        "dates": [d.strftime("%Y-%m-%d") for d in dates],
        "prices": prices,
    }


def _compute_momentum_rows(
    candidate_tickers: list[str],
    history: pd.DataFrame,
    market_caps: dict[str, float | None],
) -> list[dict[str, Any]]:
    logger.info(
        "Paso 5/7: calculando momentum, performance e indicadores técnicos de %d candidatos...",
        len(candidate_tickers),
    )
    single_ticker = len(candidate_tickers) == 1
    rows: list[dict[str, Any]] = []

    for ticker in candidate_tickers:
        extracted = _extract_closes_volumes(history, ticker, single_ticker)
        if extracted is None:
            continue
        closes, volumes = extracted

        metrics = _compute_momentum(closes)
        if metrics is None:
            continue

        rows.append(
            {
                "ticker": ticker,
                "market_cap": market_caps[ticker],
                **metrics,
                "technical": build_technical(closes, volumes),
            }
        )

    logger.info(
        "%d/%d candidatos con histórico suficiente para calcular momentum.",
        len(rows),
        len(candidate_tickers),
    )
    return rows


# --------------------------------------------------------------------------
# Paso 6: fundamentales de cada candidato (descarga)
# --------------------------------------------------------------------------


def _fetch_fundamentals(tickers: list[str], pause_seconds: float) -> dict[str, dict | None]:
    logger.info("Paso 6/7: descargando fundamentales de %d candidatos...", len(tickers))
    info_by_ticker: dict[str, dict | None] = {}
    total = len(tickers)
    for i, ticker in enumerate(tickers, start=1):
        info = _retry(lambda t=ticker: yf.Ticker(t).get_info(), ticker, "fundamentales")
        info_by_ticker[ticker] = info
        logger.info("[%d/%d] %s: fundamentales %s", i, total, ticker, "OK" if info else "no disponibles")
        _sleep_with_jitter(pause_seconds)
    return info_by_ticker


# --------------------------------------------------------------------------
# Paso 7: fundamentales + score de cada candidato (evaluación pura, sin red)
# --------------------------------------------------------------------------


def _evaluate_candidates(
    momentum_rows: list[dict[str, Any]],
    fundamentals_by_ticker: dict[str, dict | None],
) -> list[dict[str, Any]]:
    logger.info(
        "Paso 7/7: evaluando fundamentales y calculando score de %d candidatos...", len(momentum_rows)
    )

    for row in momentum_rows:
        ticker = row["ticker"]
        info = fundamentals_by_ticker.get(ticker)

        row["name"] = ticker
        row["sector"] = ""
        if info:
            row["name"] = info.get("longName") or info.get("shortName") or ticker
            row["sector"] = info.get("sector") or ""
            row["valuation"] = build_valuation(info)
            row["profitability"] = build_profitability(info)
            row["financial_health"] = build_financial_health(info)
            row["shareholder_return"] = build_shareholder_return(info)
        else:
            row["valuation"] = {}
            row["profitability"] = {}
            row["financial_health"] = {}
            row["shareholder_return"] = {}

        row["score"] = evaluate_score(row, market_cap=row["market_cap"])
        logger.info("%s evaluado (score %s)", ticker, row["score"]["overall"])

    return momentum_rows


# --------------------------------------------------------------------------
# Persistencia de la corrida
# --------------------------------------------------------------------------


def _json_default(value: Any) -> Any:
    if isinstance(value, (datetime, pd.Timestamp)):
        return value.isoformat()
    raise TypeError(f"Objeto no serializable: {value!r}")


def _write_json(path: Path, data: Any) -> None:
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False, default=_json_default), encoding="utf-8")


def run_batch(
    min_market_cap: float = DEFAULT_MIN_MARKET_CAP,
    pause_seconds: float = DEFAULT_PAUSE_SECONDS,
    *,
    output_root: Path,
) -> Path:
    """Ejecuta una corrida completa del scanner y deja todos sus archivos
    (metadata, candidatos completos, ranking final y log) en una carpeta
    propia bajo `output_root`. Devuelve el path de esa carpeta.

    Pensado para invocarse desde un proceso batch (CLI / tarea programada),
    nunca desde un request HTTP. El pipeline es una secuencia lineal de
    pasos (ver el docstring del módulo); si algún paso falla, se corta acá
    y se persiste lo que se haya evaluado hasta ese momento con
    `status: "error"`.
    """
    started_at = datetime.now(UTC)
    run_id = started_at.strftime("%Y-%m-%dT%H-%M-%SZ")
    run_dir = output_root / run_id
    run_dir.mkdir(parents=True, exist_ok=True)

    file_handler = logging.FileHandler(run_dir / "run.log", encoding="utf-8")
    file_handler.setFormatter(logging.Formatter("%(asctime)s %(levelname)s %(message)s"))
    logger.addHandler(file_handler)
    logger.setLevel(logging.INFO)

    status = "done"
    error: str | None = None
    universe: list[str] = []
    candidate_tickers: list[str] = []
    top_rows: list[dict[str, Any]] = []
    evaluated_rows: list[dict[str, Any]] = []
    price_matrix: dict[str, Any] | None = None

    try:
        # 1. Universo de tickers de EEUU.
        universe = _fetch_universe()

        # 2. Market cap de cada ticker del universo.
        market_caps = _fetch_market_caps(universe, pause_seconds)

        # 3. Filtro por umbral de market cap -> candidatos.
        candidate_tickers = _filter_by_market_cap(market_caps, min_market_cap)

        if candidate_tickers:
            # 4. Histórico de precios de los candidatos, en lote.
            history = _download_price_history(candidate_tickers)

            # 5. Momentum, performance e indicadores técnicos.
            momentum_rows = _compute_momentum_rows(candidate_tickers, history, market_caps)
            momentum_rows.sort(key=lambda r: r["momentum_12_1"], reverse=True)

            # 6. Fundamentales de los candidatos con momentum válido.
            fundamentals = _fetch_fundamentals([row["ticker"] for row in momentum_rows], pause_seconds)

            # 7. Fundamentales + score de salud.
            evaluated_rows = _evaluate_candidates(momentum_rows, fundamentals)

            top_rows = [dict(row) for row in evaluated_rows[:TOP_N]]

            # Matriz de precios de los candidatos evaluados, para el módulo
            # de optimización de portafolios (app/portfolio.py). Reusa el
            # histórico ya descargado en el paso 4, no dispara red nueva.
            price_matrix = _build_price_matrix(
                [row["ticker"] for row in evaluated_rows],
                history,
                single_ticker=len(candidate_tickers) == 1,
            )
        else:
            logger.info("Ningún ticker superó el umbral de market cap.")

        logger.info("Corrida completa. %d acciones en el ranking final.", len(top_rows))

    except Exception as exc:  # noqa: BLE001
        status = "error"
        error = str(exc)
        logger.exception("La corrida %s falló", run_id)

    finished_at = datetime.now(UTC)

    metadata = {
        "run_id": run_id,
        "status": status,
        "error": error,
        "started_at": started_at,
        "finished_at": finished_at,
        "min_market_cap": min_market_cap,
        "pause_seconds": pause_seconds,
        "universe_total": len(universe),
        "candidates_total": len(candidate_tickers),
        "top_n": TOP_N,
    }

    _write_json(run_dir / "result.json", {**metadata, "rows": top_rows})
    _write_json(run_dir / "candidates.json", {**metadata, "rows": evaluated_rows})
    if price_matrix is not None:
        _write_json(run_dir / "prices.json", price_matrix)

    logger.removeHandler(file_handler)
    file_handler.close()

    return run_dir
