"""
"Top 15 Cap." (nombre en el panel hasta 2026-08-07: "Top 15 Russell 3000",
renombrado a pedido del usuario -- ver "Cambio 2026-08-07" mas abajo) -
estrategia de Momentum Factor tipo Vanguard U.S. Momentum Factor ETF
(VFMO), adaptada a ETFs por no haber datos de acciones individuales en
este proyecto (WEB/backend/analysis/etf_russell3000_momentum.py, nombre
de archivo/modulo sin cambios pese al renombre del label en el panel).

Ver CLAUDE.md seccion 2quinquies para el detalle completo de la
metodologia y su historial de decisiones. Resumen de las adaptaciones
respecto a la regla original del usuario (pensada para acciones sueltas
del Russell 3000), todas confirmadas explicitamente por el usuario el
2026-08-07:

  - Unidad investible = ETF, no accion individual (unico dato disponible
    en este proyecto: etf_perf_data_*.csv).
  - "Capitalizacion de mercado" de cada unidad se aproxima con su AUM (el
    unico proxy de tamano de fondo disponible para un ETF).
  - Universo = ETFs de renta variable de EE.UU. categorizados por
    Morningstar en alguno de los 9 segmentos Large/Mid/Small x
    Blend/Value/Growth (CATEGORIAS_CAP_BUCKET mas abajo) -- el equivalente
    mas cercano disponible a "Russell 3000 segmentado por capitalizacion"
    que se puede armar con ETFs. Se excluyen ademas explicitamente
    Utilities y Real Estate/REIT (regla 2 del usuario), aunque hoy ninguna
    categoria de esas dos cae dentro de los 9 segmentos de cap (chequeo
    defensivo, por si la taxonomia del proveedor cambia).
  - Los 2 componentes de retorno de la regla 3 (T-12:T-1 y T-7:T-1, ambos
    "excluyendo el ultimo mes") se aproximan con Performance 1Y y
    Performance 6M tal como llegan en el CSV -- retorno trailing COMPLETO,
    SIN excluir el ultimo mes, porque el CSV no trae granularidad mensual
    para aislarlo. Aproximacion elegida por el usuario ("Aproximacion con
    datos actuales", 2026-08-07) en vez de construir un pipeline nuevo de
    precios historicos.
  - El componente de Alpha (intercepto de una regresion de retornos
    diarios/semanales de 1 año contra un benchmark regional) se aproxima
    con una formula CAPM de un solo punto, misma decision de arriba:
    Alpha_aprox = Retorno_1Y - Beta x Retorno_Benchmark_1Y, usando el Beta
    que ya trae el CSV y el Retorno_1Y de un ETF que sigue al Russell 3000
    como proxy del benchmark (IWV -- iShares Russell 3000 ETF -- con
    fallback a VTI o SPY si IWV no esta en la corrida vigente).
  - "Cartera final entre 500 y 750 posiciones": con ETFs como unidad esto
    es matematicamente imposible -- el CSV de entrada trae ~1200 ETFs EN
    TOTAL (todas las categorias: bonos, internacionales, commodities,
    etc.), y el universo de renta variable EE.UU. por cap-segmento ronda
    apenas 250-300 ANTES de cualquier filtro de liquidez. El usuario
    confirmo explicitamente ("Hazlo con lo que tengas a disposicion",
    2026-08-07) aplicar el algoritmo completo sobre TODO el universo
    disponible en la primera version de este script (90 ETFs resultantes,
    seleccionados por cobertura del 33% de AUM por bucket).

  - **Cambio 2026-08-07 (mismo dia, pedido de seguimiento)**: el usuario
    pidio simplificar la seleccion a un Top 15 literal -- "Solo deja los 5
    primeros de small cap, mid y large. con eso arma la lista de 15." Esto
    REEMPLAZA la regla 6 (cobertura del 33% de AUM por bucket) por un
    corte fijo: los 5 ETFs de mayor Score de Momentum de cada uno de los 3
    buckets (N_TOP_BUCKET mas abajo), sin importar cuanto AUM cubran. El
    nombre de la pestaña (entonces "Top 15 Russell 3000", ver mas abajo)
    ahora es literal: la cartera final tiene exactamente 15 posiciones
    (5+5+5), salvo que algun bucket tenga menos de 5 ETFs elegibles tras
    los filtros (no ocurre hoy: Large/Mid/Small tienen 108/40/38
    candidatos antes de este corte).

    Este cambio tiene una consecuencia matematica sobre la regla 7 (tope
    de peso 1.5% por ETF): con solo 15 posiciones, un tope de 1.5% es
    inalcanzable sin dejar de sumar 100% (minimo necesario: 1/0.015 = 67
    posiciones). El script detecta este caso (ver run()) y, cuando ocurre,
    pondera proporcional al Score SIN aplicar el tope (en vez de aplicar
    un tope roto que dejaria la suma muy por debajo de 100%), dejando un
    aviso impreso. Si en el futuro se pide volver a un universo mas amplio
    (donde el tope si es alcanzable), _cap_pesos() ya implementa el
    algoritmo completo de cap-and-redistribute -- no hace falta reescribirlo.

  - **Renombre 2026-08-07 (mismo dia, tercer pedido de seguimiento)**: el
    usuario pidio cambiar el nombre visible de la pestaña de "Top 15
    Russell 3000" a "Top 15 Cap." en todo el proyecto (frontend, PDF,
    docstrings/comentarios, CLAUDE.md). Es un cambio de ETIQUETA
    unicamente -- no se tocaron nombres de archivo, del modulo Python, del
    endpoint (`GET /api/russell3000` sigue igual), ni de las columnas/CSV
    de salida (`etf_russell3000_portfolio.csv`,
    `etf_russell3000_resumen_buckets.csv`, `Cap_Bucket`, etc.), que
    seguirian usando "russell3000"/"Russell 3000" internamente.
"""

