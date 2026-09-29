"""Secuencias de pipelines (fog_radar.jobs.runner / schedule / CLI --todos y --programa).

Los pipelines reales se reemplazan por dobles: acá se prueba el orden, el lock
global, los reintentos, la pausa, el registro de jobs y las alertas."""

from __future__ import annotations

import json
import os
import threading
from collections.abc import Iterator
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

import pytest

from fog_radar import jobs as jobs_pkg
from fog_radar.core.jobs import JobRegistry
from fog_radar.jobs import Pipeline, PipelineOption, runner
from fog_radar.jobs import __main__ as cli
from fog_radar.jobs.schedule import PROGRAMAS_BY_KEY, crontab
from fog_radar.settings import Settings

SCL = ZoneInfo("America/Santiago")


class Llamadas(list):
    """(módulo, opciones) de cada ejecución + fallas programadas por módulo."""

    def __init__(self) -> None:
        super().__init__()
        self.fallas: dict[str, int] = {}


@pytest.fixture
def llamadas(monkeypatch: pytest.MonkeyPatch) -> Iterator[Llamadas]:
    """Reemplaza los 4 pipelines por dobles que anotan (módulo, opciones)."""
    registro = Llamadas()
    fallas = registro.fallas

    def falso(modulo: str, opciones: tuple[PipelineOption, ...] = ()) -> Pipeline:
        def run(settings: Settings, **opts: bool) -> str:
            registro.append((modulo, opts))
            if fallas.get(modulo, 0) > 0:
                fallas[modulo] -= 1
                raise RuntimeError(f"Yahoo bloqueó {modulo}")
            return f"{modulo} ok"

        return Pipeline(run, opciones)

    originales = jobs_pkg.PIPELINES
    nuevos = {k: falso(k, p.options) for k, p in originales.items()}
    monkeypatch.setattr(jobs_pkg, "PIPELINES", nuevos)
    monkeypatch.setattr(runner, "PIPELINES", nuevos)
    monkeypatch.setattr(cli, "PIPELINES", nuevos)
    yield registro


@pytest.fixture
def env(settings: Settings) -> Iterator[None]:
    """El CLI lee Settings del entorno: apuntarlo a la carpeta temporal del test."""
    from fog_radar import settings as settings_mod

    settings_mod.get_settings.cache_clear()
    os.environ.update({"DATA_DIR": str(settings.data_dir), "ENV": "test", "PIPELINE_PAUSA_S": "0"})
    yield
    for k in ("DATA_DIR", "ENV", "PIPELINE_PAUSA_S"):
        os.environ.pop(k, None)
    settings_mod.get_settings.cache_clear()


def test_programas_y_proxima_corrida() -> None:
    semanal, diario = PROGRAMAS_BY_KEY["semanal"], PROGRAMAS_BY_KEY["diario"]
    assert semanal.modulos == ("etf",)
    assert semanal.cron == "0 6 * * 1"
    assert diario.modulos == ("cl_stocks", "us_stocks", "tendencia")
    assert diario.cron == "0 18 * * 1-5"
    lunes_noche = datetime(2026, 9, 28, 21, 0, tzinfo=SCL)
    assert semanal.proxima(lunes_noche) == datetime(2026, 10, 5, 6, 0, tzinfo=SCL)
    assert diario.proxima(lunes_noche) == datetime(2026, 9, 29, 18, 0, tzinfo=SCL)
    viernes_noche = datetime(2026, 10, 2, 19, 0, tzinfo=SCL)
    assert diario.proxima(viernes_noche) == datetime(2026, 10, 5, 18, 0, tzinfo=SCL)
    assert semanal.proxima(datetime(2026, 10, 5, 5, 59, tzinfo=SCL)).day == 5


def test_crontab_versionado_al_dia() -> None:
    """deploy/backend/crontab debe coincidir con lo que genera schedule.py."""
    versionado = Path(__file__).resolve().parents[2] / "deploy" / "backend" / "crontab"
    assert versionado.read_text(encoding="utf-8") == crontab("/opt/fog-radar/backend")


