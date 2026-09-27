"""
Score de "salud" de un ticker (0-100) a partir de los datos fundamentales y
técnicos ya construidos por `fundamentals.py`.

Es una heurística de reglas simples basada en rangos típicos de análisis
fundamental/técnico (múltiplos de valoración razonables, rentabilidad,
apalancamiento, liquidez, RSI, MACD, tendencia de precio). No es un modelo
estadístico ni asesoramiento financiero: sirve para comparar rápido entre
muchos candidatos, no para reemplazar un análisis completo.
"""

from __future__ import annotations

from typing import Any

Number = float | None

CATEGORY_WEIGHTS = {
    "valuation": 0.25,
    "profitability": 0.25,
    "financial_health": 0.20,
    "shareholder_return": 0.10,
    "technical": 0.20,
}


def _piecewise(value: Number, points: list[tuple[float, float]]) -> float | None:
    """Interpola linealmente `value` sobre una curva de puntos (x, score)
    ordenada por x ascendente; fuera de rango, se satura en el extremo."""
    if value is None:
        return None
    if value <= points[0][0]:
        return points[0][1]
    if value >= points[-1][0]:
        return points[-1][1]
    for (x0, y0), (x1, y1) in zip(points, points[1:], strict=False):
        if x0 <= value <= x1:
            t = (value - x0) / (x1 - x0)
            return y0 + t * (y1 - y0)
    return points[-1][1]


def _avg(scores: list[float | None]) -> float | None:
    valid = [s for s in scores if s is not None]
    if not valid:
        return None
    return sum(valid) / len(valid)


# --- Valoración: múltiplos más bajos = mejor, dentro de rangos razonables ---


def _score_per(value: Number) -> float | None:
    if value is None:
        return None
    if value < 0:
        return 10.0  # ganancias negativas
    return _piecewise(value, [(5, 100), (15, 90), (25, 65), (40, 35), (60, 10)])


def _score_peg(value: Number) -> float | None:
    if value is None:
        return None
    if value <= 0:
        return 30.0  # crecimiento negativo o dato poco fiable
    return _piecewise(value, [(0.5, 100), (1, 90), (1.5, 70), (2, 50), (3, 25), (5, 5)])


def _score_ev_ebitda(value: Number) -> float | None:
    if value is None:
        return None
    if value < 0:
        return 10.0  # EBITDA negativo
    return _piecewise(value, [(5, 100), (10, 90), (15, 70), (20, 50), (30, 25), (45, 5)])


def _score_price_to_book(value: Number) -> float | None:
    if value is None:
        return None
    if value < 0:
        return 5.0  # patrimonio neto negativo
    return _piecewise(value, [(0.5, 90), (1, 100), (3, 85), (6, 60), (10, 35), (20, 10)])


def score_valuation(valuation: dict[str, Number]) -> float | None:
    return _avg(
        [
            _score_per(valuation.get("per_trailing")),
            _score_peg(valuation.get("peg_ratio")),
            _score_ev_ebitda(valuation.get("ev_to_ebitda")),
            _score_price_to_book(valuation.get("price_to_book")),
        ]
    )


# --- Rentabilidad y eficiencia: más alto = mejor ---


def _score_roe(value: Number) -> float | None:
    if value is None:
        return None
    return _piecewise(value, [(-0.2, 0), (0, 25), (0.08, 55), (0.15, 75), (0.25, 90), (0.40, 100)])


def _score_roa(value: Number) -> float | None:
    if value is None:
        return None
    return _piecewise(
        value, [(-0.1, 0), (0, 25), (0.03, 50), (0.06, 65), (0.10, 80), (0.15, 95), (0.25, 100)]
    )


def _score_net_margin(value: Number) -> float | None:
    if value is None:
        return None
    return _piecewise(
        value, [(-0.1, 0), (0, 20), (0.05, 45), (0.10, 60), (0.15, 75), (0.20, 85), (0.30, 100)]
    )


def score_profitability(profitability: dict[str, Number]) -> float | None:
    return _avg(
        [
            _score_roe(profitability.get("roe")),
            _score_roa(profitability.get("roa")),
            _score_net_margin(profitability.get("net_margin")),
        ]
    )


# --- Salud financiera y deuda ---


def _score_debt_to_equity(value: Number) -> float | None:
    if value is None:
        return None
    if value < 0:
        return 5.0  # patrimonio neto negativo
    return _piecewise(value, [(0, 100), (50, 90), (100, 70), (150, 50), (250, 25), (400, 5)])


