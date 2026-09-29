from __future__ import annotations

from datetime import UTC, datetime, timedelta
from pathlib import Path

import pytest

from fog_radar.core.jobs import JobRecord, JobRegistry


def test_track_success(tmp_path: Path) -> None:
    reg = JobRegistry(tmp_path)
    with reg.track("cl_stocks", trigger="cron") as job:
        job.message = "30 acciones"
    rec = reg.latest("cl_stocks")
    assert rec is not None
    assert rec.status == "success" and rec.trigger == "cron" and rec.message == "30 acciones"
    assert rec.duration_s is not None and rec.finished_at is not None
    assert not reg.is_running("cl_stocks")


def test_track_error_records_and_reraises(tmp_path: Path) -> None:
    reg = JobRegistry(tmp_path)
    with pytest.raises(ValueError), reg.track("etf"):
        raise ValueError("Yahoo caído")
    rec = reg.latest("etf")
    assert rec is not None and rec.status == "error"
    assert rec.message == "ValueError: Yahoo caído"
    assert reg.last_success("etf") is None


def test_history_dedupes_by_id_and_orders(tmp_path: Path) -> None:
    reg = JobRegistry(tmp_path)
    for _ in range(3):
        with reg.track("us_stocks"):
            pass
    hist = reg.history("us_stocks")
    assert len(hist) == 3  # 6 líneas, 3 ids
    assert hist[0].started_at >= hist[1].started_at >= hist[2].started_at
    assert len(reg.history("us_stocks", limit=2)) == 2


def test_running_and_stale(tmp_path: Path) -> None:
    reg = JobRegistry(tmp_path)
    now = datetime.now(UTC)
    reg._append(JobRecord(id="a", module="tendencia", status="running", started_at=now))
    assert reg.is_running("tendencia")
    reg._append(JobRecord(id="b", module="tendencia", status="running", started_at=now - timedelta(hours=1)))
    reg._append(JobRecord(id="c", module="etf", status="running", started_at=now - timedelta(hours=7)))
    assert reg.latest("etf").status == "stale"
    assert not reg.is_running("etf")


def test_ignores_corrupt_lines(tmp_path: Path) -> None:
    reg = JobRegistry(tmp_path)
    with reg.track("etf"):
        pass
    with (tmp_path / "etf.jsonl").open("a") as fh:
        fh.write("{esto no es json\n")
    assert reg.latest("etf").status == "success"


def test_empty_registry(tmp_path: Path) -> None:
    reg = JobRegistry(tmp_path / "no-existe")
    assert reg.latest("etf") is None and reg.history("etf") == []


def test_registro_se_compacta(tmp_path, monkeypatch) -> None:  # noqa: ANN001
    from fog_radar.core import jobs

    monkeypatch.setattr(jobs, "MAX_BYTES", 2000)
    monkeypatch.setattr(jobs, "MAX_LINEAS", 6)
    reg = jobs.JobRegistry(tmp_path)
    for _ in range(20):
        with reg.track("etf"):
            pass
    lineas = (tmp_path / "etf.jsonl").read_text(encoding="utf-8").splitlines()
    assert len(lineas) < 40  # sin rotación serían 40 (2 por corrida)
    assert reg.latest("etf").status == "success"
