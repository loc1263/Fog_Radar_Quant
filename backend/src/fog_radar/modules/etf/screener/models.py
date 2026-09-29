"""Modelos de datos para ETFs."""

from dataclasses import dataclass

import pandas as pd


@dataclass
class ETFData:
    """Datos básicos de un ETF."""

    ticker: str
    info: dict
    price: float
    hist: pd.DataFrame


@dataclass
class ReturnsData:
    """Retornos de performance por período."""

    perf_5d: float
    perf_1m: float
    perf_6m: float
    perf_1y: float
    perf_ytd: float
    perf_5y: float


@dataclass
class RiskMetrics:
    """Métricas de riesgo de un ETF."""

    sharpe_ratio: float
    treynor_ratio: float
    beta: float
