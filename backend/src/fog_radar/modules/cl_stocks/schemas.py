"""Modelos de respuesta del módulo Chile (IPSA)."""

from __future__ import annotations

from datetime import datetime
from typing import Literal

from pydantic import BaseModel, Field

from fog_radar.modules.cl_stocks.columnas import Unidad


class PilarScores(BaseModel):
    """z-score promedio de cada pilar (None si la acción no tiene métricas en ese pilar)."""

    valor: float | None
    calidad: float | None
    crecimiento: float | None
    salud_financiera: float | None
    dividendo: float | None
    momentum: float | None


class RankingMetricas(BaseModel):
    """Métricas crudas que alimentan el score (mismas unidades que el CSV)."""

    trailing_pe: float | None
    price_book: float | None
    price_sales: float | None
    ev_ebitda: float | None
    ev_revenue: float | None
    profit_margin: float | None
    operating_margin: float | None
    roa: float | None
    roe: float | None
    revenue_growth_q: float | None
    earnings_growth_q: float | None
    debt_equity: float | None
    current_ratio: float | None
    dividend_yield: float | None = Field(description="En puntos porcentuales (5.1 = 5,1 %).")
    ret_5d: float | None
    ret_1m: float | None
    ret_3m: float | None
    ret_6m: float | None
    ret_ytd: float | None
    ret_1a: float | None


class RankingRow(BaseModel):
    ranking: int | None
    simbolo: str
    nombre: str | None
    sector: str | None
    industria: str | None
    score_compuesto: float | None
    pilares: PilarScores
    metricas_disponibles: int
    metricas_totales: int
    grupo_duplicado: int | None = Field(
        description="Mismo id = estados financieros casi idénticos (holding/filial): riesgo de doble conteo."
    )
    metricas: RankingMetricas


PilarKey = Literal["valor", "calidad", "crecimiento", "salud_financiera", "dividendo", "momentum"]


class PilarInfo(BaseModel):
    key: PilarKey
    label: str
    peso: float
    metricas: list[str]


class ModeloInfo(BaseModel):
    """Parámetros del modelo con que se generó el ranking (punto de partida de los sliders)."""

    pilares: list[PilarInfo]
    winsor: tuple[float, float]


class RankingResponse(BaseModel):
    generado_desde: str
    generado_en: datetime | None
    modelo: ModeloInfo
    filas: list[RankingRow]


class ColumnaInfo(BaseModel):
    key: str
    label: str
    grupo: str
    unidad: Unidad
    descripcion: str | None = None


class IndicadorRow(BaseModel):
    simbolo: str
    nombre: str | None
    sector: str | None
    industria: str | None
    valores: dict[str, float | str | None]


class IndicadoresResponse(BaseModel):
    generado_desde: str
    generado_en: datetime | None
    grupos: list[str]
    columnas: list[ColumnaInfo]
    filas: list[IndicadorRow]
