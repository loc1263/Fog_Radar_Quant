from __future__ import annotations

import os
import time
from pathlib import Path

from fog_radar.core.runs import data_lock, latest_file, latest_run_dir, list_files, read_json


def test_latest_file_by_name(tmp_path: Path) -> None:
    for ts in ["20260901_101010", "20260927_080000", "20260915_120000"]:
        (tmp_path / f"ranking_acciones_{ts}.csv").write_text("x")
    (tmp_path / "otro.csv").write_text("x")
    latest = latest_file(tmp_path, "ranking_acciones_*.csv")
    assert latest is not None and latest.name == "ranking_acciones_20260927_080000.csv"
    assert len(list_files(tmp_path, "ranking_acciones_*.csv")) == 3


def test_latest_file_by_mtime(tmp_path: Path) -> None:
    a, b = tmp_path / "b.json", tmp_path / "a.json"
    a.write_text("1")
    b.write_text("2")
    old = time.time() - 100
    os.utime(a, (old, old))
    assert latest_file(tmp_path, "*.json", key="mtime") == b


def test_latest_run_dir_skips_hidden_and_files(tmp_path: Path) -> None:
    for d in ["20260924_01", "20260926_01", "20260925_02", ".tmp"]:
        (tmp_path / d).mkdir()
    (tmp_path / "20269999_99.txt").write_text("no soy carpeta")
    assert latest_run_dir(tmp_path).name == "20260926_01"


def test_missing_dir_returns_none(tmp_path: Path) -> None:
    assert latest_file(tmp_path / "nada", "*.csv") is None
    assert latest_run_dir(tmp_path / "nada") is None


def test_read_json_and_lock(tmp_path: Path) -> None:
    p = tmp_path / "r.json"
    p.write_text('{"top": ["AAPL"]}', encoding="utf-8")
    assert read_json(p) == {"top": ["AAPL"]}
    with data_lock(tmp_path / "etf"):
        assert (tmp_path / "etf" / ".lock").exists()