def test_secuencia_en_orden_con_pausa(settings: Settings, llamadas: Llamadas) -> None:
    pausas: list[float] = []
    res = runner.correr_secuencia(
        settings, ["cl_stocks", "us_stocks", "tendencia"], pausa_s=45, reintentos=0, sleep=pausas.append
    )
    assert [m for m, _ in llamadas] == ["cl_stocks", "us_stocks", "tendencia"]
    assert [r.estado for r in res] == ["success"] * 3
    assert pausas == [45, 45]  # entre módulos, no después del último
    registry = JobRegistry(settings.jobs_dir)
    assert registry.latest("tendencia").message == "tendencia ok"


def test_falla_se_reintenta_y_la_secuencia_sigue(settings: Settings, llamadas: Llamadas) -> None:
    llamadas.fallas.update({"us_stocks": 5})
    esperas: list[float] = []
    res = runner.correr_secuencia(
        settings,
        ["cl_stocks", "us_stocks", "tendencia"],
        pausa_s=0,
        reintentos=1,
        espera_reintento_s=600,
        sleep=esperas.append,
    )
    assert [r.estado for r in res] == ["success", "error", "success"]
    assert res[1].intentos == 2 and "Yahoo bloqueó" in (res[1].mensaje or "")
    assert esperas == [600]
    historial = JobRegistry(settings.jobs_dir).history("us_stocks")
    assert [h.status for h in historial] == ["error", "error"]


def test_reintento_exitoso(settings: Settings, llamadas: Llamadas) -> None:
    llamadas.fallas.update({"etf": 1})
    res = runner.correr_secuencia(settings, ["etf"], reintentos=2, sleep=lambda s: None)
    assert res[0].estado == "success" and res[0].intentos == 2


def test_lock_global_impide_corridas_en_paralelo(settings: Settings, llamadas: Llamadas) -> None:
    tomado, soltar = threading.Event(), threading.Event()

    def otra_corrida() -> None:
        with runner.lock_pipelines(settings):
            tomado.set()
            soltar.wait(5)

    t = threading.Thread(target=otra_corrida)
    t.start()
    tomado.wait(5)
    try:
        with pytest.raises(runner.OcupadoError):
            runner.correr_secuencia(settings, ["cl_stocks"], esperar_lock_s=0.1)
    finally:
        soltar.set()
        t.join()
    assert llamadas == []
    # Liberado el lock, corre normalmente.
    assert runner.correr_secuencia(settings, ["cl_stocks"], sleep=lambda s: None)[0].estado == "success"


def test_modulo_ya_en_curso_se_omite(settings: Settings, llamadas: Llamadas) -> None:
    with JobRegistry(settings.jobs_dir).track("etf"):
        res = runner.correr_secuencia(settings, ["etf", "cl_stocks"], pausa_s=0)
    assert res[0].estado == "error" and "en curso" in (res[0].mensaje or "")
    assert [m for m, _ in llamadas] == ["cl_stocks"]


def test_alerta_por_webhook(settings: Settings, llamadas: Llamadas, monkeypatch: pytest.MonkeyPatch) -> None:
    from pydantic import SecretStr

    enviados: list[dict] = []

    class _Resp:
        def __enter__(self) -> _Resp:
            return self

        def __exit__(self, *a: object) -> None:
            return None

    def urlopen(req, timeout: float) -> _Resp:  # noqa: ANN001
        enviados.append({"url": req.full_url, **json.loads(req.data)})
        return _Resp()

    monkeypatch.setattr(runner.urllib.request, "urlopen", urlopen)
    settings.alert_webhook_url = SecretStr("https://hooks.example/abc")
    llamadas.fallas.update({"tendencia": 9})
    runner.correr_secuencia(settings, ["tendencia", "cl_stocks"], pausa_s=0, reintentos=0, trigger="cron")
    assert len(enviados) == 1
    assert enviados[0]["url"] == "https://hooks.example/abc"
    assert "tendencia" in enviados[0]["text"] and "cron" in enviados[0]["text"]
    # Sin fallas no hay aviso.
    runner.correr_secuencia(settings, ["cl_stocks"], pausa_s=0)
    assert len(enviados) == 1


