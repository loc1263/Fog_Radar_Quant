"""Paridad con Stock_Choice_EEUU sobre una corrida real (25-09-2026).

``fixtures/runs/<run_id>/`` son los archivos que dejó el batch original.
``fixtures/portfolio_expected.json`` se generó ejecutando el ``portfolio.py``
original sobre esos mismos precios.
"""

from __future__ import annotations

import json
from pathlib import Path

import pandas as pd
import pytest

from fog_radar.modules.us_stocks import portfolio as p
from fog_radar.modules.us_stocks import scanner as s
from fog_radar.modules.us_stocks.scoring import evaluate_score

FIXTURES = Path(__file__).with_name("fixtures")
RUN = FIXTURES / "runs" / "2026-09-25T21-00-01Z"
EXPECTED = json.loads((FIXTURES / "portfolio_expected.json").read_text(encoding="utf-8"))
CANDIDATES = json.loads((RUN / "candidates.json").read_text(encoding="utf-8"))["rows"]
PRICES = json.loads((RUN / "prices.json").read_text(encoding="utf-8"))

RETURN_KEYS = [
    "price",
    "momentum_12_1",
    "return_1d",
    "return_5d",
    "return_3m",
    "return_6m",
    "return_1y",
    "return_ytd",
]


def _approx_dict(got: dict, exp: dict, tol: float = 1e-7) -> None:
    assert set(got) == set(exp)
    for k in exp:
        assert got[k] == pytest.approx(exp[k], abs=tol), k


def test_score_identico_para_todos_los_candidatos() -> None:
    assert len(CANDIDATES) > 40
    for row in CANDIDATES:
        assert evaluate_score(row, market_cap=row["market_cap"]) == row["score"], row["ticker"]


def test_momentum_y_retornos_desde_prices_json() -> None:
    dates = pd.to_datetime(PRICES["dates"])
    for row in CANDIDATES:
        closes = pd.Series(PRICES["prices"][row["ticker"]], index=dates, dtype=float)
        m = s._compute_momentum(closes)
        assert m is not None
        for k in RETURN_KEYS:
            if row[k] is None:
                assert m[k] is None, (row["ticker"], k)
            else:
                assert m[k] == pytest.approx(row[k], rel=1e-9), (row["ticker"], k)


@pytest.fixture(scope="module")
def returns_top15() -> pd.DataFrame:
    return p.daily_returns(p.load_price_matrix(RUN, EXPECTED["tickers"]))


def test_markowitz(returns_top15: pd.DataFrame) -> None:
    _approx_dict(p.markowitz_weights(returns_top15), EXPECTED["markowitz"], tol=1e-5)
    frontier = p.markowitz_efficient_frontier(returns_top15)
    assert len(frontier) == len(EXPECTED["frontier"])
    for got, exp in zip(frontier, EXPECTED["frontier"], strict=True):
        assert got["return"] == pytest.approx(exp["return"], abs=1e-9)
        assert got["volatility"] == pytest.approx(exp["volatility"], abs=1e-5)


def test_black_litterman(returns_top15: pd.DataFrame) -> None:
    caps = {r["ticker"]: r["market_cap"] for r in CANDIDATES}
    scores = {r["ticker"]: r["score"]["overall"] for r in CANDIDATES}
    got = p.black_litterman_weights(returns_top15, caps, scores)
    # Corrección de la Fase 7 (docs/PARIDAD.md): π, Q y el posterior son retornos
    # totales (exceso + rf). Con P = I el posterior se desplaza exactamente en rf.
    rf = p.DEFAULT_RISK_FREE_RATE
    for key in ("prior_returns", "views", "posterior_returns"):
        esperado = {t: v + rf for t, v in EXPECTED["black_litterman"][key].items()}
        _approx_dict(got[key], esperado, tol=1e-9)
    assert sum(got["weights"].values()) == pytest.approx(1.0)


def test_black_litterman_con_views_neutras_recupera_el_mercado(returns_top15: pd.DataFrame) -> None:
    """Propiedad básica de BL: sin views (scores = 50) el tangente es w_mkt.
    El original (sin sumar rf al prior) daba otros pesos."""
    caps = {r["ticker"]: r["market_cap"] for r in CANDIDATES if r["ticker"] in returns_top15.columns}
    total = sum(caps.values())
    got = p.black_litterman_weights(returns_top15, caps, dict.fromkeys(caps, 50.0))
    for t, w in got["weights"].items():
        assert w == pytest.approx(caps[t] / total, abs=2e-3), t


def test_hrp(returns_top15: pd.DataFrame) -> None:
    _approx_dict(p.hrp_weights(returns_top15), EXPECTED["hrp"], tol=1e-12)


@pytest.mark.parametrize(
    ("key", "method", "kwargs"),
    [
        ("best_hrp_k3", "hrp", {"rank_by": "sharpe", "top_n": 3}),
        ("best_markowitz_k3", "markowitz", {"top_n": 3}),
        ("best_bl_k3", "black_litterman", {"top_n": 3, "with_bl": True}),
    ],
)
def test_best_combinations(key: str, method: str, kwargs: dict) -> None:
    rets = p.daily_returns(p.load_price_matrix(RUN, EXPECTED["sub"]))
    if kwargs.pop("with_bl", False):
        kwargs["market_caps"] = {r["ticker"]: r["market_cap"] for r in CANDIDATES}
        kwargs["scores"] = {r["ticker"]: r["score"]["overall"] for r in CANDIDATES}
    got = p.best_combinations(rets, method, 3, **kwargs)
    exp = EXPECTED[key]
    assert got["combinations_evaluated"] == exp["combinations_evaluated"]
    if method == "black_litterman":
        return  # pesos distintos por la corrección de BL (Fase 7); ver test_black_litterman
    assert [r["tickers"] for r in got["results"]] == [r["tickers"] for r in exp["results"]]
    for g, e in zip(got["results"], exp["results"], strict=True):
        _approx_dict(g["weights"], e["weights"], tol=1e-5)
