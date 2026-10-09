"""Modelos de respuesta compartidos."""

from __future__ import annotations

from datetime import datetime
from typing import Annotated, Literal

from pydantic import AfterValidator, BaseModel, ConfigDict

from fog_radar.core.jobs import JobRecord


def _con_zona(d: datetime) -> datetime:
    """Fechas sin zona (``datetime.now().isoformat()`` de los pipelines portados) se
    interpretan en ``Settings.timezone``: la API siempre publica el offset, así el SSR
    y el navegador muestran la misma hora."""
    if d.tzinfo is not None:
        return d
    from fog_radar.settings import get_settings

    return d.replace(tzinfo=get_settings().tzinfo)


FechaLocal = Annotated[datetime, AfterValidator(_con_zona)]
"""``datetime`` que siempre sale con zona horaria (ver :func:`_con_zona`)."""


class Out(BaseModel):
    """Base de las respuestas de los módulos: los campos con valor por defecto se
    publican como requeridos en el OpenAPI (siempre vienen en la respuesta), así los
    tipos TS generados no los marcan como opcionales (ADR 0002)."""

    model_config = ConfigDict(json_schema_serialization_defaults_required=True)


class HealthResponse(BaseModel):
    status: Literal["ok"] = "ok"
    app: str
    version: str
    env: str


class ModuleStatus(BaseModel):
    key: str
    title: str
    api_prefix: str
    source_project: str
    migrated: bool
    """True cuando el pipeline del módulo ya está portado a Fog Radar."""
    migration_phase: int
    data_available: bool
    latest_job: JobRecord | None = None
    last_success: JobRecord | None = None


class ProgramaInfo(BaseModel):
    """Una corrida programada del cron de producción (``fog_radar.jobs.schedule``)."""

    key: str
    titulo: str
    modulos: list[str]
    cron: str
    descripcion: str
    zona: str
    proxima: datetime


class SystemStatus(BaseModel):
    app: str
    version: str
    env: str
    modules: list[ModuleStatus]


# --- Transversal: hub y buscador global (Fase 7) --------------------------------

Unidad = Literal["fraccion", "numero"]
"""Cómo formatear ``valor``: ``fraccion`` → porcentaje (0.12 = 12 %), ``numero`` → tal cual."""


class HubItem(Out):
    ticker: str
    nombre: str | None = None
    valor: float | None = None


class ResumenModulo(Out):
    """Lo que cada módulo aporta al hub: fecha de sus datos y su top 3."""

    datos_al: datetime | None = None
    metrica: str
    unidad: Unidad
    top: list[HubItem]
    universo: int | None = None
    """Cuántos instrumentos cubre la última corrida (acciones, ETFs…)."""


Frescura = Literal["al_dia", "atrasado", "sin_datos", "error"]


class HubModulo(ResumenModulo):
    key: str
    titulo: str
    frescura: Frescura
    edad_horas: float | None = None
    programa: str | None = None
    """Cuándo se actualiza en producción (p. ej. "de lunes a viernes a las 18:00")."""
    proxima: datetime | None = None
    mensaje: str | None = None
    """Motivo si no hay datos o no se pudieron leer."""
    ultimo_job: JobRecord | None = None


class HubResponse(Out):
    generado_en: datetime
    modulos: list[HubModulo]


class EntradaBusqueda(Out):
    """Un instrumento en el índice de búsqueda de un módulo."""

    ticker: str
    nombre: str | None = None
    posicion: int | None = None
    """Posición en el ranking del módulo (1 = mejor), si aplica."""
    total: int | None = None
    valor: float | None = None
    metrica: str
    unidad: Unidad
    contexto: str | None = None
    """Dónde aparece (p. ej. "Top 15", "Portafolio Asia + EEUU")."""


class ResultadoBusqueda(EntradaBusqueda):
    modulo: str
    titulo_modulo: str
    exacto: bool
    """El ticker coincide exactamente con la consulta (va primero)."""


class BusquedaResponse(Out):
    consulta: str
    resultados: list[ResultadoBusqueda]


# --- Historial (history.db de cada módulo, ver core.histdb) ----------------------

EstadoDiff = Literal["nuevo", "salio", "sigue", "fuera"]


class HistDiffFila(Out):
    """Un ticker comparado entre dos corridas. ``estado`` se mide contra el top
    (``top_n``) si el módulo tiene uno, o contra la presencia en el ranking si no:
    ``nuevo`` (entró), ``salio``, ``sigue`` (en ambas) o ``fuera`` (en ninguna de
    las dos dentro del top, pero sí en el ranking)."""

    ticker: str
    nombre: str | None = None
    rank_desde: int | None = None
    rank_hasta: int | None = None
    score_desde: float | None = None
    score_hasta: float | None = None
    delta_rank: int | None = None
    """Positivo = subió posiciones."""
    delta_score: float | None = None
    estado: EstadoDiff


class HistDiff(Out):
    desde: str
    hasta: str
    top_n: int | None = None
    filas: list[HistDiffFila]
