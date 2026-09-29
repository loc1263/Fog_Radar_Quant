"""Cálculo de métricas de riesgo financiero."""

import pandas as pd

from ..config import RISK_FREE_RATE, TRADING_DAYS_PER_YEAR


def _por_fecha(serie):
    """Índice por fecha calendario (sin hora ni zona) para alinear dos series diarias."""
    if not isinstance(serie.index, pd.DatetimeIndex):
        return serie  # sin fechas (p. ej. tests): se alinea por la etiqueta tal cual
    idx = serie.index.tz_localize(None) if serie.index.tz is not None else serie.index
    out = serie.copy()
    out.index = idx.normalize()
    return out[~out.index.duplicated(keep="last")]


class RiskAnalyzer:
    """Calcula métricas de riesgo de ETFs."""

    @staticmethod
    def calculate_sharpe_ratio(history_data, risk_free_rate=RISK_FREE_RATE):
        """Calcula el ratio de Sharpe."""
        if history_data.empty or "Close" not in history_data.columns:
            return None

        returns = history_data["Close"].pct_change(fill_method=None).dropna()
        if len(returns) < 2:
            return None

        mean_return = returns.mean() * TRADING_DAYS_PER_YEAR
        std_return = returns.std() * (TRADING_DAYS_PER_YEAR**0.5)

        if std_return != 0:
            sharpe = (mean_return - risk_free_rate) / std_return
            return round(sharpe, 3)

        return None

    @staticmethod
    def calculate_beta(etf_history_data, market_history_data):
        """Calcula el beta de un ETF respecto al mercado."""
        if etf_history_data.empty or market_history_data.empty:
            return None

        # Fog Radar (Fase 7, corrección): los retornos se emparejan POR FECHA
        # (join interno). El original tomaba los últimos N de cada serie por
        # posición: si al ETF le faltaba un día, todo lo anterior quedaba
        # desfasado un día respecto del mercado y el beta se sesgaba hacia 0.
        etf_returns = _por_fecha(etf_history_data["Close"].pct_change(fill_method=None))
        market_returns = _por_fecha(market_history_data["Close"].pct_change(fill_method=None))
        pares = pd.concat([etf_returns, market_returns], axis=1, join="inner").dropna()

        if len(pares) < 2:
            return None

        etf_returns, market_returns = pares.iloc[:, 0], pares.iloc[:, 1]
        cov = etf_returns.cov(market_returns)
        var = market_returns.var()
        beta = cov / var if var != 0 else None

        return round(beta, 3) if beta is not None else None

    @staticmethod
    def calculate_treynor_ratio(etf_history_data, market_history_data, risk_free_rate=RISK_FREE_RATE):
        """Calcula el ratio de Treynor para un ETF."""
        beta = RiskAnalyzer.calculate_beta(etf_history_data, market_history_data)

        if beta is None or beta == 0:
            return None

        returns = etf_history_data["Close"].pct_change(fill_method=None).dropna()

        if len(returns) < 2:
            return None

        mean_return = returns.mean() * TRADING_DAYS_PER_YEAR
        treynor = (mean_return - risk_free_rate) / beta

        return round(treynor, 3)
