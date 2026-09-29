"""Screener de ETFs: piezas puras (sin red). Port de ``Screener_Data_ETF_Script``."""

from __future__ import annotations

from pathlib import Path

import numpy as np
import pandas as pd
import pytest

from fog_radar.modules.etf.screener import main as screener_main
from fog_radar.modules.etf.screener.analyzers.performance import PerformanceAnalyzer
from fog_radar.modules.etf.screener.analyzers.risk_metrics import RiskAnalyzer
from fog_radar.modules.etf.screener.config import INPUT_CSV, NA_VALUE
from fog_radar.modules.etf.screener.etf_discovery.discovery import AutoETFDiscovery
from fog_radar.modules.etf.screener.file_handlers.reader import CSVReader
from fog_radar.modules.etf.screener.file_handlers.writer import CSVWriter
from fog_radar.modules.etf.screener.models import ETFData, ReturnsData, RiskMetrics
from fog_radar.modules.etf.screener.utils import incidencias


def _hist(precios: list[float]) -> pd.DataFrame:
    return pd.DataFrame({"Close": precios})


def test_lista_curada_de_tickers() -> None:
    tickers = CSVReader.read_tickers_from_csv(str(INPUT_CSV))
    meta = CSVReader.read_ticker_metadata(str(INPUT_CSV))
    assert len(tickers) > 1000
    assert set(tickers) <= set(meta)
    assert meta[tickers[0]]["Company"]


def test_performance_y_riesgo() -> None:
    assert PerformanceAnalyzer._calc_performance(110, _hist([100, 105]), 0, None) == pytest.approx(10.0)
    assert PerformanceAnalyzer._calc_performance(110, pd.DataFrame(), 0, None) is None
    assert PerformanceAnalyzer.format_performance(None) == NA_VALUE
    assert PerformanceAnalyzer.format_performance(12.3456) == 12.35

    rng = np.random.default_rng(0)
    mercado = _hist(list(100 * np.cumprod(1 + rng.normal(0, 0.01, 252))))
    etf = _hist(list(mercado["Close"] * 1.0))
    assert RiskAnalyzer.calculate_beta(etf, mercado) == pytest.approx(1.0)
    assert RiskAnalyzer.calculate_sharpe_ratio(_hist([100.0])) is None
    assert RiskAnalyzer.calculate_treynor_ratio(etf, mercado) is not None


def test_resultado_prioriza_metadata_local() -> None:
    d = AutoETFDiscovery()
    d.metadata_local = {
        "VOO": {"Name": "Vanguard S&P 500 ETF", "Company": "vanguard", "Category": "Large Blend"}
    }
    info = {"quoteType": "ETF", "longName": "Vanguard S&P 500 ETF", "volume": 1, "totalAssets": 2}
    r = d._build_result_dict(
        ETFData("VOO", info, 500.0, pd.DataFrame()),
        ReturnsData(1, 2, 3, 4, 5, 6),
        RiskMetrics(1.0, 0.5, 1.0),
    )
    assert r["Proveedor"] == "Vanguard"  # Company normalizada
    assert r["Category"] == "Large Blend"
    assert r["Performance 5D (%)"] == 1


def test_seleccion_etf_choice(tmp_path: Path) -> None:
    df = pd.DataFrame(
        {
            "Ticker": ["A", "B", "C"],
            "TipoInstrumento": ["ETF", "ETF", "EQUITY"],
            "Performance 5D (%)": [1, 5, 1],
            "Performance 1M (%)": [2, 1, 2],
            "Performance 6M (%)": [3, 3, 3],
            "Performance 1Y (%)": [4, 4, 4],
            "Performance 5Y (%)": [9, 9, 9],
        }
    )
    csv = tmp_path / "etf_perf_data_x.csv"
    df.to_csv(csv, index=False)
    r = CSVWriter.save_top_etfs_by_growth(str(csv), "x", str(tmp_path))
    assert r["found_count"] == 1  # solo A: ETF, creciente y no negativo
    assert pd.read_csv(r["csv"], sep=";")["Ticker"].tolist() == ["A"]


def test_procesar_etfs_escribe_en_output_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    lista = tmp_path / "List_All_ETF.csv"
    lista.write_text("Ticker;Company;Name;Category;Update\nAAA;Vanguard;Fondo A;Large Blend;20260101\n")
    fila = {"Ticker": "AAA", "TipoInstrumento": "ETF", "Nombre": "Fondo A", "Proveedor": "Vanguard"}

    def _validate_all(self: AutoETFDiscovery, tickers: list[str]) -> pd.DataFrame:
        incidencias.registrar("LENTO", "AAA", "prueba")
        return pd.DataFrame([fila])

    monkeypatch.setattr(AutoETFDiscovery, "validate_all", _validate_all)
    salida = tmp_path / "out"
    csv = screener_main.procesar_etfs(salida, lista)
    assert csv is not None and csv.parent == salida
    assert csv.name.startswith("etf_perf_data_")
    assert pd.read_csv(csv, encoding="utf-8-sig")["Ticker"].tolist() == ["AAA"]
    assert list(salida.glob("incidencias_*.csv"))


def test_procesar_etfs_sin_validos(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(AutoETFDiscovery, "validate_all", lambda self, t: pd.DataFrame())
    lista = tmp_path / "l.csv"
    lista.write_text("Ticker;Company;Name\nAAA;X;Y\n")
    assert screener_main.procesar_etfs(tmp_path / "out", lista) is None
