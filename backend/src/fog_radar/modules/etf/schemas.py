"""Modelos de la API del módulo ETF (``/api/etf``).

En disco los CSV conservan los nombres de columna del original
(``Performance 5D (%)``, ``Sharpe Ratio``…); la API los expone en snake_case
estable (catálogo en :data:`fog_radar.modules.etf.service.COLUMNAS`). Todos los
modelos llevan el prefijo ``Etf`` porque el OpenAPI es uno solo para el sitio.
"""

from __future__ import annotations

from datetime import date, datetime

from pydantic import Field

from fog_radar.core.jobs import JobRecord
from fog_radar.core.schemas import Out


class EtfFila(Out):
    """Un ETF en cualquiera de las tablas (cada tabla trae solo algunos campos)."""

    ticker: str
    nombre: str | None = None
    categoria: str | None = None
    proveedor: str | None = None
    precio: float | None = None
    volumen: float | None = None
    aum: float | None = None
    expense_ratio: float | None = None
    # Rendimientos (%, en puntos: 12.3 = +12,3%)
    perf_5d: float | None = None
    perf_1m: float | None = None
    perf_6m: float | None = None
    perf_ytd: float | None = None
    perf_1y: float | None = None
    perf_5y: float | None = None
    max_52s: float | None = None
    min_52s: float | None = None
    # Riesgo (provistas por el screener)
    sharpe: float | None = None
    treynor: float | None = None
    beta: float | None = None
    # Ranking principal: variables derivadas, percentiles (0-100) y score
    dist_max_52s_pct: float | None = None
    rango_52s_pct: float | None = None
    periodos_positivos: float | None = None
    dispersion_periodos: float | None = None
    pasos_crecientes: float | None = None
    s_sharpe: float | None = None
    s_treynor: float | None = None
    s_1y: float | None = None
    s_6m: float | None = None
    s_consistencia: float | None = None
    s_tendencia: float | None = None
    s_baja_dispersion: float | None = None
    s_bajo_beta: float | None = None
    s_cerca_maximo: float | None = None
    score: float | None = None
    # Top 15 Perf. + Beta
    ss_5d: float | None = None
    ss_1m: float | None = None
    ss_6m: float | None = None
    ss_1y: float | None = None
    ss_bajo_beta: float | None = None
    score_simple: float | None = None
    # Excluidos por outlier
    ventana_atipica: str | None = None
    z_score: float | None = None
    # Momentum / crecimiento
    m_norm: float | None = None
    q_norm: float | None = None
    consist_norm: float | None = None
    score_comp: float | None = None


class EtfTabla(Out):
    fecha: date | None = None
    filas: list[EtfFila]


class EtfRotacionFila(EtfFila):
    racha: int | None = None
    rank_actual: int | None = None
    rank_previo: int | None = None
    delta_score: float | None = None
    delta_rank: int | None = None
    score_previo: float | None = None
    procedencia: str | None = None
    destino: str | None = None


class EtfRotacion(Out):
    disponible: bool
    """False si todavía no hay una corrida archivada con la cual comparar."""
    fecha: date | None = None
    fecha_previa: date | None = None
    mantienen: list[EtfRotacionFila] = []
    nuevos: list[EtfRotacionFila] = []
    salidas: list[EtfRotacionFila] = []


class EtfDiversificacionFila(Out):
    categoria: str
    conteo: int
    pct: float


class EtfDiversificacion(Out):
    fecha: date | None = None
    filas: list[EtfDiversificacionFila]


class EtfCategoriaFila(Out):
    categoria: str
    n: int
    sharpe_promedio: float | None = None
    treynor_mediana: float | None = None
    beta_promedio: float | None = None
    perf_1y_promedio: float | None = None
    perf_6m_promedio: float | None = None
    score_promedio: float | None = None
    pct_positivos_1y: float | None = None
    tickers: list[str] = []
    """Tickers de la categoría, ordenados por Sharpe descendente."""


class EtfCategorias(Out):
    fecha: date | None = None
    min_n_categoria: int
    filas: list[EtfCategoriaFila]


