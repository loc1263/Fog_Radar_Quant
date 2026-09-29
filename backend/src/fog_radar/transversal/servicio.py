"""Hub (estado + frescura + top 3 por módulo) y buscador global de ticker."""

from __future__ import annotations

import threading
import time
import unicodedata
from collections.abc import Callable
from datetime import datetime, timedelta
from importlib import import_module
from typing import Any

from fog_radar.core.errors import NoDataError
from fog_radar.core.jobs import JobRegistry
from fog_radar.core.logging import get_logger
from fog_radar.core.schemas import (
    BusquedaResponse,
    EntradaBusqueda,
    Frescura,
    HubModulo,
    HubResponse,
    ResultadoBusqueda,
    ResumenModulo,
)
from fog_radar.jobs.schedule import PROGRAMA_DE_MODULO
from fog_radar.modules import MODULES, ModuleInfo
from fog_radar.settings import Settings

log = get_logger("transversal")

# Margen para que una corrida programada termine antes de considerar atrasados sus datos
# (el ETF tarda ~30 min, la secuencia diaria ~25 min; con reintentos puede pasar de 1 h).
MARGEN_CORRIDA = timedelta(hours=3)
MAX_RESULTADOS = 50
_TTL_INDICE_S = 60.0


def _hub_de(modulo: str) -> Any:
    return import_module(f"fog_radar.modules.{modulo}.hub")


# --- Frescura ------------------------------------------------------------------
def frescura(modulo: str, datos_al: datetime | None, ahora: datetime) -> Frescura:
    """``al_dia`` si los datos son posteriores a la última corrida programada que ya
    debería haber terminado; ``atrasado`` si no. Sin programa: al día (no se espera nada)."""
    if datos_al is None:
        return "sin_datos"
    programa = PROGRAMA_DE_MODULO.get(modulo)
    if programa is None:
        return "al_dia"
    esperada = programa.anterior(ahora - MARGEN_CORRIDA)
    return "al_dia" if datos_al >= esperada else "atrasado"


def hub(settings: Settings, ahora: datetime | None = None) -> HubResponse:
    ahora = ahora or datetime.now(settings.tzinfo)
    registry = JobRegistry(settings.jobs_dir)
    return HubResponse(
        generado_en=ahora,
        modulos=[_hub_modulo(settings, m, registry, ahora) for m in MODULES],
    )


def _hub_modulo(settings: Settings, info: ModuleInfo, registry: JobRegistry, ahora: datetime) -> HubModulo:
    programa = PROGRAMA_DE_MODULO.get(info.key)
    base: dict[str, Any] = {
        "key": info.key,
        "titulo": info.title,
        "programa": programa.descripcion if programa else None,
        "proxima": programa.proxima(ahora) if programa else None,
        "ultimo_job": registry.latest(info.key),
    }
    try:
        resumen: ResumenModulo = _hub_de(info.key).resumen_hub(settings)
    except NoDataError as exc:
        return HubModulo(**base, metrica="", unidad="numero", top=[], frescura="sin_datos", mensaje=str(exc))
    except Exception as exc:  # datos corruptos: el hub nunca se cae por un módulo
        log.warning("Hub: no se pudo resumir %s: %s", info.key, exc)
        return HubModulo(
            **base,
            metrica="",
            unidad="numero",
            top=[],
            frescura="error",
            # En prod no se expone el detalle (rutas, mensajes internos): queda en el log.
            mensaje="Los datos del módulo no se pudieron leer."
            if settings.is_prod
            else f"{type(exc).__name__}: {exc}",
        )
    datos_al = resumen.datos_al.astimezone(settings.tzinfo) if resumen.datos_al else None
    edad = round((ahora - datos_al).total_seconds() / 3600, 1) if datos_al else None
    return HubModulo(
        **base,
        **resumen.model_dump(exclude={"datos_al"}),
        datos_al=datos_al,
        edad_horas=edad,
        frescura=frescura(info.key, datos_al, ahora),
    )


# --- Buscador ------------------------------------------------------------------
class _CacheIndice:
    """Índice por módulo con TTL corto: las búsquedas mientras se escribe no releen los CSV."""

    def __init__(self, ttl_s: float, clock: Callable[[], float] = time.monotonic) -> None:
        self.ttl_s = ttl_s
        self._clock = clock
        self._lock = threading.Lock()
        self._datos: dict[tuple[str, str], tuple[float, list[EntradaBusqueda]]] = {}

    def obtener(self, settings: Settings, modulo: str) -> list[EntradaBusqueda]:
        key = (str(settings.data_dir), modulo)
        ahora = self._clock()
        with self._lock:
            hit = self._datos.get(key)
            if hit and ahora - hit[0] < self.ttl_s:
                return hit[1]
        try:
            entradas = _hub_de(modulo).indice_busqueda(settings)
        except NoDataError:
            entradas = []
        except Exception as exc:
            log.warning("Buscador: índice de %s no disponible: %s", modulo, exc)
            entradas = []
        with self._lock:
            self._datos[key] = (ahora, entradas)
        return entradas

    def limpiar(self) -> None:
        with self._lock:
            self._datos.clear()


cache_indice = _CacheIndice(_TTL_INDICE_S)


def _normalizar(texto: str) -> str:
    sin_tildes = unicodedata.normalize("NFKD", texto).encode("ascii", "ignore").decode("ascii")
    return " ".join(sin_tildes.lower().split())


def _base_ticker(ticker: str) -> str:
    """``VAPORES.SN`` → ``vapores``; ``^VIX`` → ``vix`` (para buscar sin sufijo de bolsa)."""
    return _normalizar(ticker.split(".")[0].lstrip("^"))


def buscar(settings: Settings, consulta: str, limite: int = MAX_RESULTADOS) -> BusquedaResponse:
    """Busca por ticker (exacto, sin sufijo de bolsa o por prefijo) y por nombre (contiene).

    Orden: coincidencia exacta de ticker, luego prefijo de ticker, luego nombre; dentro
    de cada grupo, por módulo y posición en su ranking.
    """
    q = _normalizar(consulta)
    resultados: list[tuple[tuple[int, int, int], ResultadoBusqueda]] = []
    if not q:
        return BusquedaResponse(consulta=consulta, resultados=[])
    for orden_mod, info in enumerate(MODULES):
        for e in cache_indice.obtener(settings, info.key):
            t = _normalizar(e.ticker)
            if t == q or _base_ticker(e.ticker) == q:
                grupo = 0
            elif t.startswith(q):
                grupo = 1
            elif len(q) >= 3 and q in _normalizar(e.nombre or ""):
                grupo = 2
            else:
                continue
            res = ResultadoBusqueda(
                **e.model_dump(), modulo=info.key, titulo_modulo=info.title, exacto=grupo == 0
            )
            resultados.append(((grupo, orden_mod, e.posicion or 10**6), res))
    resultados.sort(key=lambda x: x[0])
    return BusquedaResponse(consulta=consulta, resultados=[r for _, r in resultados[:limite]])
