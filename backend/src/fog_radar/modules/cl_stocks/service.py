"""Transforma los CSV del módulo en las respuestas de la API."""

from __future__ import annotations

from typing import Any

import pandas as pd

from fog_radar.modules.cl_stocks import history, storage
from fog_radar.modules.cl_stocks.columnas import COLUMNAS, GRUPOS
from fog_radar.modules.cl_stocks.schemas import (
    ColumnaInfo,
    IndicadoresResponse,
    IndicadorRow,
    ModeloInfo,
    PilarInfo,
    PilarScores,
    RankingMetricas,
    RankingResponse,
    RankingRow,
)
from fog_radar.modules.cl_stocks.scoring import (
    METRICS,
    PESOS_PILARES,
    PILARES,
    WINSOR_PERCENTIL_INFERIOR,
    WINSOR_PERCENTIL_SUPERIOR,
)
from fog_radar.settings import Settings

PILAR_KEYS: dict[str, str] = {
    "Valor": "valor",
    "Calidad": "calidad",
    "Crecimiento": "crecimiento",
    "Salud Financiera": "salud_financiera",
    "Dividendo": "dividendo",
    "Momentum": "momentum",
}

# columna del CSV -> campo de RankingMetricas
METRICA_KEYS: dict[str, str] = {
    "Trailing P/E": "trailing_pe",
    "Price/Book": "price_book",
    "Price/Sales": "price_sales",
    "Enterprise Value/EBITDA": "ev_ebitda",
    "Enterprise Value/Revenue": "ev_revenue",
    "Profit Margin": "profit_margin",
    "Operating Margin (ttm)": "operating_margin",
    "Return on Assets (ttm)": "roa",
    "Return on Equity (ttm)": "roe",
    "Quarterly Revenue Growth (yoy)": "revenue_growth_q",
    "Quarterly Earnings Growth (yoy)": "earnings_growth_q",
    "Total Debt/Equity (mrq)": "debt_equity",
    "Current Ratio (mrq)": "current_ratio",
    "Forward Annual Dividend Yield (%)": "dividend_yield",
    "Retorno 5D": "ret_5d",
    "Retorno 1M": "ret_1m",
    "Retorno 3M": "ret_3m",
    "Retorno 6M": "ret_6m",
    "Retorno YTD": "ret_ytd",
    "Retorno 1A": "ret_1a",
}


def _v(value: Any) -> Any:
    """NaN → None; escalares numpy → tipos nativos de Python."""
    if value is None:
        return None
    try:
        if pd.isna(value):
            return None
    except (TypeError, ValueError):
        pass
    return value.item() if hasattr(value, "item") else value


def _float(value: Any) -> float | None:
    v = _v(value)
    return None if v is None else float(v)


def _int(value: Any) -> int | None:
    v = _v(value)
    return None if v is None else int(v)


def modelo_info() -> ModeloInfo:
    return ModeloInfo(
        pilares=[
            PilarInfo(
                key=PILAR_KEYS[p],
                label=p,
                peso=PESOS_PILARES[p],
                metricas=[METRICA_KEYS[m.columna] for m in METRICS if m.pilar == p],
            )
            for p in PILARES
        ],
        winsor=(WINSOR_PERCENTIL_INFERIOR, WINSOR_PERCENTIL_SUPERIOR),
    )


def _fila_ranking(r: dict[str, Any]) -> RankingRow:
    return RankingRow(
        ranking=_int(r.get("Ranking")),
        simbolo=str(r["Símbolo"]),
        nombre=_v(r.get("Nombre corto")),
        sector=_v(r.get("Sector")),
        industria=_v(r.get("Industria")),
        score_compuesto=_float(r.get("Score Compuesto")),
        pilares=PilarScores(**{k: _float(r.get(f"Score {p}")) for p, k in PILAR_KEYS.items()}),
        metricas_disponibles=_int(r.get("Métricas Disponibles")) or 0,
        metricas_totales=_int(r.get("Métricas Totales")) or 0,
        grupo_duplicado=_int(r.get("Grupo Financiero Duplicado")),
        metricas=RankingMetricas(**{k: _float(r.get(c)) for c, k in METRICA_KEYS.items()}),
    )


def ranking(settings: Settings) -> RankingResponse:
    loaded = storage.cargar_ranking(settings)
    return RankingResponse(
        generado_desde=loaded.path.name,
        generado_en=loaded.generado_en,
        modelo=modelo_info(),
        filas=[_fila_ranking(r) for r in loaded.df.to_dict(orient="records")],
    )


def indicadores(settings: Settings) -> IndicadoresResponse:
    loaded = storage.cargar_indicadores(settings)
    presentes = [c for c in COLUMNAS if c.csv in loaded.df.columns]
    filas = [
        IndicadorRow(
            simbolo=str(r["Símbolo"]),
            nombre=_v(r.get("Nombre corto")),
            sector=_v(r.get("Sector")),
            industria=_v(r.get("Industria")),
            valores={c.key: _v(r.get(c.csv)) for c in presentes},
        )
        for r in loaded.df.to_dict(orient="records")
    ]
    grupos = [g for g in GRUPOS if any(c.grupo == g for c in presentes)]
    return IndicadoresResponse(
        generado_desde=loaded.path.name,
        generado_en=loaded.generado_en,
        grupos=grupos,
        columnas=[
            ColumnaInfo(key=c.key, label=c.label, grupo=c.grupo, unidad=c.unidad, descripcion=c.descripcion)
            for c in presentes
        ],
        filas=filas,
    )


def historia_ticker(settings: Settings, ticker: str) -> list[dict[str, Any]]:
    """Filas de ``history.historia_ticker`` con los pilares agrupados (mismo shape que ``RankingRow``)."""
    return [
        {
            **{k: v for k, v in f.items() if k not in history.PILARES_SQL.values()},
            "pilares": {
                PILAR_KEYS[csv.removeprefix("Score ")]: f[col] for csv, col in history.PILARES_SQL.items()
            },
        }
        for f in history.historia_ticker(settings, ticker)
    ]