import glob
import os
import re

import numpy as np
import pandas as pd

from fog_radar.core.logging import get_logger

log = get_logger("etf.analysis")


# Segmentacion por cap-segment: las unicas 9 categorias Morningstar que
# traen una etiqueta de capitalizacion explicita en este dataset (Large/
# Mid/Small x Blend/Value/Growth). Cumple doble proposito: whitelist del
# universo (regla 1 del usuario) y asignacion de bucket (regla 4).
CATEGORIAS_CAP_BUCKET = {
    "Large Blend": "Large-cap",
    "Large Value": "Large-cap",
    "Large Growth": "Large-cap",
    "Mid-Cap Blend": "Mid-cap",
    "Mid-Cap Value": "Mid-cap",
    "Mid-Cap Growth": "Mid-cap",
    "Small Blend": "Small-cap",
    "Small Value": "Small-cap",
    "Small Growth": "Small-cap",
}
BUCKET_ORDEN = ["Large-cap", "Mid-cap", "Small-cap"]

# Exclusion explicita (regla 2 del usuario) -- defensiva, ver docstring.
CATEGORIAS_EXCLUIR = ["Utilities", "Real Estate", "Global Real Estate"]

# Filtro de liquidez (regla 2) -- mismos umbrales que LIQ_VOL_MIN/
# LIQ_AUM_MIN de etf_ranking_analysis.py, para mantener un unico criterio
# de "liquido" en el proyecto (cada script mantiene su propia constante
# por convencion de config.py, ver ese modulo).
LIQ_VOL_MIN = 10_000
LIQ_AUM_MIN = 30_000_000

# Benchmark del componente Alpha (regla 3): proxy del Russell 3000 dentro
# del propio universo de ETFs, con fallback si IWV no esta presente.
BENCHMARK_CANDIDATOS = ["IWV", "VTI", "SPY"]

