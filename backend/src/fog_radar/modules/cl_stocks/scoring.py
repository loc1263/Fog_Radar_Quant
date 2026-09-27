"""Scoring multi-factor del IPSA.

Port fiel de ``Stock_Choice_Nacional/WEB/backend/scoring_acciones.py``: mismo
input (``indicadores_acciones_*.csv``) → mismo ranking (test de paridad en
``tests/cl_stocks/test_parity.py``). Se conservan los nombres de columnas en
español del original porque son el formato de los CSV de datos.

Metodología (ver ``docs/METODOLOGIAS.md`` / CLAUDE.md del original):

- Pares holding/filial con estados financieros casi idénticos (Revenue y
  EBITDA) se marcan con ``Grupo Financiero Duplicado`` (no se eliminan).
- Ratios que solo tienen sentido positivos (P/E, EV/EBITDA, D/E...) excluyen
  valores <= 0.
- Dividendo: se descarta el yield si el Payout Ratio es > 100 % o no existe.
- Cada métrica se winsoriza a p5/p95 del universo y se estandariza (z-score,
  desvío poblacional). Las métricas "menor es mejor" se invierten.
- Score de pilar = promedio de los z-scores disponibles; Score Compuesto =
  promedio ponderado (``PESOS_PILARES``) de los pilares disponibles por fila.
"""

from __future__ import annotations

from collections.abc import Callable, Sequence
from dataclasses import dataclass

import numpy as np
import pandas as pd

from fog_radar.core.logging import get_logger

log = get_logger("cl.scoring")

WINSOR_PERCENTIL_INFERIOR = 0.05
WINSOR_PERCENTIL_SUPERIOR = 0.95


def _dividendo_sostenible(df: pd.DataFrame) -> pd.Series:
    """True si 0 < Payout Ratio <= 1 (dividendo pagado con utilidades)."""
    payout = pd.to_numeric(df.get("Payout Ratio"), errors="coerce")
    return payout.between(0, 1, inclusive="right")


@dataclass(frozen=True)
class Metric:
    columna: str
    pilar: str
    invertir: bool = False
    """True si "menor es mejor" (múltiplos, deuda): se invierte el z-score."""
    solo_positivos: bool = False
    """True si los valores <= 0 no tienen lectura económica y se excluyen."""
    filtro_valido: Callable[[pd.DataFrame], pd.Series] | None = None


PILARES: tuple[str, ...] = ("Valor", "Calidad", "Crecimiento", "Salud Financiera", "Dividendo", "Momentum")

METRICS: tuple[Metric, ...] = (
    # Valor: múltiplos de valoración (menor = más barato = mejor)
    Metric("Trailing P/E", "Valor", invertir=True, solo_positivos=True),
    Metric("Price/Book", "Valor", invertir=True, solo_positivos=True),
    Metric("Price/Sales", "Valor", invertir=True, solo_positivos=True),
    Metric("Enterprise Value/EBITDA", "Valor", invertir=True, solo_positivos=True),
    Metric("Enterprise Value/Revenue", "Valor", invertir=True, solo_positivos=True),
    # Calidad / rentabilidad
    Metric("Profit Margin", "Calidad"),
    Metric("Operating Margin (ttm)", "Calidad"),
    Metric("Return on Assets (ttm)", "Calidad"),
    Metric("Return on Equity (ttm)", "Calidad"),
    # Crecimiento
    Metric("Quarterly Revenue Growth (yoy)", "Crecimiento"),
    Metric("Quarterly Earnings Growth (yoy)", "Crecimiento"),
    # Salud financiera
    Metric("Total Debt/Equity (mrq)", "Salud Financiera", invertir=True, solo_positivos=True),
    Metric("Current Ratio (mrq)", "Salud Financiera"),
    # Dividendo (solo si es sostenible)
    Metric("Forward Annual Dividend Yield (%)", "Dividendo", filtro_valido=_dividendo_sostenible),
    # Momentum: retornos de precio
    Metric("Retorno 5D", "Momentum"),
    Metric("Retorno 1M", "Momentum"),
    Metric("Retorno 3M", "Momentum"),
    Metric("Retorno 6M", "Momentum"),
    Metric("Retorno YTD", "Momentum"),
    Metric("Retorno 1A", "Momentum"),
)

