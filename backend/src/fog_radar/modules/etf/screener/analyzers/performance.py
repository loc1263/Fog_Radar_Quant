"""Cálculo de métricas de performance."""

import math

import pandas as pd

from ..config import NA_VALUE, PERFORMANCE_COLUMNS


class PerformanceAnalyzer:
    """Calcula métricas de performance de ETFs."""

    @staticmethod
    def get_performance_metrics(current_price, hist_data):
        """Calcula performance para diferentes períodos usando historial descargado de yfinance para todos los períodos."""
        perf_5d = PerformanceAnalyzer._calc_performance(current_price, hist_data["hist_5d"], 0, None)
        perf_1m = PerformanceAnalyzer._calc_performance(current_price, hist_data["hist_1m"], 0, None)
        perf_6m = PerformanceAnalyzer._calc_performance(current_price, hist_data["hist_6m"], 0, None)
        perf_1y = PerformanceAnalyzer._calc_performance(current_price, hist_data["hist_1y"], 0, None)
        perf_ytd = PerformanceAnalyzer._calc_performance(current_price, hist_data["hist_ytd"], 0, None)
        perf_5y = PerformanceAnalyzer._calc_performance(current_price, hist_data["hist_5y"], 0, None)

        return (perf_5d, perf_1m, perf_6m, perf_1y, perf_ytd, perf_5y)

    @staticmethod
    def _calc_performance(current_price, history_data, idx, min_len):
        """Calcula performance para un período específico."""
        if min_len and len(history_data) < min_len:
            return None

        if history_data.empty:
            return None

        try:
            old_price = history_data["Close"].iloc[idx]
            return ((current_price / old_price) - 1) * 100
        except IndexError:
            return None

    @staticmethod
    def format_performance(perf):
        """Formatea un valor de performance."""
        if perf is None:
            return NA_VALUE

        if isinstance(perf, float) and math.isnan(perf):
            return NA_VALUE

        return round(perf, 2)

    @staticmethod
    def get_performance_columns_from_df(df, all_columns=PERFORMANCE_COLUMNS):
        """Obtiene columnas de performance que existen en el DataFrame."""
        return [col for col in all_columns if col in df.columns]

    @staticmethod
    def is_strictly_increasing_performance(row, performance_columns):
        """Verifica si performance es estrictamente creciente."""
        vals = [row[col] for col in performance_columns if pd.notna(row[col]) and row[col] != NA_VALUE]

        try:
            vals = [float(v) for v in vals]
        except (ValueError, TypeError):
            return False

        if len(vals) < 2:
            return False

        return all(x > 0 for x in vals) and all(vals[i] < vals[i + 1] for i in range(len(vals) - 1))
