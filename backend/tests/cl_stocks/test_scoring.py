"""Tests unitarios del scoring IPSA (el original no tenía tests)."""

from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.cl_stocks.scoring import (
    METRICS,
    PESOS_PILARES,
    calcular_scores,
    calcular_zscore,
    construir_ranking,
    detectar_duplicados_financieros,
    grupos_duplicados,
    score_compuesto,
    winsorizar,
)


def _universo(n: int = 6, **overrides: list) -> pd.DataFrame:
    base = {
        "Símbolo": [f"T{i}.SN" for i in range(n)],
        "Nombre corto": [f"Empresa {i}" for i in range(n)],
        "Sector": ["Financial Services"] * n,
        "Industria": ["Banks"] * n,
        "Revenue (ttm)": [100.0 * (i + 1) for i in range(n)],
        "EBITDA": [10.0 * (i + 1) for i in range(n)],
    }
    base.update(overrides)
    return pd.DataFrame(base)


# --- winsorizar -------------------------------------------------------------


def test_winsorizar_recorta_a_percentiles() -> None:
    s = pd.Series([1, 2, 3, 4, 5, 6, 7, 8, 9, 1000], dtype=float)
    w = winsorizar(s, 0.05, 0.95)
    lo, hi = s.quantile([0.05, 0.95])
    assert w.min() == pytest.approx(lo) and w.max() == pytest.approx(hi)
    assert w.max() < 1000


def test_winsorizar_no_toca_con_menos_de_5_datos() -> None:
    s = pd.Series([1.0, 2.0, 1000.0, np.nan, np.nan])
    pd.testing.assert_series_equal(winsorizar(s, 0.05, 0.95), s)


def test_winsorizar_conserva_nan() -> None:
    s = pd.Series([1.0, 2.0, 3.0, 4.0, 5.0, np.nan])
    assert np.isnan(winsorizar(s, 0.05, 0.95).iloc[-1])


# --- z-score ----------------------------------------------------------------


def test_zscore_usa_desvio_poblacional() -> None:
    s = pd.Series([1.0, 2.0, 3.0, np.nan])
    z = calcular_zscore(s)
    sigma = np.std([1.0, 2.0, 3.0])  # ddof=0
    assert z.iloc[0] == pytest.approx(-1 / sigma)
    assert z.iloc[1] == pytest.approx(0.0)
    assert np.isnan(z.iloc[3])


@pytest.mark.parametrize("valores", [[5.0, 5.0, 5.0], [1.0, np.nan, np.nan]])
def test_zscore_nan_si_no_hay_dispersion(valores: list[float]) -> None:
    assert calcular_zscore(pd.Series(valores)).isna().all()


# --- duplicados holding/filial ---------------------------------------------


def test_detecta_holding_y_filial() -> None:
    df = _universo(
        4,
        **{"Revenue (ttm)": [100.0, 500.0, 100.05, 300.0], "EBITDA": [10.0, 50.0, 10.0, 30.0]},
    )
    assert detectar_duplicados_financieros(df) == [1, None, 1, None]


def test_duplicados_ignora_nan_y_columnas_faltantes() -> None:
    df = _universo(2, **{"Revenue (ttm)": [np.nan, np.nan], "EBITDA": [1.0, 1.0]})
    assert detectar_duplicados_financieros(df) == [None, None]
    assert detectar_duplicados_financieros(df.drop(columns=["EBITDA"])) == [None, None]


def test_grupos_transitivos() -> None:
    df = _universo(3, **{"Revenue (ttm)": [100.0] * 3, "EBITDA": [10.0] * 3})
    assert detectar_duplicados_financieros(df) == [1, 1, 1]
    ranking = construir_ranking(df)
    grupos = grupos_duplicados(ranking)
    assert list(grupos) == [1] and sorted(grupos[1]) == ["T0.SN", "T1.SN", "T2.SN"]


# --- filtros de métricas ----------------------------------------------------


