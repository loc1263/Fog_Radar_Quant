import json
import os
from datetime import datetime

from fog_radar.modules.tendencia import config as settings


def test_fallback_universe_loads_when_no_snapshot(fresh_settings):
    assert len(fresh_settings.ASIA_TICKERS) > 0
    assert len(fresh_settings.US_TICKERS) > 0
    assert fresh_settings.ALL_TICKERS == {**fresh_settings.ASIA_TICKERS, **fresh_settings.US_TICKERS}


def test_load_universe_reads_todays_snapshot(tmp_path, monkeypatch):
    universe_dir = tmp_path / "data" / "universe"
    universe_dir.mkdir(parents=True)

    from datetime import datetime

    today = datetime.now().strftime("%Y%m%d")
    snapshot = {
        "discovered_at": "2026-08-11T00:00:00",
        "method": "dynamic_yfinance_screener",
        "asia_tickers": {"9999.T": ["Test Co", "Japón", "JPY"]},
        "us_tickers": {"TEST": ["Test Inc", "EEUU", "USD"]},
        "sector_map": {"9999.T": "Tecnología", "TEST": "Tecnología"},
    }
    (universe_dir / f"universe_{today}.json").write_text(json.dumps(snapshot), encoding="utf-8")

    monkeypatch.setattr(settings, "UNIVERSE_DIR", str(universe_dir))
    asia, us, sector = settings._load_universe()

    assert asia == {"9999.T": ("Test Co", "Japón", "JPY")}
    assert us == {"TEST": ("Test Inc", "EEUU", "USD")}
    assert settings.UNIVERSE_SOURCE == "dynamic_yfinance_screener"


def test_load_universe_falls_back_on_corrupt_snapshot(tmp_path, monkeypatch, capsys):
    universe_dir = tmp_path / "data" / "universe"
    universe_dir.mkdir(parents=True)
    from datetime import datetime

    today = datetime.now().strftime("%Y%m%d")
    (universe_dir / f"universe_{today}.json").write_text("{not valid json", encoding="utf-8")

    monkeypatch.setattr(settings, "UNIVERSE_DIR", str(universe_dir))
    asia, us, sector = settings._load_universe()

    assert asia == settings.FALLBACK_ASIA_TICKERS
    assert us == settings.FALLBACK_US_TICKERS


def test_get_run_dir_reuses_todays_folder_by_default(tmp_path, monkeypatch):
    monkeypatch.setattr(settings, "OUTPUT_DIR", str(tmp_path))

    first = settings.get_run_dir()
    second = settings.get_run_dir()

    assert first == second
    assert os.path.isdir(first)
    assert os.path.exists(os.path.join(first, "run_manifest.json"))


def test_get_run_dir_new_forces_next_sequence(tmp_path, monkeypatch):
    monkeypatch.setattr(settings, "OUTPUT_DIR", str(tmp_path))

    first = settings.get_run_dir()
    forced = settings.get_run_dir(new=True)
    reused = settings.get_run_dir()

    today = os.path.basename(first).split("_")[0]
    assert os.path.basename(first) == f"{today}_01"
    assert os.path.basename(forced) == f"{today}_02"
    assert reused == forced


def test_get_run_dir_ignores_non_matching_entries(tmp_path, monkeypatch):
    monkeypatch.setattr(settings, "OUTPUT_DIR", str(tmp_path))
    (tmp_path / "not_a_run_dir").mkdir()
    (tmp_path / "20260101_01").mkdir()  # otro día, no debería contar para "hoy"

    run_dir = settings.get_run_dir()

    today = datetime.now().strftime("%Y%m%d")
    assert os.path.basename(run_dir) == f"{today}_01"


def test_analyses_registry_has_a_primary_entry_using_all_tickers():
    # El análisis principal (key="") debe existir siempre, apuntar al
    # universo completo, y ser el primero de la lista — modeling/* y
    # reporting/* asumen esa convención (ver settings.ANALYSES).
    primary = settings.ANALYSES[0]
    assert primary.key == ""
    assert primary.tickers is settings.ALL_TICKERS
    assert primary.min_tickers == 1


def test_analyses_registry_secondary_entries_have_non_empty_key():
    # Cualquier análisis complementario necesita un sufijo de archivo
    # distinto de "" para no pisar los archivos del análisis principal.
    for spec in settings.ANALYSES[1:]:
        assert spec.key
        assert spec.tickers is not settings.ALL_TICKERS
