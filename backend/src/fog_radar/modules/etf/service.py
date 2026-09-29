"""Transforma los CSV del módulo ETF en las respuestas de la API.

Port de los endpoints de ``ETF_Choice/WEB/backend/main.py``: misma selección
de archivos y mismos cálculos al vuelo (Top 15 Vanguard, diversificación,
rotación), con claves snake_case en vez de los nombres de columna del CSV.
"""

from __future__ import annotations

import math
from typing import Any

import pandas as pd

from fog_radar.core.errors import NoDataError
from fog_radar.modules.etf import storage
from fog_radar.modules.etf.analysis import (
    config as cfg,
)
from fog_radar.modules.etf.analysis import (
    etf_categoria_analysis,
    etf_momentum_analysis,
    etf_ranking_analysis,
    etf_russell3000_momentum,
    rotacion,
)
from fog_radar.modules.etf.schemas import (
    EtfCap,
    EtfCategorias,
    EtfDiversificacion,
    EtfHistorial,
    EtfModelo,
    EtfPeso,
    EtfResumen,
    EtfRotacion,
    EtfTabla,
)
from fog_radar.settings import Settings

# Columna del CSV (formato del original) → clave de la API.
COLUMNAS: dict[str, str] = {
    "Ticker": "ticker",
    "Nombre": "nombre",
    "Category": "categoria",
    "Proveedor": "proveedor",
    "Precio": "precio",
    "Volumen": "volumen",
    "AUM": "aum",
    "Expense Ratio": "expense_ratio",
    "Performance 5D (%)": "perf_5d",
    "Performance 1M (%)": "perf_1m",
    "Performance 6M (%)": "perf_6m",
    "Performance YTD (%)": "perf_ytd",
    "Performance 1Y (%)": "perf_1y",
    "Performance 5Y (%)": "perf_5y",
    "52W High": "max_52s",
    "52W Low": "min_52s",
    "Sharpe Ratio": "sharpe",
    "Treynor Ratio": "treynor",
    "Beta": "beta",
    "Dist_52W_High_%": "dist_max_52s_pct",
    "Rango_52W_%": "rango_52s_pct",
    "Periodos_Positivos": "periodos_positivos",
    "Dispersion_Periodos": "dispersion_periodos",
    "Pasos_Crecientes": "pasos_crecientes",
    "Score": "score",
    "Score_Simple": "score_simple",
    "Ventana_Atipica": "ventana_atipica",
    "Z_Score": "z_score",
    "M_norm": "m_norm",
    "Q_norm": "q_norm",
    "Consist_norm": "consist_norm",
    "Score_comp": "score_comp",
    "Cap_Bucket": "cap_bucket",
    "Comp_T12T1_aprox": "comp_t12t1_aprox",
    "Comp_T7T1_aprox": "comp_t7t1_aprox",
    "Alpha_CAPM_aprox": "alpha_capm_aprox",
    "Comp1_norm": "comp1_norm",
    "Comp2_norm": "comp2_norm",
    "Comp3_norm": "comp3_norm",
    "Momentum_Score": "momentum_score",
    "Peso_Portfolio": "peso_portfolio",
    # Rotación
    "Racha": "racha",
    "Rank_actual": "rank_actual",
    "Rank_previo": "rank_previo",
    "Delta_Score": "delta_score",
    "Delta_Rank": "delta_rank",
    "Score_previo": "score_previo",
    "Procedencia": "procedencia",
    "Destino": "destino",
    # Diversificación / categorías / buckets / historial
    "Conteo": "conteo",
    "Pct": "pct",
    "N": "n",
    "Sharpe_Promedio": "sharpe_promedio",
    "Treynor_Mediana": "treynor_mediana",
    "Beta_Promedio": "beta_promedio",
    "Perf_1Y_Promedio": "perf_1y_promedio",
    "Perf_6M_Promedio": "perf_6m_promedio",
    "Score_Promedio": "score_promedio",
    "Pct_Positivos_1Y": "pct_positivos_1y",
    "Tickers": "tickers",
    "N_Universo": "n_universo",
    "AUM_Total_Bucket": "aum_total_bucket",
    "N_Seleccionados": "n_seleccionados",
    "AUM_Seleccionado": "aum_seleccionado",
    "Pct_AUM_Cubierto": "pct_aum_cubierto",
    "Universo_Bruto": "universo_bruto",
    "Universo_Depurado": "universo_depurado",
    "N_Outliers": "n_outliers",
    "Top1_Ticker": "top1_ticker",
    "Top1_Score": "top1_score",
}
# Percentiles de los componentes (s_*, ss_*) ya vienen en snake_case.
_PASAN_IGUAL = {k for k in etf_ranking_analysis.WEIGHTS} | {
    "ss_5d",
    "ss_1m",
    "ss_6m",
    "ss_1y",
    "ss_bajo_beta",
}
_TEXTO = {
    "ticker",
    "nombre",
    "categoria",
    "proveedor",
    "ventana_atipica",
    "procedencia",
    "destino",
    "cap_bucket",
    "top1_ticker",
    "tickers",
}