# Corte por bucket (regla 6, actualizada 2026-08-07 -- ver docstring
# "Cambio 2026-08-07"): top N_TOP_BUCKET ETFs por Score de Momentum en
# cada uno de los 3 buckets, en vez de cobertura de AUM. 3 buckets x 5 =
# 15 posiciones, de ahi el nombre literal de la pestaña.
N_TOP_BUCKET = 5

# Tope de peso por ETF (regla 7), tal como pide el usuario. Con
# N_TOP_BUCKET=5 (15 posiciones totales) este tope es matematicamente
# inalcanzable (ver run()) -- se deja la constante para cuando el universo
# vuelva a ser mas amplio.
TOPE_PESO = 0.015

COL_6M = "Performance 6M (%)"
COL_1Y = "Performance 1Y (%)"
COL_BETA = "Beta"
COL_AUM = "AUM"
COL_VOL = "Volumen"

REQUIRED_COLS = [COL_6M, COL_1Y, COL_BETA, COL_AUM, COL_VOL, "Precio"]


def encontrar_csv_mas_reciente(carpeta):
    candidatos = sorted(glob.glob(os.path.join(carpeta, "etf_perf_data_*.csv")))
    if not candidatos:
        raise FileNotFoundError(f"No se encontro ningun archivo etf_perf_data_*.csv en {carpeta}")
    return candidatos[-1]


def minmax_0_100(series: pd.Series) -> pd.Series:
    clean = series.copy().replace([np.inf, -np.inf], np.nan)
    min_value, max_value = clean.min(), clean.max()
    if pd.isna(min_value) or pd.isna(max_value) or max_value == min_value:
        return pd.Series(50.0, index=clean.index)
    return 100 * (clean - min_value) / (max_value - min_value)


def _elegir_benchmark(df_crudo):
    """Busca el primer ticker de BENCHMARK_CANDIDATOS presente (con
    Performance 1Y no nulo) en el CSV crudo, antes de cualquier filtro de
    universo. Devuelve (ticker, retorno_1Y) o (None, None) si ninguno esta."""
    for ticker in BENCHMARK_CANDIDATOS:
        fila = df_crudo[df_crudo["Ticker"] == ticker]
        if len(fila) and pd.notna(fila.iloc[0][COL_1Y]):
            return ticker, float(fila.iloc[0][COL_1Y])
    return None, None


def _cap_pesos(scores: pd.Series, tope: float = TOPE_PESO) -> pd.Series:
    """Pondera proporcional al Score de Momentum (regla 7, no al AUM), con
    tope maximo por posicion. Algoritmo estandar de "cap and redistribute":
    en cada pasada, las posiciones que superan el tope quedan FIJAS en el
    tope de forma permanente, y el resto se reescala para que la suma total
    vuelva a dar 100% -- se repite hasta que ninguna posicion libre supere
    el tope tras reescalar. Si el universo tiene menos de 1/tope posiciones
    (~67 con tope=1.5%), el tope es matematicamente inalcanzable sin dejar
    de sumar 100%; en ese caso todas las posiciones quedan fijas en el tope
    y la suma final queda por debajo de 100% (documentado en CLAUDE.md,
    no se fuerza a sumar 100% inventando peso)."""
    pesos = (scores / scores.sum()).astype(float).copy()
    fijo = pd.Series(False, index=pesos.index)

    for _ in range(len(pesos) + 5):
        libres = ~fijo
        if not libres.any():
            break
        nuevos_sobre_tope = libres & (pesos > tope + 1e-12)
        if not nuevos_sobre_tope.any():
            break
        fijo = fijo | nuevos_sobre_tope
        pesos[nuevos_sobre_tope] = tope

        libres = ~fijo
        if not libres.any():
            break
        objetivo_libres = 1.0 - pesos[fijo].sum()
        suma_libres_actual = pesos[libres].sum()
        if suma_libres_actual <= 0 or objetivo_libres <= 0:
            pesos[libres] = 0.0
            break
        factor = objetivo_libres / suma_libres_actual
        pesos[libres] = pesos[libres] * factor

    return pesos