def test_cli_todos_orden_y_solo(env: None, llamadas: Llamadas, capsys: pytest.CaptureFixture[str]) -> None:
    assert cli.main(["--todos"]) == 0
    assert [m for m, _ in llamadas] == ["cl_stocks", "us_stocks", "tendencia", "etf"]
    assert "Resumen de la secuencia" in capsys.readouterr().out
    llamadas.clear()
    assert cli.main(["--todos", "--solo", "etf,cl_stocks"]) == 0
    assert [m for m, _ in llamadas] == ["cl_stocks", "etf"]
    assert cli.main(["--todos", "--solo", "nada"]) == 2


def test_cli_offline(env: None, llamadas: Llamadas) -> None:
    assert cli.main(["--todos", "--offline"]) == 0
    assert llamadas == [
        ("cl_stocks", {"solo_scoring": True}),
        ("tendencia", {"skip_fetch": True, "skip_discovery": True}),
        ("etf", {"solo_analisis": True}),
    ]  # us_stocks no tiene modo offline: omitido


def test_cli_programa_cron(env: None, llamadas: Llamadas, settings: Settings) -> None:
    assert cli.main(["--programa", "semanal", "--trigger", "cron"]) == 0
    assert [m for m, _ in llamadas] == ["etf"]
    assert JobRegistry(settings.jobs_dir).latest("etf").trigger == "cron"


def test_cli_falla_devuelve_1(env: None, llamadas: Llamadas) -> None:
    llamadas.fallas.update({"etf": 1})
    assert cli.main(["etf"]) == 1  # manual: sin reintentos


def test_cli_ocupado_devuelve_3(env: None, llamadas: Llamadas, settings: Settings) -> None:
    tomado, soltar = threading.Event(), threading.Event()

    def otra() -> None:
        with runner.lock_pipelines(settings):
            tomado.set()
            soltar.wait(5)

    t = threading.Thread(target=otra)
    t.start()
    tomado.wait(5)
    try:
        assert cli.main(["--todos"]) == 3
    finally:
        soltar.set()
        t.join()


def test_cli_banderas_de_modulo_con_todos(env: None, llamadas: Llamadas) -> None:
    assert cli.main(["--todos", "--skip-fetch"]) == 2


def test_api_programas(client, auth) -> None:  # noqa: ANN001
    body = client.get("/api/jobs/programas", headers=auth).json()
    assert [p["key"] for p in body] == ["semanal", "diario"]
    assert body[1]["modulos"] == ["cl_stocks", "us_stocks", "tendencia"]
    assert body[0]["zona"] == "America/Santiago"
    assert datetime.fromisoformat(body[0]["proxima"]).weekday() == 0


def test_cli_opciones_con_valor(env: None, llamadas: Llamadas) -> None:
    """TD-CFG-1: --top-n N llega como kwarg entero; fuera de rango es error de uso."""
    assert cli.main(["tendencia", "--top-n", "20", "--max-weight", "0.1", "--skip-fetch"]) == 0
    assert llamadas[-1] == ("tendencia", {"top_n": 20, "max_weight": 0.1, "skip_fetch": True})
    with pytest.raises(SystemExit) as exc:
        cli.main(["tendencia", "--top-n", "1"])
    assert exc.value.code == 2
    with pytest.raises(SystemExit):
        cli.main(["tendencia", "--max-weight", "abc"])
    assert cli.main(["etf", "--top-n", "20"]) == 2  # opción de otro módulo


def test_programa_anterior() -> None:
    diario = PROGRAMAS_BY_KEY["diario"]
    # Domingo → la última diaria fue el viernes 18:00.
    assert diario.anterior(datetime(2026, 9, 27, 12, 0, tzinfo=SCL)) == datetime(
        2026, 9, 25, 18, 0, tzinfo=SCL
    )
    # Justo a la hora cuenta como ya ocurrida.
    assert diario.anterior(datetime(2026, 9, 28, 18, 0, tzinfo=SCL)) == datetime(
        2026, 9, 28, 18, 0, tzinfo=SCL
    )
