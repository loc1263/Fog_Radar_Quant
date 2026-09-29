import json
import os

import numpy as np
import pandas as pd

from fog_radar.modules.tendencia.config import AnalysisSpec
from fog_radar.modules.tendencia.reporting.dashboard_data import (
    _records,
    build_analysis_payload,
    build_dashboard_payload,
)
from fog_radar.modules.tendencia.reporting.run_data import AnalysisData


def test_records_converts_nan_to_none_and_resets_index():
    df = pd.DataFrame({"peso_final": [0.5, np.nan]}, index=["AAA", "BBB"])

    records = _records(df, index_as="ticker")

    assert records == [
        {"ticker": "AAA", "peso_final": 0.5},
        {"ticker": "BBB", "peso_final": None},
    ]


def test_build_analysis_payload_shapes_holdings_sectors_and_momentum():
    spec = AnalysisSpec(key="", label="Asia + EEUU", tickers={"AAA": (), "BBB": ()}, top_n=2)
    perf = {
        "tickers_seleccionados": ["AAA", "BBB"],
        "portafolio_15_finales": {
            "retorno_esperado_anual": 0.1,
            "volatilidad_anual": 0.1,
            "sharpe_ratio": 1.0,
        },
    }
    merged = pd.DataFrame(
        {
            "peso_final": [0.6, 0.4],
            "nombre": ["Empresa A", "Empresa B"],
            "pais": ["EEUU", "Japón"],
            "sector": ["Technology", "Financial Services"],
            "retorno_esperado_6m_pct": [10.0, -5.0],
            "senal_tecnica": ["Alcista", "Bajista"],
        },
        index=["AAA", "BBB"],
    )
    momentum = pd.DataFrame({"rank": [1], "ticker": ["AAA"], "momentum_score_pct": [12.0]})

    data = AnalysisData(perf=perf, merged=merged, notes=[], momentum=momentum)

    payload = build_analysis_payload(spec, data)

    assert payload["key"] == ""
    assert payload["label"] == "Asia + EEUU"
    assert payload["universe_size"] == 2
    assert payload["n_selected"] == 2
    assert [h["ticker"] for h in payload["holdings"]] == ["AAA", "BBB"]
    assert {s["sector"] for s in payload["sectors"]} == {"Technology", "Financial Services"}
    assert payload["momentum"][0]["ticker"] == "AAA"
    assert payload["momentum_semanal"] == []
    assert payload["portfolio_options"] == []


def test_build_dashboard_payload_includes_only_available_analyses(tmp_path):
    run_dir = str(tmp_path)
    perf = {
        "universo_completo": {"retorno_esperado_anual": 0.1, "volatilidad_anual": 0.1, "sharpe_ratio": 1.0},
        "portafolio_15_finales": {
            "retorno_esperado_anual": 0.1,
            "volatilidad_anual": 0.1,
            "sharpe_ratio": 1.0,
        },
        "tasa_libre_riesgo_anual": 0.03,
        "tickers_seleccionados": ["AAA"],
    }
    with open(os.path.join(run_dir, "portfolio_performance.json"), "w", encoding="utf-8") as f:
        json.dump(perf, f)
    pd.DataFrame(
        {"peso_final": [1.0], "nombre": ["Empresa A"], "pais": ["EEUU"]},
        index=["AAA"],
    ).to_csv(os.path.join(run_dir, "portfolio_final_weights.csv"))
    pd.DataFrame(
        {
            "ticker": ["AAA"],
            "retorno_esperado_6m_pct": [5.0],
            "senal_tecnica": ["Alcista"],
        }
    ).to_csv(os.path.join(run_dir, "trend_forecast_6m.csv"), index=False)
    with open(os.path.join(run_dir, "market_regime_notes.txt"), "w", encoding="utf-8") as f:
        f.write("VIX en 15: normal.")

    # Sin portfolio_performance_us.json en esta carpeta temporal: el
    # análisis "solo EEUU" no está disponible y debe quedar afuera.
    payload = build_dashboard_payload(run_dir)

    assert payload["run_dir"] == os.path.basename(run_dir)
    assert len(payload["analyses"]) == 1
    assert payload["analyses"][0]["key"] == ""
    assert payload["market_regime_notes"] == ["VIX en 15: normal."]


def test_build_dashboard_payload_includes_reduced_portfolio_options(tmp_path):
    run_dir = str(tmp_path)
    perf = {
        "universo_completo": {"retorno_esperado_anual": 0.1, "volatilidad_anual": 0.1, "sharpe_ratio": 1.0},
        "portafolio_15_finales": {
            "retorno_esperado_anual": 0.1,
            "volatilidad_anual": 0.1,
            "sharpe_ratio": 1.0,
        },
        "tasa_libre_riesgo_anual": 0.03,
        "tickers_seleccionados": ["AAA", "BBB"],
    }
    with open(os.path.join(run_dir, "portfolio_performance.json"), "w", encoding="utf-8") as f:
        json.dump(perf, f)
    pd.DataFrame(
        {"peso_final": [0.6, 0.4], "nombre": ["A", "B"], "pais": ["EEUU", "EEUU"]},
        index=["AAA", "BBB"],
    ).to_csv(os.path.join(run_dir, "portfolio_final_weights.csv"))
    pd.DataFrame(
        {
            "ticker": ["AAA", "BBB"],
            "retorno_esperado_6m_pct": [5.0, 3.0],
            "senal_tecnica": ["Alcista", "Bajista"],
        }
    ).to_csv(os.path.join(run_dir, "trend_forecast_6m.csv"), index=False)
    # Portafolio reducido de 5 (con 1 sola acción para simplificar el fixture).
    pd.DataFrame(
        {"peso_final": [1.0], "nombre": ["A"], "pais": ["EEUU"]},
        index=["AAA"],
    ).to_csv(os.path.join(run_dir, "portfolio_final_weights_top5.csv"))
    with open(os.path.join(run_dir, "portfolio_performance_top5.json"), "w", encoding="utf-8") as f:
        json.dump(
            {
                "n_acciones": 5,
                "retorno_esperado_anual": 0.2,
                "volatilidad_anual": 0.15,
                "sharpe_ratio": 1.1,
                "tickers_seleccionados": ["AAA"],
            },
            f,
        )

    payload = build_dashboard_payload(run_dir)

    options = payload["analyses"][0]["portfolio_options"]
    assert [o["n"] for o in options] == [5]  # sin archivos de top10 -> no aparece
    holding = options[0]["holdings"][0]
    assert holding["ticker"] == "AAA"
    assert holding["peso_final"] == 1.0
    assert holding["senal_tecnica"] == "Alcista"  # tomado del forecast del portafolio de 15
    assert options[0]["performance"]["sharpe_ratio"] == 1.1