def _score_current_ratio(value: Number) -> float | None:
    if value is None:
        return None
    return _piecewise(value, [(0.5, 15), (1, 50), (1.5, 75), (2, 90), (3, 100)])


def _score_fcf_yield(fcf: Number, market_cap: Number) -> float | None:
    if fcf is None or not market_cap:
        return None
    fcf_yield = fcf / market_cap
    return _piecewise(fcf_yield, [(-0.05, 5), (0, 35), (0.02, 60), (0.04, 75), (0.06, 88), (0.10, 100)])


def score_financial_health(financial_health: dict[str, Number], market_cap: Number) -> float | None:
    return _avg(
        [
            _score_debt_to_equity(financial_health.get("debt_to_equity")),
            _score_current_ratio(financial_health.get("current_ratio")),
            _score_fcf_yield(financial_health.get("free_cash_flow"), market_cap),
        ]
    )


# --- Retorno al accionista ---


def _score_dividend_yield(value: Number) -> float | None:
    if value is None:
        return None
    # Sube hasta un dividendo "saludable" (~5%) y baja de nuevo para yields
    # muy altos, que suelen señalar riesgo de recorte más que fortaleza.
    return _piecewise(value, [(0, 45), (0.02, 65), (0.035, 85), (0.05, 100), (0.09, 70), (0.15, 30)])


def score_shareholder_return(shareholder_return: dict[str, Number]) -> float | None:
    return _score_dividend_yield(shareholder_return.get("dividend_yield"))


# --- Análisis técnico ---


def _score_rsi(value: Number) -> float | None:
    if value is None:
        return None
    return _piecewise(value, [(20, 25), (30, 40), (45, 80), (60, 95), (70, 75), (80, 40), (90, 15)])


def _score_macd(macd_data: dict[str, float] | None) -> float | None:
    if not macd_data:
        return None
    hist = macd_data.get("histogram")
    if hist is None:
        return None
    # Histograma > 0 = MACD por encima de la señal (impulso alcista).
    return _piecewise(hist, [(-3, 10), (0, 50), (3, 90)])


def _score_trend(technical: dict[str, Any], price: Number) -> float | None:
    sma50 = technical.get("sma_50")
    sma200 = technical.get("sma_200")
    if price is None or sma50 is None or sma200 is None:
        return None
    hits = sum([price > sma50, price > sma200, sma50 > sma200])
    return {0: 10.0, 1: 35.0, 2: 70.0, 3: 100.0}[hits]


def score_technical(technical: dict[str, Any], price: Number) -> float | None:
    return _avg(
        [
            _score_rsi(technical.get("rsi_14")),
            _score_macd(technical.get("macd")),
            _score_trend(technical, price),
        ]
    )


def _label(score: float) -> str:
    if score >= 75:
        return "Fuerte"
    if score >= 60:
        return "Sólido"
    if score >= 45:
        return "Neutral"
    if score >= 30:
        return "Débil"
    return "Muy débil"


def evaluate_score(evaluation: dict[str, Any], market_cap: Number = None) -> dict[str, Any]:
    """Calcula el score de salud (0-100) de un ticker a partir de su
    evaluación (valoración, rentabilidad, salud financiera, retorno al
    accionista y técnico ya armados por `fundamentals.py`).

    Cada categoría se promedia sobre las métricas disponibles (si a un
    ticker le falta un dato, esa métrica simplemente no entra en el
    promedio). El score global es el promedio ponderado de las categorías
    disponibles, renormalizando los pesos si falta alguna categoría entera.
    """
    price = evaluation.get("price")
    mc = market_cap if market_cap is not None else evaluation.get("market_cap")

    category_scores = {
        "valuation": score_valuation(evaluation.get("valuation") or {}),
        "profitability": score_profitability(evaluation.get("profitability") or {}),
        "financial_health": score_financial_health(evaluation.get("financial_health") or {}, mc),
        "shareholder_return": score_shareholder_return(evaluation.get("shareholder_return") or {}),
        "technical": score_technical(evaluation.get("technical") or {}, price),
    }

    weighted_sum = 0.0
    weight_total = 0.0
    for key, score in category_scores.items():
        if score is None:
            continue
        weight = CATEGORY_WEIGHTS[key]
        weighted_sum += score * weight
        weight_total += weight

    overall = round(weighted_sum / weight_total, 1) if weight_total > 0 else None
    rounded_categories = {k: (round(v, 1) if v is not None else None) for k, v in category_scores.items()}

    return {
        "overall": overall,
        "label": _label(overall) if overall is not None else "Sin datos",
        "categories": rounded_categories,
    }
