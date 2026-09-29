"""Punto de entrada de la API.

Dev::

    uv run uvicorn fog_radar.main:app --reload --port 8000

La app se construye con ``create_app()`` para poder crear instancias con
settings distintos en los tests.
"""

from __future__ import annotations

import time
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager

from fastapi import Depends, FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware

from fog_radar.api.system import APP_VERSION, protected_router, public_router
from fog_radar.api.transversal import router as transversal_router
from fog_radar.core import yahoo
from fog_radar.core.errors import ErrorResponse, register_error_handlers
from fog_radar.core.logging import configure_logging, get_logger
from fog_radar.core.ratelimit import default_rate_limit
from fog_radar.core.security import API_KEY_HEADER, require_api_key
from fog_radar.modules.cl_stocks.router import router as cl_router
from fog_radar.modules.etf.router import router as etf_router
from fog_radar.modules.tendencia.router import router as tendencia_router
from fog_radar.modules.us_stocks.router import router as us_router
from fog_radar.settings import Settings, get_settings

log = get_logger("main")

MODULE_ROUTERS = (cl_router, us_router, etf_router, tendencia_router)

_ERROR_RESPONSES = {
    401: {"model": ErrorResponse, "description": "API key inválida o ausente"},
    503: {"model": ErrorResponse, "description": "Sin datos disponibles"},
}


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or get_settings()
    configure_logging(settings.log_level)

    @asynccontextmanager
    async def lifespan(_: FastAPI) -> AsyncIterator[None]:
        settings.data_dir.mkdir(parents=True, exist_ok=True)
        if settings.env != "test":
            # Evaluaciones en vivo (/api/us/tickers/…/evaluate) y re-análisis usan la sesión común.
            yahoo.instalar(settings)
        if settings.api_key is None:
            log.warning("API_KEY no configurada: la API acepta requests sin autenticación (solo dev/test).")
        log.info(
            "%s %s arrancando (env=%s, data=%s)",
            settings.app_name,
            APP_VERSION,
            settings.env,
            settings.data_dir,
        )
        yield

    show_docs = not settings.is_prod
    app = FastAPI(
        title=settings.app_name,
        version=APP_VERSION,
        lifespan=lifespan,
        docs_url="/docs" if show_docs else None,
        redoc_url=None,
        openapi_url="/openapi.json" if show_docs else None,
    )

    # Los tests pasan su propio Settings: que todas las dependencias lo usen.
    app.dependency_overrides[get_settings] = lambda: settings

    register_error_handlers(app, expose_internal=not settings.is_prod)
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.allowed_origins,
        allow_credentials=False,
        allow_methods=["GET", "POST"],
        allow_headers=[API_KEY_HEADER, "Content-Type"],
    )

    @app.middleware("http")
    async def _timing(request: Request, call_next: Callable[[Request], Awaitable[Response]]) -> Response:
        t0 = time.perf_counter()
        response = await call_next(request)
        ms = (time.perf_counter() - t0) * 1000
        response.headers["X-Process-Time-ms"] = f"{ms:.1f}"
        log.debug("%s %s -> %s (%.1f ms)", request.method, request.url.path, response.status_code, ms)
        return response

    limited = [Depends(default_rate_limit)]
    app.include_router(public_router, prefix="/api", dependencies=limited)
    # El límite va antes de la autenticación: también frena intentos con claves inválidas.
    protected = [*limited, Depends(require_api_key)]
    app.include_router(protected_router, prefix="/api", dependencies=protected, responses=_ERROR_RESPONSES)
    app.include_router(transversal_router, prefix="/api", dependencies=protected, responses=_ERROR_RESPONSES)
    for router in MODULE_ROUTERS:
        app.include_router(router, prefix="/api", dependencies=protected, responses=_ERROR_RESPONSES)

    return app


app = create_app()