def _limpio(v: Any) -> Any:
    """NaN/±inf → None; numpy → tipos nativos (JSON estricto)."""
    if v is None:
        return None
    if isinstance(v, float) and (math.isnan(v) or math.isinf(v)):
        return None
    if hasattr(v, "item"):  # numpy scalar
        return _limpio(v.item())
    return v


def registros(df: pd.DataFrame | None) -> list[dict[str, Any]]:
    """Filas del CSV con claves de la API (columnas desconocidas se descartan)."""
    if df is None or len(df) == 0:
        return []
    salida: list[dict[str, Any]] = []
    for fila in df.to_dict("records"):
        r: dict[str, Any] = {}
        for col, v in fila.items():
            clave = COLUMNAS.get(str(col)) or (str(col) if col in _PASAN_IGUAL else None)
            if clave is None:
                continue
            v = _limpio(v)
            if clave not in _TEXTO and isinstance(v, str):
                v = pd.to_numeric(v, errors="coerce")  # p. ej. "N/A" del screener
                v = _limpio(v)
            elif clave in _TEXTO and v is not None:
                v = str(v)
            r[clave] = v
        salida.append(r)
    return salida


def _fecha(df: pd.DataFrame | None) -> Any:
    if df is None or len(df) == 0 or "Fecha" not in df.columns:
        return None
    return storage.fecha_de(str(df["Fecha"].iloc[0]).split(".")[0])


def _tabla(df: pd.DataFrame) -> EtfTabla:
    return EtfTabla.model_validate({"fecha": _fecha(df), "filas": registros(df)})


# --- Endpoints -----------------------------------------------------------------
def modelo() -> EtfModelo:
    def pesos(d: dict[str, float], etiquetas: dict[str, str]) -> list[EtfPeso]:
        return [EtfPeso(clave=k, etiqueta=etiquetas.get(k, k), peso=v) for k, v in d.items()]

    etiquetas = {
        "s_sharpe": "Sharpe Ratio",
        "s_treynor": "Treynor Ratio",
        "s_1y": "Rendimiento 1Y",
        "s_6m": "Rendimiento 6M",
        "s_consistencia": "Consistencia (períodos positivos)",
        "s_tendencia": "Tendencia (pasos crecientes)",
        "s_baja_dispersion": "Baja dispersión entre períodos",
        "s_bajo_beta": "Bajo |Beta|",
        "s_cerca_maximo": "Cercanía al máximo 52S",
        "ss_5d": "Rendimiento 5D",
        "ss_1m": "Rendimiento 1M",
        "ss_6m": "Rendimiento 6M",
        "ss_1y": "Rendimiento 1Y",
        "ss_bajo_beta": "Bajo |Beta|",
    }
    # SIMPLE_WEIGHTS es local a run() en el original: 5 factores al 20 %.
    simple = dict.fromkeys(("ss_5d", "ss_1m", "ss_6m", "ss_1y", "ss_bajo_beta"), 0.20)
    m = etf_momentum_analysis
    r3 = etf_russell3000_momentum
    return EtfModelo(
        top_n=cfg.TOP_N,
        pesos=pesos(etf_ranking_analysis.WEIGHTS, etiquetas),
        pesos_simple=pesos(simple, etiquetas),
        pesos_momentum=[
            EtfPeso(clave="m_norm", etiqueta="Momentum (M)", peso=0.55),
            EtfPeso(clave="q_norm", etiqueta="Calidad Sharpe/Treynor (Q)", peso=0.25),
            EtfPeso(clave="consist_norm", etiqueta="Consistencia 6M vs. corrida más antigua", peso=0.20),
        ],
        pesos_momentum_m=[
            EtfPeso(clave="perf_5d", etiqueta="5D", peso=m.PESO_M_5D),
            EtfPeso(clave="perf_1m", etiqueta="1M", peso=m.PESO_M_1M),
            EtfPeso(clave="perf_6m", etiqueta="6M", peso=m.PESO_M_6M),
            EtfPeso(clave="perf_1y", etiqueta="1Y", peso=m.PESO_M_1Y),
        ],
        z_threshold=etf_ranking_analysis.Z_THRESHOLD,
        liq_vol_min=etf_ranking_analysis.LIQ_VOL_MIN,
        liq_aum_min=etf_ranking_analysis.LIQ_AUM_MIN,
        categorias_excluidas=list(etf_ranking_analysis.CATEGORIAS_EXCLUIR),
        categorias_excluidas_momentum=list(m.CATEGORIAS_EXCLUIR),
        min_n_categoria=etf_categoria_analysis.MIN_N_CATEGORIA,
        cap_n_por_bucket=r3.N_TOP_BUCKET,
        cap_tope_peso_pct=r3.TOPE_PESO * 100,
        cap_benchmarks=list(r3.BENCHMARK_CANDIDATOS),
    )


