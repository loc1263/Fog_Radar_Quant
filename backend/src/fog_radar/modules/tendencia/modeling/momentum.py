"""
Rankings de momentum (mensual y semanal) sobre el universo COMPLETO de acciones
(no solo las TOP_N_SELECTION elegidas por el modelo de Markowitz).

Metodología (factor de momentum clásico, adaptado a la base semanal del
pipeline; deliberadamente simple y transparente para el MVP):
  1. Se re-muestrea prices_usd.csv (precios semanales en USD, ver
     ingestion.build_dataset) a velas mensuales (último precio de cada
     mes calendario).
  2. Para cada ticker del universo se calcula el retorno acumulado en
     4 ventanas: 1, 3, 6 y 12 meses (MOMENTUM_MONTHS_* en settings.py).
  3. Se calcula un "momentum_score" = promedio simple de esas 4 métricas
     de retorno, ajustado por la proporción de meses positivos en los
     últimos 12 meses (consistencia de la tendencia, no solo su
     magnitud). Esto NO es un ranking ajustado por riesgo: es una medida
     de fuerza y persistencia de la tendencia reciente, pensada para
     complementar (no reemplazar) la selección de Markowitz y la
     proyección a 6 meses.
  4. Se rankean todos los tickers con suficiente historia mensual
     (MIN_MONTHS_REQUIRED_MOMENTUM) y se reportan los TOP_N_MOMENTUM con
     mejor momentum mensual.

Además del mensual se calcula un momentum SEMANAL con exactamente la
misma lógica pero sobre velas semanales (5 días hábiles): ventanas de
1, 2, 3 y 4 semanas (MOMENTUM_WEEKS_* en settings.py) y consistencia =
proporción de semanas positivas en las últimas 4. Capta impulso de
muy corto plazo que el mensual suaviza. Ver FREQUENCIES.

Al usar TODO el universo descubierto (no solo las 15 de Markowitz), este
análisis puede sacar a la luz acciones con tendencia mensual fuerte que
el modelo de covarianza descartó por volatilidad o correlación alta con
el resto del portafolio.

Calcula este ranking una vez por cada análisis configurado en
config.ANALYSES (ver settings.py): el universo completo y cualquier
análisis complementario (p. ej. "solo EEUU"), para que sea comparable
con el resto de esa sección. Un análisis se omite si tiene menos
tickers con datos que su `min_tickers`.

Ejecutar (vía wrapper de compatibilidad en la raíz del proyecto):
    python step06_monthly_momentum.py

Genera, dentro de la carpeta de la corrida actual (output/<YYYYMMDD>_<NN>,
ver tendencia.config.get_run_dir):
    top_momentum_mensual<key>.csv (uno por análisis disponible)
    top_momentum_semanal<key>.csv (ídem, momentum semanal)
"""

import os

import numpy as np
import pandas as pd

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.ingestion.build_dataset import load_prices_usd
from fog_radar.modules.tendencia.logging_utils import get_logger

logger = get_logger("step06_momentum", config.LOGS_DIR, log_filename="step06_monthly_momentum.log")


def resample_monthly(prices):
    """Precios semanales -> último precio disponible de cada mes calendario."""
    return prices.resample("ME").last()


def month_return(monthly_prices, months_back):
    """Retorno simple entre el cierre de hace `months_back` velas (meses o semanas) y el último cierre disponible."""
    s = monthly_prices.dropna()
    if len(s) <= months_back:
        return np.nan
    return float(s.iloc[-1] / s.iloc[-1 - months_back] - 1)


def positive_months_ratio(monthly_prices, months_back):
    """Fracción de velas (meses o semanas) con retorno positivo dentro de las últimas `months_back`."""
    s = monthly_prices.dropna()
    if len(s) <= months_back:
        return np.nan
    rets = s.pct_change().dropna().iloc[-months_back:]
    if rets.empty:
        return np.nan
    return float((rets > 0).mean())


def momentum_score(mom_1m, mom_3m, mom_6m, mom_12m, pct_positive_12m):
    """
    Score compuesto: promedio de las 4 ventanas de retorno (fracción, no
    %), escalado por consistencia (proporción de meses positivos del
    último año). Un score alto requiere tanto retorno fuerte como
    persistencia; una racha de un solo mes bueno pesa menos que una
    tendencia sostenida. Devuelve NaN si falta cualquier ventana de
    retorno (no se puede rankear con datos incompletos).
    """
    components = [mom_1m, mom_3m, mom_6m, mom_12m]
    if any(pd.isna(c) for c in components):
        return np.nan
    avg_return = sum(components) / len(components)
    consistency = pct_positive_12m if pd.notna(pct_positive_12m) else 0.5
    return avg_return * (0.5 + 0.5 * consistency)


def resample_weekly(prices):
    """Último precio disponible de cada semana (vela de 5 días hábiles).
    prices_usd.csv ya es semanal, así que en la práctica no cambia nada;
    se re-muestrea igual para que el cálculo no dependa de eso."""
    return prices.resample("W").last()


