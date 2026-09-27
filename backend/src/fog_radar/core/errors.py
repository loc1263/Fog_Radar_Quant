"""Formato de error uniforme: ``{"detail": str, "code": str}``.

El frontend (BFF) puede decidir qué mostrar según ``code`` sin parsear textos.
"""

from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from starlette.exceptions import HTTPException as StarletteHTTPException

from fog_radar.core.logging import get_logger

log = get_logger("errors")


class ErrorResponse(BaseModel):
    detail: str
    code: str


class ApiError(Exception):
    """Error de dominio con código estable (p. ej. ``no_data``, ``run_in_progress``)."""

    def __init__(self, status_code: int, code: str, detail: str) -> None:
        super().__init__(detail)
        self.status_code = status_code
        self.code = code
        self.detail = detail


class NoDataError(ApiError):
    """El módulo todavía no tiene una corrida disponible."""

    def __init__(self, detail: str = "Todavía no hay datos para este módulo.") -> None:
        super().__init__(503, "no_data", detail)


_HTTP_CODES = {
    400: "bad_request",
    401: "unauthorized",
    403: "forbidden",
    404: "not_found",
    405: "method_not_allowed",
    409: "conflict",
    429: "rate_limited",
}


def _json(status: int, code: str, detail: str, headers: dict[str, str] | None = None) -> JSONResponse:
    return JSONResponse(status_code=status, content={"detail": detail, "code": code}, headers=headers)


def register_error_handlers(app: FastAPI, *, expose_internal: bool) -> None:
    @app.exception_handler(ApiError)
    async def _api_error(_: Request, exc: ApiError) -> JSONResponse:
        return _json(exc.status_code, exc.code, exc.detail)

    @app.exception_handler(StarletteHTTPException)
    async def _http_error(_: Request, exc: StarletteHTTPException) -> JSONResponse:
        code = _HTTP_CODES.get(exc.status_code, "http_error")
        return _json(exc.status_code, code, str(exc.detail), getattr(exc, "headers", None))

    @app.exception_handler(RequestValidationError)
    async def _validation_error(_: Request, exc: RequestValidationError) -> JSONResponse:
        first = exc.errors()[0] if exc.errors() else {}
        loc = ".".join(str(p) for p in first.get("loc", []))
        msg = first.get("msg", "Parámetros inválidos")
        return _json(422, "validation_error", f"{loc}: {msg}" if loc else msg)

    @app.exception_handler(Exception)
    async def _unhandled(request: Request, exc: Exception) -> JSONResponse:
        log.exception("Error no controlado en %s %s", request.method, request.url.path)
        detail = f"{type(exc).__name__}: {exc}" if expose_internal else "Error interno del servidor."
        return _json(500, "internal_error", detail)
