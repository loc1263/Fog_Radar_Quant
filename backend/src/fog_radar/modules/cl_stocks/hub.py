"""Aporte de Chile al hub y al buscador global."""

from __future__ import annotations

from fog_radar.core.schemas import EntradaBusqueda, HubItem, ResumenModulo
from fog_radar.modules.cl_stocks import service
from fog_radar.modules.cl_stocks.schemas import RankingResponse, RankingRow
from fog_radar.settings import Settings

METRICA = "Score compuesto"


def _filas(settings: Settings) -> tuple[RankingResponse, list[RankingRow]]:
    r = service.ranking(settings)
    filas = sorted(r.filas, key=lambda f: (f.ranking is None, f.ranking or 0))
    return r, filas


def resumen_hub(settings: Settings) -> ResumenModulo:
    r, filas = _filas(settings)
    return ResumenModulo(
        datos_al=r.generado_en,
        metrica=METRICA,
        unidad="numero",
        top=[HubItem(ticker=f.simbolo, nombre=f.nombre, valor=f.score_compuesto) for f in filas[:3]],
        universo=len(filas),
    )


def indice_busqueda(settings: Settings) -> list[EntradaBusqueda]:
    _, filas = _filas(settings)
    return [
        EntradaBusqueda(
            ticker=f.simbolo,
            nombre=f.nombre,
            posicion=f.ranking,
            total=len(filas),
            valor=f.score_compuesto,
            metrica=METRICA,
            unidad="numero",
            contexto="Ranking IPSA",
        )
        for f in filas
    ]
