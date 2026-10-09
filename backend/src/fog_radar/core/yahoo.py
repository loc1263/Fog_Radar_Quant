"""Cliente común de Yahoo Finance (yfinance) para los 4 módulos.

Antes cada módulo tenía su propia defensa contra el rate-limit de Yahoo
(pausas, reintentos, caché). Esas defensas se mantienen (son parte de la
paridad de cada módulo), pero ahora todas pasan además por **una sola sesión
HTTP** que aplica reglas globales al proceso:

1. **Espaciado mínimo entre requests** (``min_intervalo_s`` + jitter),
   compartido por todos los hilos: el screener de ETF y ``yf.download`` de
   EEUU usan hilos y antes podían disparar ráfagas.
2. **Reintento con backoff exponencial + jitter ante HTTP 429**, de forma
   transparente para el código que llama (``Ticker.history``, ``yf.screen``,
   ``yf.download``…).
3. **Contadores** (requests, 429, esperas) para el resumen de cada corrida.

yfinance (>= 0.2.5x / 1.x) guarda una sesión *singleton* (``YfData``) que
usan todas las llamadas del proceso; :func:`instalar` la reemplaza por
:class:`SesionYahoo` (curl_cffi con *impersonation* de Chrome, igual que la
de yfinance por defecto). También fija la caché de zonas horarias de
yfinance dentro de ``data/_cache`` (en vez de ``~/.cache``).

Además expone la **descarga de históricos con caché en disco** que usaba
Tendencia (``net/robust_yf``), ahora parametrizable para cualquier módulo.

Entre procesos no hace falta coordinación: las corridas batch ya se
serializan con el lock global de ``fog_radar.jobs.runner``.
"""

from __future__ import annotations

import contextvars
import logging
import os
import random
import threading
import time
from collections.abc import Callable, Iterable, Iterator
from contextlib import contextmanager
from dataclasses import asdict, dataclass
from datetime import UTC, datetime
from typing import TYPE_CHECKING, Any

from fog_radar.core.logging import get_logger

if TYPE_CHECKING:  # pragma: no cover
    import pandas as pd

    from fog_radar.settings import Settings

log = get_logger("yahoo")

Sleep = Callable[[float], None]
Clock = Callable[[], float]


# ---------------------------------------------------------------------------
# Espaciado global entre requests
# ---------------------------------------------------------------------------
class Espaciador:
    """Garantiza un intervalo mínimo (con jitter) entre el inicio de dos requests.

    Seguro entre hilos: reserva el próximo turno bajo un lock y duerme fuera
    de él, así varios hilos quedan en fila sin bloquearse mutuamente.
    """

    def __init__(
        self,
        min_intervalo_s: float,
        jitter_s: float = 0.0,
        *,
        clock: Clock = time.monotonic,
        sleep: Sleep = time.sleep,
        rng: random.Random | None = None,
    ) -> None:
        self.min_intervalo_s = max(0.0, min_intervalo_s)
        self.jitter_s = max(0.0, jitter_s)
        self._clock = clock
        self._sleep = sleep
        self._rng = rng or random.Random()
        self._lock = threading.Lock()
        self._proximo = 0.0

    def esperar_turno(self) -> float:
        """Duerme lo necesario y devuelve los segundos esperados."""
        with self._lock:
            ahora = self._clock()
            inicio = max(ahora, self._proximo)
            intervalo = self.min_intervalo_s + (self._rng.uniform(0, self.jitter_s) if self.jitter_s else 0.0)
            self._proximo = inicio + intervalo
        espera = inicio - ahora
        if espera > 0:
            self._sleep(espera)
        return max(0.0, espera)


def backoff_s(intento: int, base_s: float, max_s: float, rng: random.Random | None = None) -> float:
    """Backoff exponencial con tope y jitter: ``min(base·2^intento, max) + U(0, 1)``."""
    r = rng or random
    return min(base_s * (2**intento), max_s) + r.uniform(0, 1)