# Las dos frecuencias de momentum: misma lógica (4 ventanas de retorno +
# consistencia sobre la ventana más larga), distinta vela. `unit` es el
# sufijo de las columnas de salida ("m" = meses, "s" = semanas).
FREQUENCIES = {
    "mensual": {
        "resample": resample_monthly,
        "unit": "m",
        "positive_col": "pct_meses_positivos_12m",
        "windows": lambda: (
            config.MOMENTUM_MONTHS_SHORT,
            config.MOMENTUM_MONTHS_MED,
            config.MOMENTUM_MONTHS_LONG,
            config.MOMENTUM_MONTHS_LONGEST,
        ),
        "min_periods": lambda: config.MIN_MONTHS_REQUIRED_MOMENTUM,
        "period_label": "meses",
    },
    "semanal": {
        "resample": resample_weekly,
        "unit": "s",
        "positive_col": "pct_semanas_positivas_4s",
        "windows": lambda: (
            config.MOMENTUM_WEEKS_SHORT,
            config.MOMENTUM_WEEKS_MED,
            config.MOMENTUM_WEEKS_LONG,
            config.MOMENTUM_WEEKS_LONGEST,
        ),
        "min_periods": lambda: config.MIN_WEEKS_REQUIRED_MOMENTUM,
        "period_label": "semanas",
    },
}


def compute_momentum_table(prices, frequency="mensual"):
    """
    prices: DataFrame de precios en USD, índice de fechas (semanal o
    mensual), una columna por ticker. `frequency`: "mensual" o "semanal"
    (ver FREQUENCIES). Devuelve un DataFrame rankeado por
    momentum_score_pct (descendente), con columna "rank" ya asignada.
    """
    freq = FREQUENCIES[frequency]
    windows = freq["windows"]()
    unit = freq["unit"]
    candles = freq["resample"](prices)
    rows = []
    for ticker in candles.columns:
        s = candles[ticker]
        if len(s.dropna()) < freq["min_periods"]():
            continue

        moms = [month_return(s, w) for w in windows]
        pct_pos = positive_months_ratio(s, windows[-1])
        score = momentum_score(*moms, pct_pos)

        name, country, _currency = config.ALL_TICKERS.get(ticker, (ticker, "", ""))
        row = {
            "ticker": ticker,
            "nombre": name,
            "pais": country,
            "sector": config.SECTOR_MAP.get(ticker, "Otro"),
            "precio_actual_usd": round(float(s.dropna().iloc[-1]), 2),
        }
        for w, m in zip(windows, moms, strict=True):
            row[f"momentum_{w}{unit}_pct"] = round(m * 100, 2) if pd.notna(m) else None
        row[freq["positive_col"]] = round(pct_pos * 100, 1) if pd.notna(pct_pos) else None
        row["momentum_score_pct"] = round(score * 100, 2) if pd.notna(score) else None
        rows.append(row)

    out = pd.DataFrame(rows)
    if out.empty:
        return out

    out = out.dropna(subset=["momentum_score_pct"])
    out = out.sort_values("momentum_score_pct", ascending=False).reset_index(drop=True)
    out.insert(0, "rank", out.index + 1)
    return out


def _rank_and_write(prices, run_dir, out_suffix, label, frequency="mensual"):
    freq = FREQUENCIES[frequency]
    ranked = compute_momentum_table(prices, frequency)
    out_path = os.path.join(run_dir, f"top_momentum_{frequency}{out_suffix}.csv")

    if ranked.empty:
        logger.warning(
            f"Ningún ticker ({label}) tiene los {freq['min_periods']()} {freq['period_label']} "
            f"de historia mínima requeridos; no se generó ranking de momentum {frequency}."
        )
        ranked.to_csv(out_path, index=False)
        return ranked

    top = ranked.head(config.TOP_N_MOMENTUM).copy()
    top.to_csv(out_path, index=False)

    logger.info(
        f"Momentum {frequency} ({label}) calculado sobre {len(ranked)} tickers "
        f"(mínimo {freq['min_periods']()} {freq['period_label']} de historia)."
    )
    logger.info(f"Top {len(top)} por momentum {frequency} ({label}): {list(top['ticker'])}")

    return top


def main():
    prices = load_prices_usd()
    run_dir = config.get_run_dir()

    # Un ranking por cada análisis configurado (ver config.ANALYSES): el
    # universo completo y cualquier análisis complementario (p. ej. "solo
    # EEUU"). Agregar un análisis nuevo es agregar una entrada a esa lista.
    primary_result = None
    for spec in config.ANALYSES:
        cols = [t for t in prices.columns if t in spec.tickers]
        if len(cols) < spec.min_tickers:
            logger.warning(
                f"Solo {len(cols)} tickers disponibles para '{spec.label}' (mínimo requerido: "
                f"{spec.min_tickers}); se omite el ranking de momentum."
            )
            continue
        for frequency in FREQUENCIES:
            top = _rank_and_write(prices[cols], run_dir, spec.key, spec.label, frequency)
            if spec.key == "" and frequency == "mensual":
                primary_result = top

    return primary_result


if __name__ == "__main__":
    main()
