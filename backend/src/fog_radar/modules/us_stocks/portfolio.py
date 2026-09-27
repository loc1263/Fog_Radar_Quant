"""
Optimización de portafolios sobre los candidatos de la corrida batch más
reciente: Markowitz clásico (máximo Sharpe), Black-Litterman y Hierarchical
Risk Parity (HRP).

Igual que `scoring.py`, son funciones puras sobre datos ya persistidos por
el batch (`prices.json`, `candidates.json`): nunca consultan Yahoo Finance.
Si al armado del portafolio le faltan datos (un ticker sin histórico común
suficiente), se levanta `PortfolioDataError` en lugar de intentar bajarlos
acá — eso es responsabilidad exclusiva de `app/batch.py` en su próxima
corrida.

Los 3 modelos son **long-only** (pesos >= 0, suman 1): es el caso de uso
típico de un inversor retail armando una cartera real, y evita que
Markowitz/Black-Litterman devuelvan posiciones en corto poco realistas para
este dataset.
"""

from __future__ import annotations

import itertools
import json
import math
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from scipy.cluster.hierarchy import linkage
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

MIN_COMBINATION_SIZE = 2
MAX_COMBINATION_SIZE = 10
# Tope duro de combinaciones a evaluar por búsqueda. Medido con datos reales
# (no sintéticos): Markowitz/Black-Litterman rondan ~160 combos/s (resuelven
# un SLSQP por combinación, más lento cuanto más correlacionados están los
# retornos reales), HRP bastante más rápido. Con este tope, el caso más
# lento tarda ~15s en un solo request HTTP síncrono — no minutos.
MAX_COMBINATIONS = 2500
RANK_METRICS = ("expected_return", "sharpe")


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
# Markowitz clásico: portafolio tangente (máximo Sharpe), long-only
# --------------------------------------------------------------------------


def _max_sharpe_weights(mean_returns: np.ndarray, cov: np.ndarray, risk_free_rate: float) -> np.ndarray:
    n = len(mean_returns)

    def neg_sharpe(w: np.ndarray) -> float:
        ret = w @ mean_returns
        vol = float(np.sqrt(w @ cov @ w))
        if vol <= 1e-12:
            return 1e6
        return -(ret - risk_free_rate) / vol

    constraints = [{"type": "eq", "fun": lambda w: np.sum(w) - 1.0}]
    bounds = [(0.0, 1.0)] * n
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


def markowitz_weights(
    returns: pd.DataFrame, risk_free_rate: float = DEFAULT_RISK_FREE_RATE
) -> dict[str, float]:
    tickers = list(returns.columns)
    mean_returns = _annualized_mean(returns)
    cov = _annualized_cov(returns)
    w = _max_sharpe_weights(mean_returns, cov, risk_free_rate)
    return dict(zip(tickers, w.tolist(), strict=True))


def _min_variance_for_target(mean_returns: np.ndarray, cov: np.ndarray, target: float) -> np.ndarray | None:
    n = len(mean_returns)

    def variance(w: np.ndarray) -> float:
        return float(w @ cov @ w)

    constraints = [
        {"type": "eq", "fun": lambda w: np.sum(w) - 1.0},
        {"type": "eq", "fun": lambda w: w @ mean_returns - target},
    ]
    bounds = [(0.0, 1.0)] * n

    # SLSQP puede quedar atrapado lejos del óptimo real según el punto de
    # partida, sobre todo cerca de los extremos de la frontera (donde la
    # solución tiende a concentrarse en un solo activo). Probamos varios x0
    # y nos quedamos con el factible de menor varianza, para que la frontera
    # salga monótona en vez de zigzagueante.
    closest = int(np.argmin(np.abs(mean_returns - target)))
    one_hot = np.zeros(n)
    one_hot[closest] = 1.0
    candidates_x0 = [np.repeat(1.0 / n, n), one_hot, 0.5 * np.repeat(1.0 / n, n) + 0.5 * one_hot]

    best: np.ndarray | None = None
    best_var = np.inf
    for x0 in candidates_x0:
        result = minimize(
            variance,
            x0,
            method="SLSQP",
            bounds=bounds,
            constraints=constraints,
            options={"maxiter": 500, "ftol": 1e-12},
        )
        if not result.success:
            continue
        if abs(result.x.sum() - 1.0) > 1e-4 or abs(result.x @ mean_returns - target) > 1e-4:
            continue
        var = float(result.x @ cov @ result.x)
        if var < best_var:
            best_var = var
            best = result.x

    return best


def _global_min_variance_return(mean_returns: np.ndarray, cov: np.ndarray) -> float:
    """Retorno del portafolio de varianza mínima (sin objetivo de retorno).
    Marca dónde arranca la rama *eficiente* de la frontera: por debajo de
    este retorno, minimizar varianza para un target más bajo en realidad
    requiere MÁS volatilidad (rama ineficiente), así que no se muestra."""
    n = len(mean_returns)

    def variance(w: np.ndarray) -> float:
        return float(w @ cov @ w)

    result = minimize(
        variance,
        np.repeat(1.0 / n, n),
        method="SLSQP",
        bounds=[(0.0, 1.0)] * n,
        constraints=[{"type": "eq", "fun": lambda w: np.sum(w) - 1.0}],
        options={"maxiter": 500, "ftol": 1e-12},
    )
    if not result.success:
        return float(mean_returns.min())
    return float(result.x @ mean_returns)