# ---------------------------------------------------------------------------
# Contadores
# ---------------------------------------------------------------------------
@dataclass
class Estadisticas:
    requests: int = 0
    rate_limited: int = 0
    reintentos: int = 0
    espera_s: float = 0.0

    def resumen(self) -> str:
        return (
            f"{self.requests} requests a Yahoo, {self.rate_limited} respuestas 429, "
            f"{self.reintentos} reintentos, {self.espera_s:.0f} s de espera"
        )


_stats = Estadisticas()
_stats_lock = threading.Lock()


def estadisticas() -> Estadisticas:
    """Copia de los contadores del proceso (desde :func:`instalar` o :func:`reiniciar_estadisticas`)."""
    with _stats_lock:
        return Estadisticas(**asdict(_stats))


def reiniciar_estadisticas() -> None:
    global _stats
    with _stats_lock:
        _stats = Estadisticas()


def _sumar(**deltas: float) -> None:
    with _stats_lock:
        for k, v in deltas.items():
            setattr(_stats, k, getattr(_stats, k) + v)


# ---------------------------------------------------------------------------
# Política y sesión
# ---------------------------------------------------------------------------
@dataclass(frozen=True)
class Politica:
    min_intervalo_s: float = 0.35
    jitter_s: float = 0.25
    reintentos_429: int = 3
    backoff_base_s: float = 5.0
    backoff_max_s: float = 60.0
    reintentos_red: int = 2
    """Reintentos de yfinance ante errores de red transitorios (``YfConfig.network.retries``)."""

    @classmethod
    def desde_settings(cls, settings: Settings) -> Politica:
        return cls(
            min_intervalo_s=settings.yahoo_min_intervalo_s,
            jitter_s=settings.yahoo_jitter_s,
            reintentos_429=settings.yahoo_reintentos_429,
            backoff_base_s=settings.yahoo_backoff_base_s,
            backoff_max_s=settings.yahoo_backoff_max_s,
            reintentos_red=settings.yahoo_reintentos_red,
        )


def _respuesta_429(resp: Any) -> bool:
    return getattr(resp, "status_code", None) == 429


_sin_reintentos: contextvars.ContextVar[bool] = contextvars.ContextVar("yahoo_sin_reintentos", default=False)


@contextmanager
def interactivo() -> Iterator[None]:
    """Dentro de una solicitud HTTP: un 429 de Yahoo se devuelve de inmediato (sin
    esperar backoff de decenas de segundos con un hilo del servidor tomado)."""
    token = _sin_reintentos.set(True)
    try:
        yield
    finally:
        _sin_reintentos.reset(token)


class ControlTrafico:
    """Espaciado + reintento ante 429 alrededor de una función que hace el request.

    Separado de la sesión para poder testearlo sin red.
    """

    def __init__(
        self, politica: Politica, *, sleep: Sleep = time.sleep, clock: Clock = time.monotonic
    ) -> None:
        self.politica = politica
        self._sleep = sleep
        self.espaciador = Espaciador(politica.min_intervalo_s, politica.jitter_s, clock=clock, sleep=sleep)

    def ejecutar(self, hacer_request: Callable[[], Any], url: str = "") -> Any:
        p = self.politica
        reintentos = 0 if _sin_reintentos.get() else p.reintentos_429
        resp = None
        for intento in range(reintentos + 1):
            esperado = self.espaciador.esperar_turno()
            resp = hacer_request()
            _sumar(requests=1, espera_s=esperado)
            if not _respuesta_429(resp):
                return resp
            _sumar(rate_limited=1)
            if intento == reintentos:
                break
            pausa = backoff_s(intento, p.backoff_base_s, p.backoff_max_s)
            log.warning(
                "Yahoo respondió 429 (%s); reintento %d/%d en %.0f s",
                url.split("?")[0][-80:],
                intento + 1,
                p.reintentos_429,
                pausa,
            )
            _sumar(reintentos=1, espera_s=pausa)
            self._sleep(pausa)
        return resp


