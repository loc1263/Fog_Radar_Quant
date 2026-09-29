import json
import os

import pandas as pd
import pytest

from fog_radar.modules.tendencia.reporting.run_data import compute_sector_summary, load_analysis


def test_compute_sector_summary_aggregates_correctly():
    merged = pd.DataFrame(
        {
            "peso_final": [0.30, 0.20, 0.15, 0.35],
            "retorno_esperado_6m_pct": [10.0, 8.0, -2.0, 6.0],
            "senal_tecnica": ["Alcista", "Alcista", "Bajista", "Lateral / mixta"],
            "sector": ["Tecnología", "Tecnología", "Energía", "Financiero"],
        },
        index=["AAA", "BBB", "CCC", "DDD"],
    )

    summary = compute_sector_summary(merged)

    assert set(summary.index) == {"Tecnología", "Energía", "Financiero"}
    tech = summary.loc["Tecnología"]
    assert tech["n_acciones"] == 2
    assert tech["peso_total"] == pytest.approx(0.50)
    assert tech["retorno_6m_prom"] == pytest.approx(9.0)
    assert tech["senal_predominante"] == "Alcista"
    assert set(tech["tickers"].split(", ")) == {"AAA", "BBB"}

    assert summary.index[0] == "Tecnología"


def test_load_analysis_returns_none_when_suffixed_file_missing(tmp_path):
    # Sin portfolio_performance_us.json en la carpeta: el análisis "solo
    # EEUU" se omitió ese día (universo insuficiente) y load_analysis debe
    # devolver None en vez de fallar, para que el llamador la omita.
    assert load_analysis(str(tmp_path), suffix="_us") is None


def test_load_analysis_loads_suffixed_analysis(tmp_path):
    run_dir = str(tmp_path)
    perf = {
        "universo_completo": {"retorno_esperado_anual": 0.10, "volatilidad_anual": 0.12, "sharpe_ratio": 1.1},
        "portafolio_15_finales": {
            "retorno_esperado_anual": 0.09,
            "volatilidad_anual": 0.11,
            "sharpe_ratio": 1.2,
        },
        "tasa_libre_riesgo_anual": 0.03,
        "tickers_seleccionados": ["AAA"],
    }
    with open(os.path.join(run_dir, "portfolio_performance_us.json"), "w", encoding="utf-8") as f:
        json.dump(perf, f)

    pd.DataFrame(
        {"peso_final": [1.0], "nombre": ["Empresa A"], "pais": ["EEUU"]},
        index=["AAA"],
    ).to_csv(os.path.join(run_dir, "portfolio_final_weights_us.csv"))
    pd.DataFrame(
        {
            "ticker": ["AAA"],
            "retorno_esperado_6m_pct": [5.0],
            "senal_tecnica": ["Alcista"],
            "precio_actual_usd": [10.0],
        }
    ).to_csv(os.path.join(run_dir, "trend_forecast_6m_us.csv"), index=False)

    result = load_analysis(run_dir, suffix="_us")

    assert result is not None
    assert result.perf["tickers_seleccionados"] == ["AAA"]
    assert list(result.merged.index) == ["AAA"]
    # Archivos opcionales que no existen en este test -> vacíos, sin fallar.
    assert result.momentum.empty
    assert result.momentum_weekly.empty
    assert result.reduced_portfolios == []
