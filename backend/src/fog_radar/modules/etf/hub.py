"""Aporte de ETFs al hub y al buscador global."""

from __future__ import annotations

from fog_radar.core.errors import NoDataError
from fog_radar.core.schemas import EntradaBusqueda, HubItem, ResumenModulo
from fog_radar.modules.etf import service, storage
from fog_radar.settings import Settings

METRICA = "Score"


def resumen_hub(settings: Settings) -> ResumenModulo:
    top = service.tabla(settings, storage.TOP15)
    with storage.bloqueo_datos(settings):
        generado = storage.generado_en(settings)
        ranking = storage.leer(settings, storage.RANKING)
    return ResumenModulo(
        datos_al=generado,
        metrica=METRICA,
        unidad="numero",
        top=[HubItem(ticker=f.ticker, nombre=f.nombre, valor=f.score) for f in top.filas[:3]],
        universo=None if ranking is None else len(ranking),
    )


def indice_busqueda(settings: Settings) -> list[EntradaBusqueda]:
    """Universo depurado, en el orden del ranking principal; marca los del Top 15."""
    try:
        ranking = service.tabla(settings, storage.RANKING)
    except NoDataError:
        return []
    top15 = {f.ticker for f in service.tabla(settings, storage.TOP15).filas}
    filas = sorted(ranking.filas, key=lambda f: -(f.score if f.score is not None else float("-inf")))
    return [
        EntradaBusqueda(
            ticker=f.ticker,
            nombre=f.nombre,
            posicion=i,
            total=len(filas),
            valor=f.score,
            metrica=METRICA,
            unidad="numero",
            contexto=("Top 15 · " if f.ticker in top15 else "") + (f.categoria or "Universo"),
        )
        for i, f in enumerate(filas, start=1)
    ]