# Pesos del Score Compuesto (pedido del usuario: inclinar hacia Momentum).
# Es un parámetro de estilo de inversión, no un valor "correcto". El
# frontend los recibe por API como punto de partida de los sliders.
PESOS_PILARES: dict[str, float] = {
    "Valor": 0.5,
    "Calidad": 1.0,
    "Crecimiento": 1.0,
    "Salud Financiera": 1.0,
    "Dividendo": 0.25,
    "Momentum": 2.0,
}

# Columnas mínimas para poder puntuar un CSV de indicadores.
COLUMNAS_IDENTIDAD: tuple[str, ...] = ("Símbolo", "Nombre corto", "Sector", "Industria")

# Orden de columnas del ranking_acciones_*.csv (igual que el original).
COLUMNAS_SCORE_PILARES = tuple(f"Score {p}" for p in PILARES)
COLUMNAS_RANKING: tuple[str, ...] = (
    "Ranking",
    *COLUMNAS_IDENTIDAD,
    "Score Compuesto",
    *COLUMNAS_SCORE_PILARES,
    "Métricas Disponibles",
    "Métricas Totales",
    "Grupo Financiero Duplicado",
    *(m.columna for m in METRICS),
)


def _casi_iguales(a: Sequence[float], b: Sequence[float], tolerancia: float = 0.001) -> bool:
    """True si todos los valores de a y b coinciden dentro de una tolerancia relativa."""
    for x, y in zip(a, b, strict=True):
        if pd.isna(x) or pd.isna(y):
            return False
        base = max(abs(x), abs(y), 1.0)
        if abs(x - y) / base > tolerancia:
            return False
    return True


def detectar_duplicados_financieros(
    df: pd.DataFrame,
    columnas: Sequence[str] = ("Revenue (ttm)", "EBITDA"),
    tolerancia: float = 0.001,
) -> list[int | None]:
    """Id de grupo por fila para tickers con Revenue y EBITDA prácticamente
    idénticos (holding y su filial, p. ej. IAM / AGUAS-A); None si no tiene par."""
    n = len(df)
    grupo_id: list[int | None] = [None] * n
    if any(c not in df.columns for c in columnas):
        return grupo_id
    valores = df[list(columnas)].apply(pd.to_numeric, errors="coerce").to_numpy(dtype=float)
    siguiente = 1
    for i in range(n):
        for j in range(i + 1, n):
            if _casi_iguales(valores[i], valores[j], tolerancia):
                if grupo_id[i] is None and grupo_id[j] is None:
                    grupo_id[i] = grupo_id[j] = siguiente
                    siguiente += 1
                elif grupo_id[i] is not None:
                    grupo_id[j] = grupo_id[i]
                else:
                    grupo_id[i] = grupo_id[j]
    return grupo_id


def winsorizar(serie: pd.Series, q_inf: float, q_sup: float) -> pd.Series:
    """Recorta a los percentiles [q_inf, q_sup] (sin efecto con < 5 datos)."""
    validos = serie.dropna()
    if len(validos) < 5:
        return serie.copy()
    lo, hi = validos.quantile([q_inf, q_sup])
    return serie.clip(lower=lo, upper=hi)


def calcular_zscore(serie: pd.Series) -> pd.Series:
    """z-score con media y desvío poblacional (ddof=0) de los valores presentes."""
    validos = serie.dropna()
    if len(validos) < 2 or validos.std(ddof=0) == 0:
        return pd.Series(np.nan, index=serie.index)
    return (serie - validos.mean()) / validos.std(ddof=0)


