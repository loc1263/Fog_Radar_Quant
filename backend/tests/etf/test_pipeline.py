"""Pipeline batch del módulo ETF (port de ``actualizar_datos_semanal.py``), sin Yahoo:
el screener se reemplaza por un doble que devuelve el CSV real del 24-09."""

from __future__ import annotations

import os
import time
from datetime import datetime, timedelta
from pathlib import Path

import pytest

from fog_radar.core.jobs import JobRegistry
from fog_radar.jobs import __main__ as cli
from fog_radar.modules.etf import pipeline, storage
from fog_radar.settings import Settings

from .conftest import FIXTURES, copiar_fixture, expected, leer_bytes


@pytest.fixture
def screener_falso(settings: Settings, monkeypatch: pytest.MonkeyPatch, etf_hist: Path) -> Path:
    """El screener "genera" etf_perf_data_20260924_230338.csv en data/etf/screener."""
    d = storage.screener_dir(settings)
    d.mkdir(parents=True)
    copiar_fixture(FIXTURES / "input" / "etf_perf_data_20260924.csv.gz", d)
    generado = d / "etf_perf_data_20260924_230338.csv"
    (d / "etf_perf_data_20260924.csv").rename(generado)
    monkeypatch.setattr(pipeline, "correr_screener", lambda _s: generado)
    return generado


def test_primera_corrida_instala_sin_archivar(settings: Settings, screener_falso: Path) -> None:
    resumen = pipeline.run(settings)
    current = storage.current_dir(settings)
    assert "corrida 20260924" in resumen
    assert "683/1189 ETFs depurados" in resumen
    assert "top 3: CRAK" in resumen
    assert (current / "etf_perf_data_20260924.csv").is_file()
    for nombre in (
        "etf_top15_recomendados.csv",
        "etf_russell3000_portfolio.csv",
        "etf_categorias_ranking.csv",
    ):
        assert (current / nombre).read_bytes() == leer_bytes(expected(nombre)), nombre
    # Sin corrida vigente previa no hay nada que archivar; no quedan staging.
    assert not list(storage.hist_dir(settings).glob("*20260924*"))
    assert not list(storage.module_dir(settings).glob(".staging_*"))


def test_segunda_corrida_archiva_la_vigente(settings: Settings, screener_falso: Path) -> None:
    pipeline.run(settings)
    pipeline.run(settings)
    hist = storage.hist_dir(settings)
    assert (hist / "etf_perf_data_20260924.csv").is_file()
    assert (hist / "etf_top15_recomendados_20260924.csv").is_file()
    pipeline.run(settings)  # misma fecha otra vez → contador de secuencia
    assert (hist / "etf_top15_recomendados_20260924_2.csv").is_file()
    # El historial no duplica la fecha.
    historial = (storage.current_dir(settings) / storage.HISTORIAL).read_text().splitlines()
    assert sum(1 for linea in historial if linea.startswith("20260924,")) == 1


def test_analisis_fallido_no_toca_la_corrida_vigente(
    settings: Settings, etf_corrida: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    antes = {p.name: p.read_bytes() for p in etf_corrida.glob("*.csv")}

    def _falla(*_a: object, **_k: object) -> dict:
        raise KeyError("Sharpe Ratio")

    monkeypatch.setattr(storage, "correr_analisis", _falla)
    with pytest.raises(KeyError):
        pipeline.run(settings, solo_analisis=True)
    despues = {p.name: p.read_bytes() for p in etf_corrida.glob("*.csv")}
    assert despues == antes
    assert not list(storage.module_dir(settings).glob(".staging_*"))


def test_solo_analisis_reproduce_los_outputs(settings: Settings, etf_corrida: Path) -> None:
    (etf_corrida / storage.TOP15).unlink()
    resumen = pipeline.run(settings, solo_analisis=True)
    assert resumen.startswith("solo análisis")
    assert (etf_corrida / storage.TOP15).read_bytes() == leer_bytes(expected(storage.TOP15))


def test_respaldo_de_outputs_viejos_del_screener(tmp_path: Path) -> None:
    viejo = (datetime.now() - timedelta(days=40)).strftime("%Y%m%d_%H%M%S")
    nuevo = (datetime.now() - timedelta(days=2)).strftime("%Y%m%d_%H%M%S")
    for nombre in (
        f"etf_perf_data_{viejo}.csv",
        f"etf_choice_{viejo}.xlsx",
        f"incidencias_{viejo}.csv",
        f"etf_perf_data_{nuevo}.csv",
        "List_All_ETF.csv",
    ):
        (tmp_path / nombre).write_text("x")
    movidos = pipeline.respaldar_outputs_viejos_screener(tmp_path)
    assert sorted(p.name for p in movidos) == sorted(
        [f"etf_perf_data_{viejo}.csv", f"etf_choice_{viejo}.xlsx", f"incidencias_{viejo}.csv"]
    )
    assert (tmp_path / f"etf_perf_data_{nuevo}.csv").is_file()
    assert (tmp_path / "List_All_ETF.csv").is_file()
    assert pipeline.respaldar_outputs_viejos_screener(tmp_path, dias=0) == []


def test_cli_solo_analisis_registra_el_job(settings: Settings, etf_corrida: Path) -> None:
    from fog_radar import settings as settings_mod

    settings_mod.get_settings.cache_clear()
    os.environ["DATA_DIR"] = str(settings.data_dir)
    os.environ["ENV"] = "test"
    try:
        assert cli.main(["etf", "--solo-analisis"]) == 0
        assert cli.main(["etf", "--solo-scoring"]) == 2  # opción de otro módulo
    finally:
        del os.environ["DATA_DIR"], os.environ["ENV"]
        settings_mod.get_settings.cache_clear()
    rec = JobRegistry(settings.jobs_dir).latest("etf")
    assert rec is not None and rec.status == "success"
    assert rec.message and "solo análisis" in rec.message


def test_mtime_no_importa_para_elegir_el_csv(tmp_path: Path) -> None:
    """La corrida vigente es la de fecha más nueva en el nombre, no la modificada último."""
    a, b = tmp_path / "etf_perf_data_20260922.csv", tmp_path / "etf_perf_data_20260924.csv"
    b.write_text("x")
    time.sleep(0.01)
    a.write_text("x")
    assert storage.csv_actual(tmp_path) == b