def run(ruta_csv):
    log.info(f"Leyendo: {ruta_csv}")
    df_crudo = pd.read_csv(ruta_csv, encoding="utf-8-sig")
    for col in [COL_6M, COL_1Y, COL_BETA, COL_AUM, COL_VOL, "Precio"]:
        if col in df_crudo.columns:
            df_crudo[col] = pd.to_numeric(df_crudo[col], errors="coerce")

    benchmark_ticker, benchmark_ret_1y = _elegir_benchmark(df_crudo)
    if benchmark_ticker is None:
        raise ValueError(
            f"Ninguno de los benchmarks candidatos {BENCHMARK_CANDIDATOS} esta presente "
            "en la corrida vigente -- no se puede calcular el componente Alpha."
        )

    universo = df_crudo[df_crudo["TipoInstrumento"] == "ETF"].copy()
    universo = universo[universo["Ticker"] != benchmark_ticker].copy()

    # Regla 1: universo = ETFs de renta variable EE.UU. por cap-segmento.
    universo["Cap_Bucket"] = universo["Category"].map(CATEGORIAS_CAP_BUCKET)
    universo = universo[universo["Cap_Bucket"].notna()].copy()

    # Regla 2: exclusion explicita de Utilities/REIT (defensiva, ver docstring).
    universo = universo[~universo["Category"].isin(CATEGORIAS_EXCLUIR)].copy()

    # Completitud de datos.
    universo = universo.dropna(subset=REQUIRED_COLS).copy()

    # Filtro de liquidez (regla 2).
    universo = universo[(universo[COL_VOL] >= LIQ_VOL_MIN) & (universo[COL_AUM] >= LIQ_AUM_MIN)].copy()

    # Regla 3: Score de Momentum, 3 componentes a igual peso (1/3 c/u).
    universo["Comp_T12T1_aprox"] = universo[COL_1Y]
    universo["Comp_T7T1_aprox"] = universo[COL_6M]
    universo["Alpha_CAPM_aprox"] = universo[COL_1Y] - universo[COL_BETA] * benchmark_ret_1y

    universo["Comp1_norm"] = minmax_0_100(universo["Comp_T12T1_aprox"])
    universo["Comp2_norm"] = minmax_0_100(universo["Comp_T7T1_aprox"])
    universo["Comp3_norm"] = minmax_0_100(universo["Alpha_CAPM_aprox"])
    universo["Momentum_Score"] = (
        universo["Comp1_norm"] + universo["Comp2_norm"] + universo["Comp3_norm"]
    ) / 3.0

    # Reglas 4-6: bucketing + top N_TOP_BUCKET por bucket (actualizado
    # 2026-08-07, ver docstring "Cambio 2026-08-07"), rankeando por Score
    # de Momentum de mayor a menor dentro de cada bucket. Pct_AUM_Cubierto
    # queda como dato informativo (cuanto AUM representan los 5 elegidos
    # dentro de su bucket), ya NO determina cuantos se seleccionan.
    seleccion_partes = []
    resumen_buckets = []
    for bucket in BUCKET_ORDEN:
        sub = universo[universo["Cap_Bucket"] == bucket].sort_values("Momentum_Score", ascending=False).copy()
        aum_total_bucket = float(sub[COL_AUM].sum())
        n_seleccion = min(N_TOP_BUCKET, len(sub))

        seleccionados = sub.head(n_seleccion)
        seleccion_partes.append(seleccionados)
        resumen_buckets.append(
            {
                "Cap_Bucket": bucket,
                "N_Universo": int(len(sub)),
                "AUM_Total_Bucket": aum_total_bucket,
                "N_Seleccionados": int(n_seleccion),
                "AUM_Seleccionado": float(seleccionados[COL_AUM].sum()),
                "Pct_AUM_Cubierto": (float(seleccionados[COL_AUM].sum()) / aum_total_bucket * 100)
                if aum_total_bucket > 0
                else 0.0,
            }
        )

    portfolio = (
        pd.concat(seleccion_partes, ignore_index=True) if seleccion_partes else universo.iloc[0:0].copy()
    )

    # Regla 7: ponderacion proporcional al Score de Momentum (no al AUM),
    # con tope de 1.5% por ETF -- SOLO si es matematicamente alcanzable
    # (se necesitan al menos 1/TOPE_PESO posiciones para que el tope no
    # deje la suma por debajo de 100%, ver docstring "Cambio 2026-08-07").
    # Con 15 posiciones (N_TOP_BUCKET=5 x 3 buckets) no lo es: se pondera
    # proporcional al Score sin tope y se avisa por consola.
    if len(portfolio):
        min_posiciones_para_tope = int(np.ceil(1.0 / TOPE_PESO))
        if len(portfolio) * TOPE_PESO < 1.0:
            log.info(
                f"[AVISO] Tope de peso {TOPE_PESO * 100:.1f}% inalcanzable con {len(portfolio)} posiciones "
                f"(minimo necesario: {min_posiciones_para_tope}). Se pondera proporcional al Score SIN tope."
            )
            portfolio["Peso_Portfolio"] = (
                portfolio["Momentum_Score"] / portfolio["Momentum_Score"].sum()
            ).values * 100
        else:
            portfolio["Peso_Portfolio"] = _cap_pesos(portfolio["Momentum_Score"]).values * 100
    else:
        portfolio["Peso_Portfolio"] = pd.Series(dtype=float)

    portfolio = portfolio.sort_values("Peso_Portfolio", ascending=False).reset_index(drop=True)

    m_fecha = re.search(r"(\d{8})", os.path.basename(ruta_csv))
    fecha_tag = m_fecha.group(0) if m_fecha else "SINFECHA"
    portfolio["Fecha"] = fecha_tag
    portfolio["Benchmark_Alpha"] = benchmark_ticker

    cols_out = [
        "Ticker",
        "Nombre",
        "Category",
        "Cap_Bucket",
        COL_6M,
        COL_1Y,
        COL_BETA,
        COL_AUM,
        COL_VOL,
        "Comp_T12T1_aprox",
        "Comp_T7T1_aprox",
        "Alpha_CAPM_aprox",
        "Comp1_norm",
        "Comp2_norm",
        "Comp3_norm",
        "Momentum_Score",
        "Peso_Portfolio",
        "Benchmark_Alpha",
        "Fecha",
    ]
    cols_out = [c for c in cols_out if c in portfolio.columns]

    carpeta_salida = os.path.dirname(os.path.abspath(ruta_csv))
    portfolio_path = os.path.join(carpeta_salida, "etf_russell3000_portfolio.csv")
    resumen_path = os.path.join(carpeta_salida, "etf_russell3000_resumen_buckets.csv")

    portfolio[cols_out].to_csv(portfolio_path, index=False)
    resumen_df = pd.DataFrame(resumen_buckets)
    resumen_df["Fecha"] = fecha_tag
    resumen_df["Benchmark_Alpha"] = benchmark_ticker
    resumen_df["Benchmark_Retorno_1Y"] = benchmark_ret_1y
    resumen_df.to_csv(resumen_path, index=False)

    log.info(resumen_df.to_string(index=False))
    log.info(f"Benchmark Alpha: {benchmark_ticker} (Retorno 1Y: {benchmark_ret_1y:.2f}%)")
    log.info(
        f"Portfolio final: {len(portfolio)} ETFs. Suma de pesos: {portfolio['Peso_Portfolio'].sum():.2f}%"
    )

    return {
        "fecha_tag": fecha_tag,
        "portfolio": portfolio[cols_out],
        "resumen_buckets": resumen_df,
        "benchmark_ticker": benchmark_ticker,
        "benchmark_ret_1y": benchmark_ret_1y,
    }
