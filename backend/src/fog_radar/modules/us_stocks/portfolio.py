"""
Modelo único de portafolio del módulo EEUU (docs/MODELO_NEW.md), sobre los
candidatos de la corrida batch más reciente:

1. **Ranking** multi-factor: el score de salud (`scoring.py`) ya calculado por el batch.
2. **Diversificación**: clustering jerárquico sobre la distancia de correlación;
   se elige 1 representante (el de mayor score) por cada uno de los `n_assets`
   clusters.
3. **Ponderación**: Black-Litterman (prior de equilibrio por market cap + views
   derivadas del score) y portafolio de máximo Sharpe long-only con tope de
   peso por activo.

Igual que `scoring.py`, son funciones puras sobre datos ya persistidos por
el batch (`prices.json`, `candidates.json`): nunca consultan Yahoo Finance.
Si faltan datos (un ticker sin histórico común suficiente) se levanta
`PortfolioDataError`; bajarlos es responsabilidad exclusiva del batch.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from scipy.cluster.hierarchy import fcluster, linkage
from scipy.optimize import minimize
from scipy.spatial.distance import squareform

TRADING_DAYS = 252
MIN_TICKERS = 2
MIN_COMMON_DAYS = 30

DEFAULT_RISK_FREE_RATE = 0.04
DEFAULT_RISK_AVERSION = 2.5  # delta: coeficiente de aversión al riesgo del equilibrio de mercado
DEFAULT_TAU = 0.05  # incertidumbre del prior de equilibrio en Black-Litterman
DEFAULT_BL_VIEW_SCALE = 0.5  # k: cuánto tilt aplica el score de salud sobre el prior
COV_RIDGE = 1e-6  # regularización para evitar covarianzas casi singulares

DEFAULT_N_ASSETS = 10  # tamaño de la cartera final (un representante por cluster)
MIN_N_ASSETS = 2
MAX_N_ASSETS = 20
DEFAULT_MAX_WEIGHT = 0.25  # tope de peso por activo
MIN_MAX_WEIGHT = 0.05
CLUSTER_LINKAGE = "average"


class PortfolioDataError(Exception):
    """Datos insuficientes para armar el portafolio pedido (nunca dispara una descarga)."""


# --------------------------------------------------------------------------
# Carga de datos ya persistidos por el batch
# --------------------------------------------------------------------------


def load_price_matrix(run_dir: Path, tickers: list[str]) -> pd.DataFrame:
    """Lee `prices.json` de la corrida y devuelve los cierres de `tickers`
    alineados por fecha (solo los días en que TODOS tienen dato)."""
    if len(tickers) < MIN_TICKERS:
        raise PortfolioDataError(f"Se necesitan al menos {MIN_TICKERS} tickers para optimizar un portafolio.")

    prices_path = run_dir / "prices.json"
    if not prices_path.exists():
        raise PortfolioDataError(
            "La corrida más reciente todavía no tiene datos de precios (prices.json). "
            "Corré el batch de nuevo (python -m app.batch) para generarlos."
        )

    raw = json.loads(prices_path.read_text(encoding="utf-8"))
    all_prices: dict[str, list[float | None]] = raw.get("prices", {})

    missing = [t for t in tickers if t not in all_prices]
    if missing:
        raise PortfolioDataError(
            f"Sin histórico de precios para: {', '.join(missing)}. "
            "Esperá a la próxima corrida del batch o elegí otros tickers."
        )

    dates = pd.to_datetime(raw["dates"])
    df = pd.DataFrame({t: all_prices[t] for t in tickers}, index=dates)
    df = df.dropna(how="any")

    if len(df) < MIN_COMMON_DAYS:
        raise PortfolioDataError(
            "Histórico común insuficiente entre los tickers seleccionados "
            f"({len(df)} días, se necesitan al menos {MIN_COMMON_DAYS})."
        )
    return df


def daily_returns(prices: pd.DataFrame) -> pd.DataFrame:
    returns = prices.pct_change().dropna(how="any")
    if returns.empty or len(returns.columns) < MIN_TICKERS:
        raise PortfolioDataError("No hay suficientes retornos diarios para calcular la covarianza.")
    return returns


def _annualized_mean(returns: pd.DataFrame) -> np.ndarray:
    return returns.mean().to_numpy() * TRADING_DAYS


def _annualized_cov(returns: pd.DataFrame) -> np.ndarray:
    cov = returns.cov().to_numpy() * TRADING_DAYS
    return cov + np.eye(len(cov)) * COV_RIDGE


# --------------------------------------------------------------------------
# Etapa 2: diversificación por clustering jerárquico de la correlación
# --------------------------------------------------------------------------


def select_by_cluster(
    returns: pd.DataFrame, scores: dict[str, float | None], n_assets: int = DEFAULT_N_ASSETS
) -> list[dict[str, Any]]:
    """Elige `n_assets` tickers, uno por cluster de correlación (el de mayor score).

    Distancia d = sqrt(0.5·(1-ρ)), linkage `average`, corte en `n_assets` clusters.
    Si el corte da menos clusters (p. ej. retornos casi idénticos) se completa con
    los tickers de mayor score aún no elegidos. Devuelve, ordenado por score
    descendente, `{ticker, cluster, score}` (cluster 1..m en ese mismo orden).
    """
    if not MIN_N_ASSETS <= n_assets <= MAX_N_ASSETS:
        raise PortfolioDataError(f"El portafolio debe tener entre {MIN_N_ASSETS} y {MAX_N_ASSETS} activos.")

    tickers = list(returns.columns)
    n = len(tickers)
    score_of = {t: scores.get(t) if scores.get(t) is not None else 0.0 for t in tickers}

    def rank_key(t: str) -> tuple[float, str]:
        return (-score_of[t], t)

    if n <= n_assets:
        labels = np.arange(1, n + 1)
    else:
        corr = returns.corr().clip(-1.0, 1.0).fillna(0.0).to_numpy()
        dist = np.sqrt(0.5 * (1.0 - corr))
        np.fill_diagonal(dist, 0.0)
        link = linkage(squareform(dist, checks=False), method=CLUSTER_LINKAGE)
        labels = fcluster(link, t=n_assets, criterion="maxclust")

    members: dict[int, list[str]] = {}
    for t, lab in zip(tickers, labels, strict=True):
        members.setdefault(int(lab), []).append(t)
    picks = {min(group, key=rank_key) for group in members.values()}

    wanted = min(n_assets, n)
    if len(picks) < wanted:
        rest = sorted((t for t in tickers if t not in picks), key=rank_key)
        picks.update(rest[: wanted - len(picks)])

    ordered = sorted(picks, key=rank_key)
    return [{"ticker": t, "cluster": i + 1, "score": scores.get(t)} for i, t in enumerate(ordered)]


# --------------------------------------------------------------------------
# Etapa 3: Black-Litterman (prior de equilibrio de mercado + views automáticas
# derivadas del score de salud, blendeadas con la fórmula estándar de BL) y
# portafolio tangente long-only con tope de peso por activo.
# --------------------------------------------------------------------------


def _max_sharpe_weights(
    mean_returns: np.ndarray, cov: np.ndarray, risk_free_rate: float, max_weight: float = 1.0
) -> np.ndarray:
    n = len(mean_returns)
    cap = max(max_weight, 1.0 / n)  # con tope < 1/n no existe solución que sume 1

    def neg_sharpe(w: np.ndarray) -> float:
        ret = w @ mean_returns
        vol = float(np.sqrt(w @ cov @ w))
        if vol <= 1e-12:
            return 1e6
        return -(ret - risk_free_rate) / vol

    constraints = [{"type": "eq", "fun": lambda w: np.sum(w) - 1.0}]
    bounds = [(0.0, cap)] * n
    x0 = np.repeat(1.0 / n, n)

    result = minimize(
        neg_sharpe,
        x0,
        method="SLSQP",
        bounds=bounds,
        constraints=constraints,
        options={"maxiter": 500, "ftol": 1e-10},
    )
    if not result.success:
        raise PortfolioDataError(f"No se pudo optimizar el portafolio: {result.message}")

    w = np.clip(result.x, 0.0, None)
    total = w.sum()
    if total <= 0:
        raise PortfolioDataError(
            "La optimización devolvió un portafolio degenerado (todos los pesos en cero)."
        )
    return w / total


def black_litterman_weights(
    returns: pd.DataFrame,
    market_caps: dict[str, float | None],
    scores: dict[str, float | None],
    risk_free_rate: float = DEFAULT_RISK_FREE_RATE,
    views: dict[str, float] | None = None,
    risk_aversion: float = DEFAULT_RISK_AVERSION,
    tau: float = DEFAULT_TAU,
    max_weight: float = 1.0,
) -> dict[str, Any]:
    tickers = list(returns.columns)
    n = len(tickers)
    cov = _annualized_cov(returns)

    caps = np.array([max(market_caps.get(t) or 0.0, 0.0) for t in tickers])
    w_mkt = caps / caps.sum() if caps.sum() > 0 else np.repeat(1.0 / n, n)

    # Prior de equilibrio (reverse optimization): δ·Σ·w_mkt es el retorno en EXCESO
    # de la tasa libre de riesgo; se le suma rf para trabajar en retornos totales
    # (como ``pypfopt.black_litterman.market_implied_prior_returns``). Fog Radar
    # (Fase 7, corrección): el original no sumaba rf y luego _max_sharpe_weights
    # la restaba otra vez, así que con views neutras no recuperaba w_mkt.
    pi = risk_aversion * (cov @ w_mkt) + risk_free_rate

    # Views automáticas: score de salud alto -> view por encima del
    # equilibrio, bajo -> por debajo, escalado por la volatilidad propia de
    # cada ticker. `views` (opcional) permite overridear cualquiera antes de
    # blendear con el prior.
    sigmas = np.sqrt(np.diag(cov))
    overrides = views or {}
    q = np.array(
        [
            overrides[t]
            if t in overrides
            else pi[i] + DEFAULT_BL_VIEW_SCALE * ((scores.get(t) or 50.0) / 100.0 - 0.5) * sigmas[i]
            for i, t in enumerate(tickers)
        ]
    )

    p = np.eye(n)
    omega = np.diag(np.diag(tau * p @ cov @ p.T)) + np.eye(n) * 1e-8  # confianza de cada view

    tau_sigma_inv = np.linalg.inv(tau * cov)
    omega_inv = np.linalg.inv(omega)
    posterior_precision = tau_sigma_inv + p.T @ omega_inv @ p
    posterior_cov_term = np.linalg.inv(posterior_precision)
    posterior_mean = posterior_cov_term @ (tau_sigma_inv @ pi + p.T @ omega_inv @ q)

    w = _max_sharpe_weights(posterior_mean, cov, risk_free_rate, max_weight)

    return {
        "weights": dict(zip(tickers, w.tolist(), strict=True)),
        "prior_returns": dict(zip(tickers, pi.tolist(), strict=True)),
        "views": dict(zip(tickers, q.tolist(), strict=True)),
        "posterior_returns": dict(zip(tickers, posterior_mean.tolist(), strict=True)),
    }


# --------------------------------------------------------------------------
# Métricas del portafolio resultante
# --------------------------------------------------------------------------


def portfolio_metrics(
    weights: dict[str, float], returns: pd.DataFrame, risk_free_rate: float = DEFAULT_RISK_FREE_RATE
) -> dict[str, float]:
    tickers = list(returns.columns)
    w = np.array([weights.get(t, 0.0) for t in tickers])
    mean_returns = _annualized_mean(returns)
    cov = _annualized_cov(returns)

    expected_return = float(w @ mean_returns)
    volatility = float(np.sqrt(w @ cov @ w))
    sharpe = (expected_return - risk_free_rate) / volatility if volatility > 1e-12 else 0.0

    return {"expected_return": expected_return, "volatility": volatility, "sharpe": sharpe}


# --------------------------------------------------------------------------
# Modelo completo: selección por clusters + Black-Litterman sobre los elegidos
# --------------------------------------------------------------------------


def build_portfolio(
    returns: pd.DataFrame,
    market_caps: dict[str, float | None],
    scores: dict[str, float | None],
    risk_free_rate: float = DEFAULT_RISK_FREE_RATE,
    views: dict[str, float] | None = None,
    n_assets: int = DEFAULT_N_ASSETS,
    max_weight: float = DEFAULT_MAX_WEIGHT,
) -> dict[str, Any]:
    """Aplica las etapas 2 y 3 sobre el universo `returns.columns` (ya filtrado y
    puntuado en la etapa 1). Devuelve la selección, los pesos, las métricas y el
    detalle de Black-Litterman."""
    if not MIN_MAX_WEIGHT <= max_weight <= 1.0:
        raise PortfolioDataError(f"El peso máximo por activo debe estar entre {MIN_MAX_WEIGHT:.0%} y 100%.")

    selection = select_by_cluster(returns, scores, n_assets)
    chosen = [s["ticker"] for s in selection]
    sub = returns[chosen]
    sub_caps = {t: market_caps.get(t) for t in chosen}
    sub_scores = {t: scores.get(t) for t in chosen}
    sub_views = {t: v for t, v in (views or {}).items() if t in chosen} or None

    bl = black_litterman_weights(sub, sub_caps, sub_scores, risk_free_rate, sub_views, max_weight=max_weight)
    return {
        "selection": selection,
        "weights": bl["weights"],
        "metrics": portfolio_metrics(bl["weights"], sub, risk_free_rate),
        "prior_returns": bl["prior_returns"],
        "views": bl["views"],
        "posterior_returns": bl["posterior_returns"],
        "max_weight": max(max_weight, 1.0 / len(chosen)),
    }