def resumen(settings: Settings) -> EtfResumen:
    with storage.bloqueo_datos(settings):
        csv = storage.csv_actual(storage.current_dir(settings))
        if csv is None:
            raise NoDataError(storage.SIN_CORRIDA)
        historial = storage.leer(settings, storage.HISTORIAL)
        generado = storage.generado_en(settings)
    fila = None
    if historial is not None and len(historial):
        fila = registros(historial.sort_values("Fecha").tail(1))[0]
        fila["fecha"] = storage.fecha_de(str(historial.sort_values("Fecha")["Fecha"].iloc[-1]))
    return EtfResumen.model_validate(
        {
            "fecha": storage.fecha_de(csv.name),
            "archivo_csv": csv.name,
            "generado_en": generado,
            "tiene_corrida_anterior": storage.hay_corrida_anterior(settings),
            "resumen": fila,
            "modelo": modelo(),
        }
    )


def tabla(settings: Settings, nombre: str, detalle: str = storage.SIN_CORRIDA) -> EtfTabla:
    with storage.bloqueo_datos(settings):
        df = storage.leer_requerido(settings, nombre, detalle)
    return _tabla(df)


def top15_vanguard(settings: Settings) -> EtfTabla:
    """Ranking principal filtrado a Proveedor == "Vanguard" antes de cortar al Top N."""
    with storage.bloqueo_datos(settings):
        df = storage.leer_requerido(settings, storage.RANKING)
    vanguard = df[df["Proveedor"] == "Vanguard"].sort_values("Score", ascending=False).head(cfg.TOP_N)
    return EtfTabla.model_validate({"fecha": _fecha(df), "filas": registros(vanguard)})


def diversificacion(settings: Settings) -> EtfDiversificacion:
    with storage.bloqueo_datos(settings):
        top15 = storage.leer_requerido(settings, storage.TOP15)
    return EtfDiversificacion.model_validate(
        {"fecha": _fecha(top15), "filas": registros(rotacion.diversificacion_top15(top15))}
    )


def rotacion_vs_anterior(settings: Settings) -> EtfRotacion:
    with storage.bloqueo_datos(settings):
        top15 = storage.leer_requerido(settings, storage.TOP15)
        ranking = storage.leer_requerido(settings, storage.RANKING)
        excl = storage.leer_requerido(settings, storage.EXCLUIDOS)
        hist = storage.hist_dir(settings)
        resultado = rotacion.generar_datos_rotacion(
            str(storage.current_dir(settings)), str(hist), top15, excl, ranking
        )
        previo = rotacion._encontrar_archivo_previo(str(hist), "etf_top15_recomendados.csv")
    if resultado is None:
        return EtfRotacion(disponible=False, fecha=_fecha(top15))
    return EtfRotacion.model_validate(
        {
            "disponible": True,
            "fecha": _fecha(top15),
            "fecha_previa": storage.fecha_de(previo) if previo else None,
            "mantienen": registros(resultado["mantienen"]),
            "nuevos": registros(resultado["nuevos"]),
            "salidas": registros(resultado["salidas"]),
        }
    )


def categorias(settings: Settings) -> EtfCategorias:
    with storage.bloqueo_datos(settings):
        df = storage.leer_requerido(
            settings, storage.CATEGORIAS, "El análisis de Categoría no se corrió en esta corrida."
        )
    filas = registros(df)
    for f in filas:
        tickers = f.get("tickers")
        f["tickers"] = [t.strip() for t in str(tickers).split(",") if t.strip()] if tickers else []
    return EtfCategorias.model_validate(
        {"fecha": _fecha(df), "min_n_categoria": etf_categoria_analysis.MIN_N_CATEGORIA, "filas": filas}
    )


def top15_cap(settings: Settings) -> EtfCap:
    with storage.bloqueo_datos(settings):
        portfolio = storage.leer_requerido(
            settings, storage.CAP_PORTFOLIO, "El análisis Top 15 Cap. no se corrió en esta corrida."
        )
        buckets = storage.leer(settings, storage.CAP_BUCKETS)
    bench_ticker = bench_ret = None
    if buckets is not None and len(buckets):
        bench_ticker = _limpio(buckets["Benchmark_Alpha"].iloc[0]) if "Benchmark_Alpha" in buckets else None
        bench_ret = (
            _limpio(buckets["Benchmark_Retorno_1Y"].iloc[0]) if "Benchmark_Retorno_1Y" in buckets else None
        )
    elif "Benchmark_Alpha" in portfolio and len(portfolio):
        bench_ticker = _limpio(portfolio["Benchmark_Alpha"].iloc[0])
    return EtfCap.model_validate(
        {
            "fecha": _fecha(portfolio),
            "benchmark_ticker": bench_ticker,
            "benchmark_retorno_1y": bench_ret,
            "filas": registros(portfolio),
            "buckets": registros(buckets),
        }
    )


def historial(settings: Settings) -> EtfHistorial:
    with storage.bloqueo_datos(settings):
        df = storage.leer(settings, storage.HISTORIAL)
    if df is None or len(df) == 0:
        return EtfHistorial(filas=[])
    df = df.sort_values("Fecha")
    filas = registros(df)
    for f, fecha in zip(filas, df["Fecha"].tolist(), strict=True):
        f["fecha"] = storage.fecha_de(str(fecha))
    return EtfHistorial.model_validate({"filas": filas})