def _clase_sesion() -> type:
    from curl_cffi import requests as curl_requests

    class SesionYahoo(curl_requests.Session):  # type: ignore[misc, name-defined]
        """Sesión curl_cffi que pasa cada request por :class:`ControlTrafico`."""

        control: ControlTrafico

        def request(self, method: str, url: str, **kwargs: Any) -> Any:  # type: ignore[override]
            return self.control.ejecutar(lambda: super(SesionYahoo, self).request(method, url, **kwargs), url)

    return SesionYahoo


_instalada: Politica | None = None
_instalar_lock = threading.Lock()


def instalar(settings: Settings | None = None, politica: Politica | None = None) -> bool:
    """Hace que todo yfinance del proceso use la sesión común. Idempotente.

    Devuelve ``False`` (sin fallar) si yfinance/curl_cffi no están disponibles
    o su API interna cambió: en ese caso yfinance sigue con su sesión propia.
    """
    global _instalada
    if settings is not None and politica is None:
        politica = Politica.desde_settings(settings)
    politica = politica or Politica()
    with _instalar_lock:
        if _instalada == politica:
            return True
        try:
            import yfinance as yf
            from yfinance.data import YfData

            sesion = _clase_sesion()(impersonate="chrome")
            sesion.control = ControlTrafico(politica)
            YfData(session=sesion)  # el singleton adopta la sesión nueva
            yf.config.network.retries = politica.reintentos_red
            # yfinance loguea como ERROR cada ticker sin datos (404, "possibly delisted"); los
            # pipelines ya reportan esos tickers (✗ TICKER / excluidos), así que es ruido duplicado.
            logging.getLogger("yfinance").setLevel(logging.CRITICAL)
            if settings is not None:
                cache = settings.data_dir / "_cache" / "yfinance"
                cache.mkdir(parents=True, exist_ok=True)
                yf.set_tz_cache_location(str(cache))
        except Exception as exc:  # pragma: no cover - depende de la versión de yfinance
            log.warning("No se pudo instalar la sesión común de Yahoo (%s); yfinance usa la suya.", exc)
            return False
        _instalada = politica
        log.info(
            "Sesión común de Yahoo instalada (≥ %.2f s entre requests, %d reintentos ante 429)",
            politica.min_intervalo_s,
            politica.reintentos_429,
        )
        return True


# ---------------------------------------------------------------------------
# Históricos con caché en disco (ex tendencia/net/robust_yf)
# ---------------------------------------------------------------------------
def cache_path(cache_dir: str | os.PathLike[str], ticker: str) -> str:
    """Ruta del CSV en caché de ``ticker`` (``^`` y ``=`` no son válidos en nombres de archivo)."""
    safe_name = ticker.replace("^", "_").replace("=", "_")
    return os.path.join(cache_dir, f"{safe_name}.csv")


def cache_fresca(path: str | os.PathLike[str], max_age_hours: float = 20) -> bool:
    if not os.path.exists(path):
        return False
    mtime = datetime.fromtimestamp(os.path.getmtime(path), tz=UTC)
    age_hours = (datetime.now(UTC) - mtime).total_seconds() / 3600
    return age_hours < max_age_hours


def _es_rate_limit(exc: BaseException) -> bool:
    msg = str(exc).lower()
    return (
        type(exc).__name__ == "YFRateLimitError"
        or "429" in msg
        or "rate" in msg
        or "too many requests" in msg
    )


@dataclass(frozen=True)
class OpcionesHistorial:
    period: str = "5y"
    interval: str = "1wk"
    max_retries: int = 5
    pausa_min_s: float = 1.5
    pausa_max_s: float = 3.5
    backoff_base_s: float = 5.0
    backoff_max_s: float = 120.0
    timeout_s: float = 30.0
    cache_max_horas: float = 20.0
    auto_adjust: bool = True