def test_dividendo_se_descarta_si_payout_mayor_a_100() -> None:
    df = _universo(
        6,
        **{
            "Forward Annual Dividend Yield (%)": [3.0, 4.0, 5.0, 6.0, 25.0, 7.0],
            "Payout Ratio": [0.5, 0.6, 0.7, 0.8, 1.9, np.nan],
        },
    )
    out = calcular_scores(df).set_index("Símbolo")
    assert np.isnan(out.loc["T4.SN", "z_Forward Annual Dividend Yield (%)"])  # payout 190 %
    assert np.isnan(out.loc["T5.SN", "z_Forward Annual Dividend Yield (%)"])  # sin payout
    assert out.loc["T3.SN", "Score Dividendo"] > out.loc["T0.SN", "Score Dividendo"]


def test_multiplos_no_positivos_se_excluyen_y_se_invierten() -> None:
    df = _universo(6, **{"Trailing P/E": [5.0, 10.0, 15.0, 20.0, -8.0, 0.0]})
    out = calcular_scores(df).set_index("Símbolo")
    assert out.loc[["T4.SN", "T5.SN"], "z_Trailing P/E"].isna().all()
    # menor P/E = más barato = mayor score
    assert out.loc["T0.SN", "z_Trailing P/E"] > out.loc["T3.SN", "z_Trailing P/E"]


# --- score compuesto y ranking ----------------------------------------------


def test_score_compuesto_ignora_pilares_faltantes() -> None:
    valores = np.array([[1.0, np.nan], [np.nan, np.nan], [1.0, -1.0]])
    out = score_compuesto(valores, np.array([1.0, 3.0]))
    assert out[0] == pytest.approx(1.0)  # no se trata el NaN como 0
    assert np.isnan(out[1])
    assert out[2] == pytest.approx((1 - 3) / 4)


def test_ranking_empates_comparten_puesto() -> None:
    df = _universo(5, **{"Retorno 1M": [0.1, 0.1, 0.05, -0.1, np.nan]})
    r = construir_ranking(df)
    assert r["Ranking"].tolist()[:4] == [1.0, 1.0, 3.0, 4.0]
    assert np.isnan(r["Ranking"].iloc[-1]) and np.isnan(r["Score Compuesto"].iloc[-1])


def test_metricas_disponibles_y_totales() -> None:
    df = _universo(5, **{"Retorno 1M": [0.1, 0.2, 0.3, 0.4, np.nan], "Retorno 3M": [0.1, 0.2, 0.3, 0.4, 0.5]})
    r = construir_ranking(df).set_index("Símbolo")
    assert (r["Métricas Totales"] == 2).all()
    assert r.loc["T4.SN", "Métricas Disponibles"] == 1


def test_pesos_personalizados_cambian_el_orden() -> None:
    df = _universo(
        5,
        **{
            "Retorno 1M": [0.5, 0.4, 0.3, 0.2, 0.1],  # T0 gana en momentum
            "Profit Margin": [0.01, 0.02, 0.03, 0.04, 0.5],  # T4 gana en calidad
        },
    )
    assert construir_ranking(df)["Símbolo"].iloc[0] == "T0.SN"
    solo_calidad = dict.fromkeys(PESOS_PILARES, 0.0) | {"Calidad": 1.0, "Momentum": 0.01}
    assert construir_ranking(df, solo_calidad)["Símbolo"].iloc[0] == "T4.SN"


def test_faltan_columnas_de_identidad() -> None:
    with pytest.raises(ValueError, match="Sector"):
        construir_ranking(_universo(3).drop(columns=["Sector"]))


def test_configuracion_de_pesos_por_defecto() -> None:
    # Decisión del usuario documentada en el original: inclinar hacia Momentum.
    assert PESOS_PILARES == {
        "Valor": 0.5,
        "Calidad": 1.0,
        "Crecimiento": 1.0,
        "Salud Financiera": 1.0,
        "Dividendo": 0.25,
        "Momentum": 2.0,
    }
    assert len(METRICS) == 20