class EtfCapFila(EtfFila):
    cap_bucket: str | None = None
    comp_t12t1_aprox: float | None = None
    comp_t7t1_aprox: float | None = None
    alpha_capm_aprox: float | None = None
    comp1_norm: float | None = None
    comp2_norm: float | None = None
    comp3_norm: float | None = None
    momentum_score: float | None = None
    peso_portfolio: float | None = None


class EtfCapBucket(Out):
    cap_bucket: str
    n_universo: int
    aum_total_bucket: float | None = None
    n_seleccionados: int
    aum_seleccionado: float | None = None
    pct_aum_cubierto: float | None = None


class EtfCap(Out):
    fecha: date | None = None
    benchmark_ticker: str | None = None
    benchmark_retorno_1y: float | None = None
    filas: list[EtfCapFila]
    buckets: list[EtfCapBucket]


class EtfHistorialFila(Out):
    fecha: date
    universo_bruto: int
    universo_depurado: int
    n_outliers: int
    sharpe_promedio: float | None = None
    beta_promedio: float | None = None
    top1_ticker: str | None = None
    top1_score: float | None = None


class EtfHistorial(Out):
    filas: list[EtfHistorialFila]
    """De la corrida más antigua a la más reciente."""


class EtfPeso(Out):
    clave: str
    etiqueta: str
    peso: float


class EtfModelo(Out):
    """Parámetros de las metodologías (el front no los duplica)."""

    top_n: int
    pesos: list[EtfPeso]
    pesos_simple: list[EtfPeso]
    pesos_momentum: list[EtfPeso]
    pesos_momentum_m: list[EtfPeso]
    z_threshold: float
    liq_vol_min: float
    liq_aum_min: float
    categorias_excluidas: list[str]
    categorias_excluidas_momentum: list[str]
    min_n_categoria: int
    cap_n_por_bucket: int
    cap_tope_peso_pct: float
    cap_benchmarks: list[str]


class EtfResumen(Out):
    fecha: date | None = None
    archivo_csv: str
    generado_en: datetime | None = None
    tiene_corrida_anterior: bool
    resumen: EtfHistorialFila | None = None
    modelo: EtfModelo


class EtfRunResult(Out):
    job: JobRecord


# --- Historial (history.db) --------------------------------------------------------


class EtfHistCorrida(Out):
    run_key: str = Field(description="<yyyymmdd> o <yyyymmdd>_<n> (mismo sufijo que hist/).")
    fecha: date
    secuencia: int
    run_at: datetime | None = None
    universo_bruto: int | None = None
    universo_depurado: int
    n_outliers: int | None = None
    sharpe_promedio: float | None = None
    beta_promedio: float | None = None
    top1_ticker: str | None = None
    top1_score: float | None = None


class EtfHistTickerPunto(Out):
    run_key: str
    fecha: date
    universo_depurado: int
    nombre: str | None = None
    category: str | None = None
    rank: int | None = Field(default=None, description="Puesto en el ranking principal (None = no estaba).")
    en_top15: bool
    score: float | None = None
    score_simple: float | None = None
    precio: float | None = None
    sharpe: float | None = None
    treynor: float | None = None
    beta: float | None = None
    perf_1m: float | None = None
    perf_6m: float | None = None
    perf_1y: float | None = None
    momentum_rank: int | None = None
    momentum_score: float | None = None
    excluido_ventana: str | None = Field(
        default=None, description="Si fue excluido por outlier en esa corrida."
    )
    excluido_z: float | None = None


class EtfHistCategoria(Out):
    run_key: str
    fecha: date
    category: str
    n: int | None = None
    score_promedio: float | None = None
    sharpe_promedio: float | None = None
    perf_1y_promedio: float | None = None
    en_top15: int


class EtfHistPermanencia(Out):
    ticker: str
    nombre: str | None = None
    veces_en_top: int
    corridas: int
    racha_actual: int
    rank_ultima: int | None = None
    score_ultima: float | None = None
