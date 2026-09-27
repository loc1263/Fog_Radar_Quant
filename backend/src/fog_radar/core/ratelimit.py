"""Rate limiting como dependencia de FastAPI (librería ``limits``).

Se usa una dependencia en vez del middleware de slowapi porque éste no
reconoce las rutas incluidas con las versiones recientes de FastAPI (las deja
pasar sin límite).

- Límite por defecto (``RATE_LIMIT_DEFAULT``) en todos los endpoints de ``/api``.
- Límites estrictos en endpoints costosos::

      @router.post("/run", dependencies=[Depends(rate_limit("3/minute", scope="etf-run"))])

Con el patrón BFF todo el tráfico llega desde el servidor SvelteKit, así que
la clave es la IP del usuario final que el BFF reenvía en ``X-Forwarded-For``
(confiable: el backend solo acepta requests del servidor frontend, por
firewall + API key).

El almacenamiento es en memoria por proceso: con N workers de uvicorn el
límite efectivo es N veces el configurado. Suficiente como red de seguridad;
si hiciera falta exactitud, cambiar a ``RedisStorage``.
"""

from __future__ import annotations

from collections.abc import Callable
from typing import Annotated

from fastapi import Depends, Request
from limits import parse
from limits.storage import MemoryStorage
from limits.strategies import MovingWindowRateLimiter

from fog_radar.core.errors import ApiError
from fog_radar.settings import Settings, get_settings

_storage = MemoryStorage()
_strategy = MovingWindowRateLimiter(_storage)


def client_key(request: Request) -> str:
    forwarded = request.headers.get("x-forwarded-for")
    if forwarded:
        return forwarded.split(",")[0].strip()
    return request.client.host if request.client else "desconocido"


def _check(limit: str, scope: str, request: Request, settings: Settings) -> None:
    if settings.env == "test" and not settings.rate_limit_in_tests:
        return
    item = parse(limit)
    if not _strategy.hit(item, scope, client_key(request)):
        raise ApiError(429, "rate_limited", f"Demasiadas solicitudes (límite {limit}). Intenta más tarde.")


def rate_limit(limit: str, *, scope: str) -> Callable[..., None]:
    """Dependencia con un límite fijo para un endpoint o grupo (``scope``)."""

    def _dep(request: Request, settings: Annotated[Settings, Depends(get_settings)]) -> None:
        _check(limit, scope, request, settings)

    return _dep


def default_rate_limit(request: Request, settings: Annotated[Settings, Depends(get_settings)]) -> None:
    """Límite global configurable por ``RATE_LIMIT_DEFAULT``."""
    _check(settings.rate_limit_default, "default", request, settings)


def reset_rate_limits() -> None:
    """Solo para tests."""
    _storage.reset()
