"""Aporte de Tendencia al hub y al buscador global (última corrida)."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from fog_radar.core.schemas import EntradaBusqueda, HubItem, ResumenModulo
from fog_radar.modules.tendencia import run_files
from fog_radar.settings import Settings

METRICA = "Peso en el portafolio"


def _ultima(settings: Settings) -> dict[str, Any]:
    return run_files.read_dashboard(run_files.latest_run(settings))


def _fecha(payload: dict[str, Any], settings: Settings) -> datetime | None:
    try:
        f = datetime.fromisoformat(payload["generated_at"])
    except (KeyError, TypeError, ValueError):
        return None
    return f if f.tzinfo else f.replace(tzinfo=settings.tzinfo)


def resumen_hub(settings: Settings) -> ResumenModulo:
    payload = _ultima(settings)
    principal = payload["analyses"][0]
    holdings = sorted(principal["holdings"], key=lambda h: -(h.get("peso_final") or 0))
    return ResumenModulo(
        datos_al=_fecha(payload, settings),
        metrica=METRICA,
        unidad="fraccion",
        top=[
            HubItem(ticker=h["ticker"], nombre=h.get("nombre"), valor=h.get("peso_final"))
            for h in holdings[:3]
        ],
        universo=principal.get("universe_size"),
    )


def indice_busqueda(settings: Settings) -> list[EntradaBusqueda]:
    """Acciones de los portafolios (cada análisis) y líderes de momentum de la última corrida."""
    payload = _ultima(settings)
    out: list[EntradaBusqueda] = []
    vistos: set[str] = set()
    for a in payload["analyses"]:
        holdings = sorted(a["holdings"], key=lambda h: -(h.get("peso_final") or 0))
        for i, h in enumerate(holdings, start=1):
            out.append(
                EntradaBusqueda(
                    ticker=h["ticker"],
                    nombre=h.get("nombre"),
                    posicion=i,
                    total=len(holdings),
                    valor=h.get("peso_final"),
                    metrica=METRICA,
                    unidad="fraccion",
                    contexto=f"Portafolio {a['label']}",
                )
            )
            vistos.add(h["ticker"])
        for m in a.get("momentum") or []:
            if m.get("ticker") in vistos:
                continue
            vistos.add(m["ticker"])
            out.append(
                EntradaBusqueda(
                    ticker=m["ticker"],
                    nombre=m.get("nombre"),
                    posicion=m.get("rank"),
                    total=None,
                    valor=(m["momentum_score_pct"] / 100)
                    if m.get("momentum_score_pct") is not None
                    else None,
                    metrica="Score de momentum",
                    unidad="fraccion",
                    contexto=f"Momentum mensual {a['label']}",
                )
            )
    return out
