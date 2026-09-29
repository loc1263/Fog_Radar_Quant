"""Aporte de EEUU al hub y al buscador global (última corrida terminada)."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from fog_radar.core.schemas import EntradaBusqueda, HubItem, ResumenModulo
from fog_radar.modules.us_stocks import storage
from fog_radar.settings import Settings

METRICA = "Momentum 12-1"


def resumen_hub(settings: Settings) -> ResumenModulo:
    run = storage.latest_finished_run(settings)
    result: dict[str, Any] = storage.read_run_file(run, "result.json")
    filas = result.get("rows") or []
    return ResumenModulo(
        datos_al=datetime.fromisoformat(result["finished_at"]),
        metrica=METRICA,
        unidad="fraccion",
        top=[
            HubItem(ticker=r["ticker"], nombre=r.get("name"), valor=r.get("momentum_12_1")) for r in filas[:3]
        ],
        universo=result.get("candidates_total"),
    )


def indice_busqueda(settings: Settings) -> list[EntradaBusqueda]:
    """Los candidatos evaluados, ordenados por momentum (la posición del Top 15 sale de ahí)."""
    run = storage.latest_finished_run(settings)
    top = {r["ticker"] for r in (storage.read_run_file(run, "result.json").get("rows") or [])}
    try:
        filas = storage.read_run_file(run, "candidates.json").get("rows") or []
    except Exception:  # corridas sin candidates.json: solo el Top 15
        filas = storage.read_run_file(run, "result.json").get("rows") or []
    filas = sorted(filas, key=lambda r: -(r.get("momentum_12_1") or float("-inf")))
    return [
        EntradaBusqueda(
            ticker=r["ticker"],
            nombre=r.get("name"),
            posicion=i,
            total=len(filas),
            valor=r.get("momentum_12_1"),
            metrica=METRICA,
            unidad="fraccion",
            contexto="Top 15" if r["ticker"] in top else "Candidatos",
        )
        for i, r in enumerate(filas, start=1)
    ]
