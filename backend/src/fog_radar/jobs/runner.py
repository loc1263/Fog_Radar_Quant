"""Ejecución de pipelines en secuencia, nunca en paralelo.

Yahoo Finance bloquea por ráfagas de requests: si dos pipelines descargan a la
vez (el cron y una corrida manual, o dos programas que se solapan) los dos
terminan con errores de rate limit. Por eso:

- Toda ejecución (un módulo o una secuencia) toma un **lock global entre
  procesos** (``data/_jobs/.pipelines.lock``). Si está tomado, espera hasta
  ``esperar_lock`` segundos o termina con "ocupado".
- Los módulos de una secuencia corren **uno detrás de otro** con una pausa entre
  medio (``Settings.pipeline_pausa_s``).
- Un pipeline que falla se **reintenta** (``pipeline_reintentos``, tras
  ``pipeline_espera_reintento_s``) y la secuencia **sigue** con el siguiente
  módulo: una falla de Tendencia no deja a Chile sin datos.
- Si algo falló y hay ``ALERT_WEBHOOK_URL``, se manda un aviso.

Cada intento queda en el registro de jobs (``/api/jobs``, página ``/estado``).
"""

from __future__ import annotations

import json
import time
import urllib.request
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from dataclasses import dataclass
from typing import Literal

from filelock import FileLock, Timeout

from fog_radar.core.jobs import JobRegistry, JobTrigger
from fog_radar.core.logging import get_logger
from fog_radar.jobs import PIPELINES
from fog_radar.settings import Settings

log = get_logger("jobs.runner")

EstadoModulo = Literal["success", "error", "omitido"]


class OcupadoError(RuntimeError):
    """Otra secuencia de pipelines tiene el lock global."""


@dataclass
class ResultadoModulo:
    modulo: str
    estado: EstadoModulo
    intentos: int = 0
    mensaje: str | None = None
    duracion_s: float = 0.0


@contextmanager
def lock_pipelines(settings: Settings, esperar_s: float = 0) -> Iterator[None]:
    """Lock de SO (se libera solo si el proceso muere)."""
    settings.jobs_dir.mkdir(parents=True, exist_ok=True)
    lock = FileLock(str(settings.jobs_dir / ".pipelines.lock"), timeout=esperar_s)
    try:
        lock.acquire()
    except Timeout as exc:
        raise OcupadoError(
            "Ya hay una corrida de pipelines en curso (lock data/_jobs/.pipelines.lock)."
        ) from exc
    try:
        yield
    finally:
        lock.release()


def correr_secuencia(
    settings: Settings,
    modulos: list[str] | tuple[str, ...],
    *,
    trigger: JobTrigger = "cli",
    opciones: dict[str, dict[str, bool]] | None = None,
    omitir: dict[str, str] | None = None,
    pausa_s: float | None = None,
    reintentos: int | None = None,
    espera_reintento_s: float | None = None,
    esperar_lock_s: float = 0,
    sleep: Callable[[float], None] = time.sleep,
) -> list[ResultadoModulo]:
    """Corre ``modulos`` en orden bajo el lock global. Nunca lanza por la falla de
    un módulo: devuelve el resultado de cada uno (``OcupadoError`` si el lock no
    se consigue)."""
    opciones = opciones or {}
    omitir = omitir or {}
    pausa = settings.pipeline_pausa_s if pausa_s is None else pausa_s
    max_reintentos = settings.pipeline_reintentos if reintentos is None else reintentos
    espera = settings.pipeline_espera_reintento_s if espera_reintento_s is None else espera_reintento_s
    registry = JobRegistry(settings.jobs_dir)
    resultados: list[ResultadoModulo] = []

    with lock_pipelines(settings, esperar_lock_s):
        log.info("Secuencia de pipelines: %s", " → ".join(modulos))
        pendientes = [m for m in modulos if m not in omitir]
        for modulo in modulos:
            if modulo in omitir:
                log.info("[%s] omitido: %s", modulo, omitir[modulo])
                resultados.append(ResultadoModulo(modulo, "omitido", mensaje=omitir[modulo]))
                continue
            res = _correr_modulo(
                settings, registry, modulo, trigger, opciones.get(modulo, {}), max_reintentos, espera, sleep
            )
            resultados.append(res)
            pendientes.remove(modulo)
            if pendientes and pausa > 0:
                log.info("Pausa de %.0f s antes de %s (anti bloqueo de Yahoo)...", pausa, pendientes[0])
                sleep(pausa)

    fallidos = [r for r in resultados if r.estado == "error"]
    if fallidos:
        _alertar(settings, fallidos, trigger)
    return resultados


def _correr_modulo(
    settings: Settings,
    registry: JobRegistry,
    modulo: str,
    trigger: JobTrigger,
    opts: dict[str, bool],
    max_reintentos: int,
    espera: float,
    sleep: Callable[[float], None],
) -> ResultadoModulo:
    pipeline = PIPELINES[modulo]
    res = ResultadoModulo(modulo, "error")
    t0 = time.perf_counter()
    if registry.is_running(modulo):
        res.mensaje = "ya había una corrida de este módulo en curso"
        log.warning("[%s] %s: se omite", modulo, res.mensaje)
        return res
    for intento in range(1, max_reintentos + 2):
        res.intentos = intento
        log.info(
            "[%s] inicio (intento %d/%d)%s", modulo, intento, max_reintentos + 1, f" {opts}" if opts else ""
        )
        try:
            with registry.track(modulo, trigger=trigger) as job:
                job.message = pipeline.run(settings, **opts)
        except Exception as exc:
            res.mensaje = f"{type(exc).__name__}: {exc}"
            log.error("[%s] falló: %s", modulo, res.mensaje)
            if intento <= max_reintentos:
                log.info("[%s] reintento en %.0f s", modulo, espera)
                sleep(espera)
                continue
        else:
            res.estado = "success"
            res.mensaje = job.message
            log.info("[%s] OK: %s", modulo, job.message or "")
        break
    res.duracion_s = round(time.perf_counter() - t0, 1)
    return res


def _alertar(settings: Settings, fallidos: list[ResultadoModulo], trigger: str) -> None:
    if settings.alert_webhook_url is None:
        return
    texto = f"Fog Radar Quant: falló la corrida ({trigger}) de " + "; ".join(
        f"{r.modulo} ({r.mensaje})" for r in fallidos
    )
    # "text" lo leen Slack/Teams y "content" Discord.
    cuerpo = json.dumps({"text": texto, "content": texto[:1900]}).encode("utf-8")
    req = urllib.request.Request(
        settings.alert_webhook_url.get_secret_value(),
        data=cuerpo,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10):  # noqa: S310 - URL configurada por el administrador
            pass
    except Exception as exc:  # el aviso nunca debe romper la corrida
        log.warning("No se pudo enviar la alerta al webhook: %s", exc)


def resumen(resultados: list[ResultadoModulo]) -> str:
    iconos = {"success": "OK", "error": "ERROR", "omitido": "omitido"}
    lineas = [
        f"  {r.modulo:10s} {iconos[r.estado]:8s} {r.duracion_s:7.1f}s  {r.mensaje or ''}" for r in resultados
    ]
    return "\n".join(lineas)