def markowitz_efficient_frontier(returns: pd.DataFrame, n_points: int = 25) -> list[dict[str, float]]:
    """Frontera eficiente (retorno, volatilidad) long-only, para graficar en
    el frontend junto con el portafolio de máximo Sharpe. Solo la rama
    eficiente (desde el portafolio de varianza mínima hacia arriba) — la
    rama inferior no es de interés para un inversor y rompería la
    monotonía del gráfico."""
    mean_returns = _annualized_mean(returns)
    cov = _annualized_cov(returns)

    hi = float(mean_returns.max())
    lo = max(_global_min_variance_return(mean_returns, cov), float(mean_returns.min()))
    if hi <= lo:
        return []

    points: list[dict[str, float]] = []
    for target in np.linspace(lo, hi, n_points):
        w = _min_variance_for_target(mean_returns, cov, float(target))
        if w is None:
            continue
        vol = float(np.sqrt(w @ cov @ w))
        points.append({"return": float(target), "volatility": vol})
    return points


# --------------------------------------------------------------------------
# Black-Litterman: prior de equilibrio de mercado + views automáticas
# derivadas del score de salud, blendeadas con la fórmula estándar de BL.
# --------------------------------------------------------------------------


def black_litterman_weights(
    returns: pd.DataFrame,
    market_caps: dict[str, float | None],
    scores: dict[str, float | None],
    risk_free_rate: float = DEFAULT_RISK_FREE_RATE,
    views: dict[str, float] | None = None,
    risk_aversion: float = DEFAULT_RISK_AVERSION,
    tau: float = DEFAULT_TAU,
) -> dict[str, Any]:
    tickers = list(returns.columns)
    n = len(tickers)
    cov = _annualized_cov(returns)

    caps = np.array([max(market_caps.get(t) or 0.0, 0.0) for t in tickers])
    w_mkt = caps / caps.sum() if caps.sum() > 0 else np.repeat(1.0 / n, n)

    # Prior de equilibrio (reverse optimization): π = δ·Σ·w_mkt
    pi = risk_aversion * (cov @ w_mkt)

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

    w = _max_sharpe_weights(posterior_mean, cov, risk_free_rate)

    return {
        "weights": dict(zip(tickers, w.tolist(), strict=True)),
        "prior_returns": dict(zip(tickers, pi.tolist(), strict=True)),
        "views": dict(zip(tickers, q.tolist(), strict=True)),
        "posterior_returns": dict(zip(tickers, posterior_mean.tolist(), strict=True)),
    }


# --------------------------------------------------------------------------
# Hierarchical Risk Parity (López de Prado): clustering jerárquico sobre la
# distancia de correlación + bisección recursiva por varianza inversa. No
# requiere estimar retornos esperados, así que no depende de las views.
# --------------------------------------------------------------------------


def _quasi_diag(link: np.ndarray, n_items: int) -> list[int]:
    def recurse(node_idx: int) -> list[int]:
        if node_idx < n_items:
            return [node_idx]
        left, right = link[node_idx - n_items, 0], link[node_idx - n_items, 1]
        return recurse(int(left)) + recurse(int(right))

    root = 2 * n_items - 2
    return recurse(root)


def _cluster_var(cov: np.ndarray, idx: list[int]) -> float:
    sub_cov = cov[np.ix_(idx, idx)]
    ivp = 1.0 / np.diag(sub_cov)
    ivp /= ivp.sum()
    return float(ivp @ sub_cov @ ivp)


