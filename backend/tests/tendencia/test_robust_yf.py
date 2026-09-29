import pandas as pd

from fog_radar.modules.tendencia.net import robust_yf


def test_cache_path_sanitizes_special_characters(tmp_path):
    path = robust_yf.cache_path(str(tmp_path), "^VIX")
    assert path.endswith("_VIX.csv")
    path2 = robust_yf.cache_path(str(tmp_path), "USDKRW=X")
    assert path2.endswith("USDKRW_X.csv")


def test_is_cache_fresh_false_when_missing(tmp_path):
    assert robust_yf._is_cache_fresh(str(tmp_path / "nope.csv")) is False


def test_is_cache_fresh_true_for_recent_file(tmp_path):
    f = tmp_path / "AAPL.csv"
    f.write_text("Date,Close\n2026-01-01,100\n")
    assert robust_yf._is_cache_fresh(str(f), max_age_hours=20) is True


def test_download_history_uses_fresh_cache_without_network_call(tmp_path, monkeypatch):
    cache_file = tmp_path / "AAPL.csv"
    dates = pd.date_range("2026-01-02", periods=3, freq="W-FRI")
    df = pd.DataFrame({"Close": [1.0, 2.0, 3.0]}, index=dates)
    df.index.name = "Date"
    df.to_csv(cache_file)

    monkeypatch.setattr(robust_yf, "_sleep_jitter", lambda *a, **k: None)
    result = robust_yf.download_history("AAPL", cache_dir=str(tmp_path))
    assert len(result) == 3
