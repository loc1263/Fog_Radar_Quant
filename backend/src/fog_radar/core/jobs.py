"""Registro de ejecuciones de pipelines (batch).

Cada ejecución deja dos líneas en ``data/_jobs/<modulo>.jsonl``: una al
empezar (``running``) y otra al terminar (``success``/``error``) con el mismo
``id``. La API lee este registro para mostrar el estado de cada módulo en el
hub y en ``/estado``.
"""

from __future__ import annotations

import time
import traceback
import uuid
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Literal

from filelock import FileLock
from pydantic import BaseModel

from fog_radar.core.logging import get_logger

log = get_logger("jobs")

JobStatus = Literal["running", "success", "error", "stale"]
JobTrigger = Literal["cli", "cron", "api"]

# Una corrida "running" más vieja que esto se considera colgada.
STALE_AFTER = timedelta(hours=6)


class JobRecord(BaseModel):
    id: str
    module: str
    status: JobStatus
    trigger: JobTrigger = "cli"
    started_at: datetime
    finished_at: datetime | None = None
    duration_s: float | None = None
    message: str | None = None


class JobRegistry:
    def __init__(self, jobs_dir: Path) -> None:
        self.jobs_dir = jobs_dir

    def _path(self, module: str) -> Path:
        return self.jobs_dir / f"{module}.jsonl"

    def _append(self, record: JobRecord) -> None:
        self.jobs_dir.mkdir(parents=True, exist_ok=True)
        path = self._path(record.module)
        with FileLock(str(path) + ".lock", timeout=10), path.open("a", encoding="utf-8") as fh:
            fh.write(record.model_dump_json() + "\n")

    def _read(self, module: str) -> list[JobRecord]:
        path = self._path(module)
        if not path.is_file():
            return []
        records: list[JobRecord] = []
        for line in path.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                records.append(JobRecord.model_validate_json(line))
            except ValueError:
                log.warning("Línea inválida en %s, se ignora", path.name)
        return records

    @staticmethod
    def _mark_stale(rec: JobRecord, now: datetime) -> JobRecord:
        if rec.status == "running" and now - rec.started_at > STALE_AFTER:
            return rec.model_copy(update={"status": "stale"})
        return rec

    def history(self, module: str, limit: int = 20) -> list[JobRecord]:
        """Ejecuciones del módulo (la más reciente primero), una por ``id``."""
        latest_by_id: dict[str, JobRecord] = {}
        for rec in self._read(module):
            latest_by_id[rec.id] = rec  # la última línea de cada id gana
        now = datetime.now(UTC)
        ordered = sorted(latest_by_id.values(), key=lambda r: r.started_at, reverse=True)
        return [self._mark_stale(r, now) for r in ordered[:limit]]

    def latest(self, module: str) -> JobRecord | None:
        hist = self.history(module, limit=1)
        return hist[0] if hist else None

    def last_success(self, module: str) -> JobRecord | None:
        return next((r for r in self.history(module, limit=200) if r.status == "success"), None)

    def is_running(self, module: str) -> bool:
        rec = self.latest(module)
        return rec is not None and rec.status == "running"

    @contextmanager
    def track(self, module: str, *, trigger: JobTrigger = "cli") -> Iterator[JobRecord]:
        """Envuelve una ejecución de pipeline y registra su resultado.

        Uso::

            with registry.track("etf", trigger="cron") as job:
                run_pipeline()
                job.message = "1.532 ETFs procesados"
        """
        rec = JobRecord(
            id=uuid.uuid4().hex[:12],
            module=module,
            status="running",
            trigger=trigger,
            started_at=datetime.now(UTC),
        )
        self._append(rec)
        t0 = time.perf_counter()
        log.info("Job %s (%s) iniciado", rec.id, module)
        try:
            yield rec
        except BaseException as exc:
            rec.status = "error"
            rec.message = rec.message or f"{type(exc).__name__}: {exc}"
            log.error("Job %s (%s) falló:\n%s", rec.id, module, traceback.format_exc())
            raise
        else:
            rec.status = "success"
        finally:
            rec.finished_at = datetime.now(UTC)
            rec.duration_s = round(time.perf_counter() - t0, 2)
            self._append(rec)
            log.info("Job %s (%s) terminó: %s en %.1fs", rec.id, module, rec.status, rec.duration_s)
