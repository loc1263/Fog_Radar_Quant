"""Autenticación servidor-a-servidor con API key (header ``X-API-Key``).

Solo el servidor SvelteKit (BFF) conoce la key; el navegador nunca llama a
FastAPI directamente. En ``dev``/``test`` sin ``API_KEY`` configurada se deja
pasar todo (con un aviso en el log al arrancar).
"""

from __future__ import annotations

import secrets
from typing import Annotated

from fastapi import Depends, HTTPException, Security, status
from fastapi.security import APIKeyHeader

from fog_radar.settings import Settings, get_settings

API_KEY_HEADER = "X-API-Key"
_api_key_header = APIKeyHeader(name=API_KEY_HEADER, auto_error=False)


def require_api_key(
    provided: Annotated[str | None, Security(_api_key_header)],
    settings: Annotated[Settings, Depends(get_settings)],
) -> None:
    expected = settings.api_key
    if expected is None:
        # Solo posible fuera de prod (Settings lo valida).
        return
    # En bytes: compare_digest con str lanza TypeError si hay caracteres no ASCII (→ 500).
    if provided is None or not secrets.compare_digest(
        provided.encode("utf-8"), expected.get_secret_value().encode("utf-8")
    ):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="API key inválida o ausente.",
            headers={"WWW-Authenticate": "ApiKey"},
        )
