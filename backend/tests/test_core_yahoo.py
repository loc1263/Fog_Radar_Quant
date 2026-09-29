"""Cliente común de Yahoo (fog_radar.core.yahoo): espaciado, 429, caché en disco."""

from __future__ import annotations

import os
import threading
import time
from pathlib import Path

import pandas as pd
import pytest

from fog_radar.core import yahoo


class Reloj:
    def __init__(self) -> None:
        self.t = 0.0
        self.dormido: list[float] = []

    def clock(self) -> float:
        return self.t

    def sleep(self, s: float) -> None:
        self.dormido.append(s)
        self.t += s


def test_espaciador_respeta_intervalo_minimo() -> None:
    r = Reloj()
    esp = yahoo.Espaciador(1.0, clock=r.clock, sleep=r.sleep)
    assert esp.esperar_turno() == 0  # la primera no espera
    assert esp.esperar_turno() == pytest.approx(1.0)
    r.t += 5  # pasó tiempo de sobra: no espera
    assert esp.esperar_turno() == 0
    assert r.dormido == [pytest.approx(1.0)]


def test_espaciador_entre_hilos_serializa_turnos() -> None:
    esp = yahoo.Espaciador(0.02)
    inicios: list[float] = []
    lock = threading.Lock()

    def tarea() -> None:
        esp.esperar_turno()
        with lock:
            inicios.append(time.monotonic())

    hilos = [threading.Thread(target=tarea) for _ in range(5)]
    for h in hilos:
        h.start()
    for h in hilos:
        h.join()
    inicios.sort()
    gaps = [b - a for a, b in zip(inicios, inicios[1:], strict=False)]
    assert min(gaps) >= 0.015  # margen por la resolución del reloj


class Resp:
    def __init__(self, status: int) -> None:
        self.status_code = status


def test_control_reintenta_429_con_backoff() -> None:
    yahoo.reiniciar_estadisticas()
    r = Reloj()
    control = yahoo.ControlTrafico(
        yahoo.Politica(min_intervalo_s=0, jitter_s=0, reintentos_429=3, backoff_base_s=2, backoff_max_s=5),
        sleep=r.sleep,
        clock=r.clock,
    )
    respuestas = iter([429, 429, 200])
    resp = control.ejecutar(lambda: Resp(next(respuestas)), "https://query1.finance.yahoo.com/v8/x?a=1")
    assert resp.status_code == 200
    # backoff: 2·2^0 + U(0,1), 2·2^1 + U(0,1)
    assert 2 <= r.dormido[0] <= 3 and 4 <= r.dormido[1] <= 5
    st = yahoo.estadisticas()
    assert (st.requests, st.rate_limited, st.reintentos) == (3, 2, 2)


def test_control_devuelve_el_429_si_se_agotan_los_reintentos() -> None:
    r = Reloj()
    control = yahoo.ControlTrafico(
        yahoo.Politica(min_intervalo_s=0, jitter_s=0, reintentos_429=1, backoff_base_s=0, backoff_max_s=0),
        sleep=r.sleep,
        clock=r.clock,
    )
    assert control.ejecutar(lambda: Resp(429)).status_code == 429
    assert len(r.dormido) == 1


def test_backoff_tiene_tope() -> None:
    assert yahoo.backoff_s(10, 5, 60) <= 61


def test_instalar_reemplaza_la_sesion_de_yfinance(settings) -> None:  # noqa: ANN001
    from yfinance.data import YfData

    politica = yahoo.Politica(min_intervalo_s=0.01, reintentos_429=2)
    assert yahoo.instalar(settings, politica)
    sesion = YfData()._session
    assert type(sesion).__name__ == "SesionYahoo"
    assert sesion.control.politica == politica
    assert (settings.data_dir / "_cache" / "yfinance").is_dir()
    assert yahoo.instalar(settings, politica)  # idempotente


def _df(n: int = 3) -> pd.DataFrame:
    idx = pd.date_range("2026-01-02", periods=n, freq="W-FRI", name="Date")
    return pd.DataFrame({"Close": range(1, n + 1)}, index=idx, dtype=float)


def test_descargar_historial_reintenta_y_cachea(tmp_path: Path) -> None:
    llamadas = []

    def bajar(ticker: str, o: yahoo.OpcionesHistorial) -> pd.DataFrame:
        llamadas.append(ticker)
        if len(llamadas) == 1:
            raise RuntimeError("Too Many Requests")
        return _df()

    opciones = yahoo.OpcionesHistorial(max_retries=3, pausa_min_s=0, pausa_max_s=0, backoff_base_s=0)
    df = yahoo.descargar_historial("7203.T", tmp_path, opciones, descargar=bajar, sleep=lambda s: None)
    assert len(df) == 3 and len(llamadas) == 2
    assert (tmp_path / "7203.T.csv").is_file() and not list(tmp_path.glob("*.tmp"))
    # Segunda vez: sale de la caché, sin llamar.
    df2 = yahoo.descargar_historial("7203.T", tmp_path, opciones, descargar=bajar, sleep=lambda s: None)
    assert len(df2) == 3 and len(llamadas) == 2
    # force_refresh vuelve a pedir.
    yahoo.descargar_historial(
        "7203.T", tmp_path, opciones, descargar=bajar, sleep=lambda s: None, force_refresh=True
    )
    assert len(llamadas) == 3


def test_descargar_historial_falla_definitiva_devuelve_vacio(tmp_path: Path) -> None:
    def vacio(ticker: str, o: yahoo.OpcionesHistorial) -> pd.DataFrame:
        return pd.DataFrame()

    opciones = yahoo.OpcionesHistorial(max_retries=2, pausa_min_s=0, pausa_max_s=0, backoff_base_s=0)
    res, fallidos = yahoo.descargar_varios(
        ["A", "B"], tmp_path, opciones, descargar=vacio, sleep=lambda s: None
    )
    assert res == {} and fallidos == ["A", "B"]


def test_cache_vieja_no_se_usa(tmp_path: Path) -> None:
    f = tmp_path / "X.csv"
    _df().to_csv(f)
    viejo = time.time() - 3 * 24 * 3600
    os.utime(f, (viejo, viejo))
    assert not yahoo.cache_fresca(f, 20)
    assert yahoo.cache_fresca(f, 100)
