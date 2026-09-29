"""Cálculo de métricas de riesgo financiero."""

from ..config import RISK_FREE_RATE, TRADING_DAYS_PER_YEAR


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

        etf_returns = etf_history_data["Close"].pct_change(fill_method=None).dropna()
        market_returns = market_history_data["Close"].pct_change(fill_method=None).dropna()
        min_len = min(len(etf_returns), len(market_returns))

        if min_len < 2:
            return None

        etf_returns = etf_returns[-min_len:]
        market_returns = market_returns[-min_len:]

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