def descargar_historial(
    ticker: str,
    cache_dir: str | os.PathLike[str],
    opciones: OpcionesHistorial | None = None,
    *,
    force_refresh: bool = False,
    logger: logging.Logger | None = None,
    sleep: Sleep | None = None,
    descargar: Callable[[str, OpcionesHistorial], pd.DataFrame] | None = None,
) -> pd.DataFrame:
    """Histórico de un ticker con caché en disco, reintentos y pausa entre tickers.

    Devuelve un DataFrame vacío si falla tras agotar los reintentos (el
    llamador decide si excluye el ticker o aborta).
    """
    import pandas as pd

    o = opciones or OpcionesHistorial()
    lg = logger or log
    dormir = sleep or time.sleep
    os.makedirs(cache_dir, exist_ok=True)
    cache_file = cache_path(cache_dir, ticker)

    if not force_refresh and cache_fresca(cache_file, o.cache_max_horas):
        try:
            df = pd.read_csv(cache_file, index_col=0, parse_dates=True)
            if not df.empty:
                lg.info(f"[cache] {ticker}: {len(df)} filas (sin request nuevo)")
                return df
        except Exception:
            pass  # caché corrupta: se vuelve a descargar

    bajar = descargar or _history_yfinance
    last_error: BaseException | None = None
    for attempt in range(o.max_retries):
        try:
            df = bajar(ticker, o)
            if df is None or df.empty:
                raise ValueError("respuesta vacía de Yahoo Finance")
            tmp = f"{cache_file}.tmp"
            df.to_csv(tmp)
            os.replace(tmp, cache_file)  # escritura atómica: un corte no deja un CSV a medias
            lg.info(f"[ok] {ticker}: {len(df)} filas descargadas (intento {attempt + 1})")
            dormir(random.uniform(o.pausa_min_s, o.pausa_max_s))
            return df
        except Exception as e:
            last_error = e
            level = "RATE-LIMIT" if _es_rate_limit(e) else "ERROR"
            lg.warning(f"[{level}] {ticker}: intento {attempt + 1}/{o.max_retries} falló: {e}")
            if attempt < o.max_retries - 1:
                dormir(backoff_s(attempt, o.backoff_base_s, o.backoff_max_s) + random.uniform(0, 1))

    lg.error(f"[FALLO DEFINITIVO] {ticker}: {last_error}")
    return pd.DataFrame()


def _history_yfinance(ticker: str, o: OpcionesHistorial) -> pd.DataFrame:
    import yfinance as yf

    return yf.Ticker(ticker).history(
        period=o.period, interval=o.interval, timeout=o.timeout_s, auto_adjust=o.auto_adjust
    )


def descargar_varios(
    tickers: Iterable[str],
    cache_dir: str | os.PathLike[str],
    opciones: OpcionesHistorial | None = None,
    *,
    force_refresh: bool = False,
    label: str = "",
    logger: logging.Logger | None = None,
    **kw: Any,
) -> tuple[dict[str, pd.DataFrame], list[str]]:
    """Descarga secuencial (nunca en paralelo). Devuelve ``({ticker: df}, fallidos)``."""
    lg = logger or log
    results: dict[str, pd.DataFrame] = {}
    failed: list[str] = []
    lista = list(tickers)
    for i, ticker in enumerate(lista, start=1):
        lg.info(f"({i}/{len(lista)}) {label} descargando {ticker} ...")
        df = descargar_historial(ticker, cache_dir, opciones, force_refresh=force_refresh, logger=lg, **kw)
        if df.empty:
            failed.append(ticker)
        else:
            results[ticker] = df
    if failed:
        lg.warning(f"Tickers fallidos ({len(failed)}/{len(lista)}): {failed}")
    return results, failed