def _recursive_bisection(cov: np.ndarray, sorted_idx: list[int]) -> np.ndarray:
    """Pesos en el mismo orden que `sorted_idx` (posiciones, no índices de
    `cov`). Opera en numpy puro — nada de indexado por label de pandas, que
    es el cuello de botella cuando esto se llama miles de veces en la
    búsqueda combinatoria de `best_combination`."""
    n = len(sorted_idx)
    weights = np.ones(n)
    clusters = [list(range(n))]  # posiciones dentro de sorted_idx/weights
    while len(clusters) > 0:
        clusters = [
            c[start:end]
            for c in clusters
            for start, end in ((0, len(c) // 2), (len(c) // 2, len(c)))
            if len(c) > 1
        ]
        for i in range(0, len(clusters), 2):
            left_pos, right_pos = clusters[i], clusters[i + 1]
            var_left = _cluster_var(cov, [sorted_idx[p] for p in left_pos])
            var_right = _cluster_var(cov, [sorted_idx[p] for p in right_pos])
            alpha = 1.0 - var_left / (var_left + var_right)
            weights[left_pos] *= alpha
            weights[right_pos] *= 1.0 - alpha
    return weights


def hrp_weights(returns: pd.DataFrame) -> dict[str, float]:
    tickers = list(returns.columns)
    cov = (returns.cov() * TRADING_DAYS).to_numpy()
    corr = returns.corr().clip(-1.0, 1.0).to_numpy()

    dist = np.sqrt(0.5 * (1.0 - corr))
    np.fill_diagonal(dist, 0.0)
    condensed = squareform(dist, checks=False)
    link = linkage(condensed, method="single")

    sorted_idx = _quasi_diag(link, n_items=len(tickers))
    sorted_weights = _recursive_bisection(cov, sorted_idx)

    weights = np.zeros(len(tickers))
    weights[sorted_idx] = sorted_weights
    return dict(zip(tickers, weights.tolist(), strict=True))


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
# Búsqueda combinatoria: mejor portafolio de tamaño FIJO k dentro de un
# universo de tickers. Distinto de simplemente optimizar sobre todo el
# universo: ahí el optimizador ya puede dejar cualquier ticker en 0% (así
# que ya es "lo mejor entre todas las combinaciones posibles" sin necesidad
# de fuerza bruta), pero acá el pedido es otro — un portafolio de
# EXACTAMENTE k tickers — y eso sí es un problema combinatorio real que el
# optimizador continuo no resuelve solo.
# --------------------------------------------------------------------------


def _weights_for_method(
    method: str,
    sub_returns: pd.DataFrame,
    market_caps: dict[str, float | None],
    scores: dict[str, float | None],
    risk_free_rate: float,
) -> dict[str, float]:
    if method == "markowitz":
        return markowitz_weights(sub_returns, risk_free_rate)
    if method == "black_litterman":
        sub_caps = {t: market_caps.get(t) for t in sub_returns.columns}
        sub_scores = {t: scores.get(t) for t in sub_returns.columns}
        return black_litterman_weights(sub_returns, sub_caps, sub_scores, risk_free_rate)["weights"]
    if method == "hrp":
        return hrp_weights(sub_returns)
    raise PortfolioDataError(f"Método desconocido: {method}")


def best_combinations(
    returns: pd.DataFrame,
    method: str,
    k: int,
    market_caps: dict[str, float | None] | None = None,
    scores: dict[str, float | None] | None = None,
    risk_free_rate: float = DEFAULT_RISK_FREE_RATE,
    rank_by: str = "expected_return",
    top_n: int = 5,
) -> dict[str, Any]:
    """Evalúa TODAS las combinaciones de `k` tickers del universo
    (`returns.columns`), optimiza cada una con el objetivo nativo del
    método (máximo Sharpe en Markowitz/Black-Litterman, paridad de riesgo en
    HRP — la búsqueda solo decide QUÉ k tickers entran, no cambia cómo se
    pesan una vez elegidos) y devuelve las `top_n` mejores combinaciones
    según `rank_by`.
    """
    if rank_by not in RANK_METRICS:
        raise PortfolioDataError(f"rank_by inválido: {rank_by} (opciones: {', '.join(RANK_METRICS)}).")

    tickers = list(returns.columns)
    n = len(tickers)
    if k < MIN_COMBINATION_SIZE:
        raise PortfolioDataError(f"El portafolio necesita al menos {MIN_COMBINATION_SIZE} tickers.")
    if k > MAX_COMBINATION_SIZE:
        raise PortfolioDataError(f"El portafolio no puede tener más de {MAX_COMBINATION_SIZE} tickers.")
    if k > n:
        raise PortfolioDataError(
            f"Pediste combinaciones de {k} tickers pero el universo seleccionado solo tiene {n}."
        )

    total = math.comb(n, k)
    if total > MAX_COMBINATIONS:
        raise PortfolioDataError(
            f"Esa búsqueda tiene {total:,} combinaciones posibles (universo de {n} tickers, "
            f"portafolios de {k}), más que el máximo soportado por búsqueda ({MAX_COMBINATIONS:,}). "
            "Achicá el universo seleccionado o el tamaño del portafolio."
        )

    market_caps = market_caps or {}
    scores = scores or {}

    results: list[dict[str, Any]] = []
    for combo in itertools.combinations(tickers, k):
        sub_returns = returns[list(combo)]
        try:
            weights = _weights_for_method(method, sub_returns, market_caps, scores, risk_free_rate)
        except PortfolioDataError:
            continue  # combinación degenerada (p. ej. covarianza casi singular): se descarta, no corta la búsqueda
        metrics = portfolio_metrics(weights, sub_returns, risk_free_rate)
        results.append({"tickers": list(combo), "weights": weights, "metrics": metrics})

    if not results:
        raise PortfolioDataError("Ninguna combinación pudo optimizarse con los datos disponibles.")

    results.sort(key=lambda r: r["metrics"][rank_by], reverse=True)

    return {"combinations_evaluated": len(results), "results": results[:top_n]}