def _serie_metrica(df: pd.DataFrame, m: Metric) -> pd.Series:
    serie = pd.to_numeric(df[m.columna], errors="coerce")
    if m.solo_positivos:
        serie = serie.where(serie > 0, other=np.nan)
    if m.filtro_valido is not None:
        try:
            mascara = m.filtro_valido(df).fillna(False).astype(bool)
            serie = serie.where(mascara, other=np.nan)
        except Exception as exc:
            log.warning("No se pudo aplicar filtro_valido a '%s': %s", m.columna, exc)
    return serie


def score_compuesto(valores_pilares: np.ndarray, pesos: np.ndarray) -> np.ndarray:
    """Promedio ponderado por fila ignorando pilares NaN (ni numerador ni denominador)."""
    validos = ~np.isnan(valores_pilares)
    pesos_validos = np.where(validos, np.tile(pesos, (len(valores_pilares), 1)), 0.0)
    suma_pesos = pesos_validos.sum(axis=1)
    numerador = (np.nan_to_num(valores_pilares, nan=0.0) * pesos_validos).sum(axis=1)
    out = np.full(len(valores_pilares), np.nan)
    np.divide(numerador, suma_pesos, out=out, where=suma_pesos > 0)
    return out


def calcular_scores(df: pd.DataFrame, pesos: dict[str, float] | None = None) -> pd.DataFrame:
    """Agrega z-scores, scores por pilar, Score Compuesto y Ranking; ordena por score."""
    pesos = pesos or PESOS_PILARES
    df = df.copy()
    df["Grupo Financiero Duplicado"] = detectar_duplicados_financieros(df)

    z_por_pilar: dict[str, list[str]] = {}
    z_todas: list[str] = []
    for m in METRICS:
        if m.columna not in df.columns:
            continue
        z = calcular_zscore(
            winsorizar(_serie_metrica(df, m), WINSOR_PERCENTIL_INFERIOR, WINSOR_PERCENTIL_SUPERIOR)
        )
        col_z = f"z_{m.columna}"
        df[col_z] = -z if m.invertir else z
        z_por_pilar.setdefault(m.pilar, []).append(col_z)
        z_todas.append(col_z)

    for pilar, cols in z_por_pilar.items():
        df[f"Score {pilar}"] = df[cols].mean(axis=1, skipna=True)

    # Transparencia: cuántas métricas aportaron dato a cada fila.
    df["Métricas Disponibles"] = df[z_todas].notna().sum(axis=1)
    df["Métricas Totales"] = len(z_todas)

    cols_pilares = [f"Score {p}" for p in z_por_pilar]
    w = np.array([pesos.get(p, 1.0) for p in z_por_pilar])
    df["Score Compuesto"] = score_compuesto(df[cols_pilares].to_numpy(dtype=float), w)
    df["Ranking"] = df["Score Compuesto"].rank(ascending=False, method="min")
    return df.sort_values("Score Compuesto", ascending=False, na_position="last")


def construir_ranking(indicadores: pd.DataFrame, pesos: dict[str, float] | None = None) -> pd.DataFrame:
    """Tabla final ``ranking_acciones_*.csv``: columnas del original, redondeo a 4 decimales."""
    faltantes = [c for c in COLUMNAS_IDENTIDAD if c not in indicadores.columns]
    if faltantes:
        raise ValueError(f"Faltan columnas en el CSV de indicadores: {', '.join(faltantes)}")
    scored = calcular_scores(indicadores, pesos)
    columnas = [c for c in COLUMNAS_RANKING if c in scored.columns]
    return scored[columnas].round(4).reset_index(drop=True)


def grupos_duplicados(ranking: pd.DataFrame) -> dict[int, list[str]]:
    """{grupo: [símbolos]} a partir de la columna ``Grupo Financiero Duplicado``."""
    dup = ranking[ranking["Grupo Financiero Duplicado"].notna()]
    return {int(g): sub["Símbolo"].tolist() for g, sub in dup.groupby("Grupo Financiero Duplicado")}
