"""
Analisis Cuantitativo de ETFs - Ranking y Deteccion de Outliers (version WEB)
==============================================================================

Copia adaptada de etf_ranking_analysis.py (carpeta raiz del proyecto
ETF_Choice) para ser invocada desde el backend FastAPI de WEB/. Mantiene
EXACTAMENTE la misma metodologia y los mismos pesos (WEIGHTS) que la version
raiz -- ver CLAUDE.md seccion 2 para el detalle completo de cada decision.

Diferencias respecto a la version raiz (deliberadas, para el uso desde una
API en vez de linea de comandos):
  - Expone run(ruta_csv) -> dict con los DataFrames resultantes (top15,
    ranking, excluidos) ademas de escribir los mismos 4 CSV + historial que
    la version raiz (sin el grafico PNG: el frontend web dibuja sus propios
    graficos a partir del JSON, no hace falta generar una imagen).
  - No requiere matplotlib.
  - No asume que el CSV vive en la raiz del proyecto: la carpeta de salida
    es la misma carpeta donde esta el CSV de entrada (WEB/backend/data).
"""

import glob
import os
import re

import pandas as pd

from fog_radar.core.logging import get_logger

from .config import PERIOD_COLS, RISK_COLS, TOP_N

log = get_logger("etf.analysis")

# ----------------------------------------------------------------------
# Parametros configurables (identicos a la version raiz)
# ----------------------------------------------------------------------
LIQ_VOL_MIN = 10_000
LIQ_AUM_MIN = 30_000_000
Z_THRESHOLD = 6.0

OTHER_REQUIRED = ["52W High", "52W Low", "Precio", "Volumen", "AUM"]

# Categorias excluidas del analisis principal por decision explicita del
# usuario (2026-07-24): "no me aportan nada". Hasta esta fecha estas 3
# categorias NO se excluian aca -- solo se las demotaba via WEIGHTS (ver
# historial Paso 1/2/3 en CLAUDE.md seccion 2), a diferencia de
# etf_momentum_analysis.py y etf_categoria_analysis.py que ya las excluian
# explicitamente. Mismo criterio de fondo documentado en esos dos scripts:
# Beta~=0 infla Sharpe/Treynor como artefacto del denominador, no por buen
# retorno absoluto.
CATEGORIAS_EXCLUIR = ["Ultrashort Bond", "Securitized Bond - Focused", "Target Maturity"]

# Pesos del score compuesto -- identicos a la version raiz (ver
# CLAUDE.md seccion 2 para el historial completo de items #16/#17 y del
# ajuste "Paso 3" 2026-07-18).
WEIGHTS = {
    "s_sharpe": 0.25,
    "s_treynor": 0.1175,
    "s_1y": 0.14,
    "s_6m": 0.075,
    "s_consistencia": 0.15,
    "s_tendencia": 0.10,
    "s_baja_dispersion": 0.05,
    "s_bajo_beta": 0.0675,
    "s_cerca_maximo": 0.05,
}
assert abs(sum(WEIGHTS.values()) - 1.0) < 1e-9, "Los pesos deben sumar 1.0"

HISTORIAL_COLS = [
    "Fecha",
    "Universo_Bruto",
    "Universo_Depurado",
    "N_Outliers",
    "Sharpe_Promedio",
    "Beta_Promedio",
    "Top1_Ticker",
    "Top1_Score",
]


# ----------------------------------------------------------------------
# Utilidades (identicas a la version raiz)
# ----------------------------------------------------------------------
def encontrar_csv_mas_reciente(carpeta):
    candidatos = sorted(glob.glob(os.path.join(carpeta, "etf_perf_data_*.csv")))
    if not candidatos:
        raise FileNotFoundError(f"No se encontro ningun archivo etf_perf_data_*.csv en {carpeta}")
    return candidatos[-1]


def mad_z_score(serie):
    mediana = serie.median()
    mad = (serie - mediana).abs().median()
    if mad == 0:
        return pd.Series(0.0, index=serie.index), mediana, mad
    return 0.6745 * (serie - mediana) / mad, mediana, mad


def pct_rank(serie, ascending=True):
    return serie.rank(pct=True, ascending=ascending) * 100


def pasos_crecientes(row):
    vals = [row[c] for c in PERIOD_COLS]
    return sum(1 for i in range(len(vals) - 1) if vals[i + 1] >= vals[i])


def actualizar_historial(
    carpeta_salida,
    fecha_tag,
    n_bruto,
    n_depurado,
    n_outliers,
    sharpe_prom,
    beta_prom,
    top1_ticker,
    top1_score,
):
    historial_path = os.path.join(carpeta_salida, "historial_corridas.csv")
    fila_nueva = pd.DataFrame(
        [
            {
                "Fecha": fecha_tag,
                "Universo_Bruto": n_bruto,
                "Universo_Depurado": n_depurado,
                "N_Outliers": n_outliers,
                "Sharpe_Promedio": round(float(sharpe_prom), 4),
                "Beta_Promedio": round(float(beta_prom), 4),
                "Top1_Ticker": top1_ticker,
                "Top1_Score": round(float(top1_score), 2),
            }
        ],
        columns=HISTORIAL_COLS,
    )

    historial_previo = None
    if os.path.exists(historial_path):
        historial_previo = pd.read_csv(historial_path, dtype={"Fecha": str})
        historial_previo = historial_previo[historial_previo["Fecha"] != fecha_tag]

    # pd.concat() con un DataFrame vacio (sin filas -- primera corrida, o
    # una corrida previa que quedo vacia tras sacarle la fila de
    # fecha_tag) dispara un FutureWarning de pandas sobre inferencia de
    # dtypes en concatenaciones con entradas vacias/todo-NA (mejoras
    # pendientes de WEB/, item 5). Si no hay nada previo (o quedo vacio),
    # se usa directamente fila_nueva sin concatenar nada.
    if historial_previo is None or historial_previo.empty:
        historial = fila_nueva
    else:
        historial = pd.concat([historial_previo, fila_nueva], ignore_index=True)

    historial = historial.sort_values("Fecha").reset_index(drop=True)
    historial.to_csv(historial_path, index=False)
    return historial_path


# ----------------------------------------------------------------------
# Analisis principal
# ----------------------------------------------------------------------
def run(ruta_csv):
    """Corre el pipeline completo sobre ruta_csv y escribe los outputs en la
    misma carpeta. Devuelve un dict con los DataFrames resultantes para que
    el backend FastAPI los sirva directamente sin tener que releer los CSV."""
    log.info(f"Leyendo: {ruta_csv}")
    df = pd.read_csv(ruta_csv, encoding="utf-8-sig")

    universo = df[df["TipoInstrumento"] == "ETF"].copy()
    n_universo_bruto = len(universo)

    universo = universo[~universo["Category"].isin(CATEGORIAS_EXCLUIR)].copy()

    columnas_requeridas = PERIOD_COLS + RISK_COLS + OTHER_REQUIRED
    universo = universo.dropna(subset=columnas_requeridas).copy()

    universo = universo[(universo["Volumen"] >= LIQ_VOL_MIN) & (universo["AUM"] >= LIQ_AUM_MIN)].copy()

    z_cols = []
    for col in PERIOD_COLS:
        z, _, _ = mad_z_score(universo[col])
        zc = col + "_z"
        universo[zc] = z
        z_cols.append(zc)

    es_outlier = (universo[z_cols].abs() > Z_THRESHOLD).any(axis=1)
    excluidos = universo[es_outlier].copy()
    universo_limpio = universo[~es_outlier].copy()

    def ventana_y_z_mas_extremo(row):
        zs = {c: row[c + "_z"] for c in PERIOD_COLS}
        peor_col = max(zs, key=lambda k: abs(zs[k]))
        etiqueta = peor_col.replace(" (%)", "").replace("Performance ", "")
        return pd.Series({"Ventana_Atipica": etiqueta, "Z_Score": zs[peor_col]})

    if len(excluidos):
        extra = excluidos.apply(ventana_y_z_mas_extremo, axis=1)
        excluidos = pd.concat([excluidos, extra], axis=1)
        # Orden por Z_Score CON signo, descendente (pedido explicito del
        # usuario 2026-07-24): antes se ordenaba por |Z_Score| descendente
        # (el mas distorsionado primero sin importar el signo), lo que
        # intercalaba picos positivos y negativos. El usuario prefiere ver
        # primero los picos positivos mas extremos y al final los
        # negativos mas extremos, sin mezclar signos.
        excluidos = excluidos.sort_values("Z_Score", ascending=False)

    universo_limpio["Dist_52W_High_%"] = (
        (universo_limpio["Precio"] - universo_limpio["52W High"]) / universo_limpio["52W High"] * 100
    )
    universo_limpio["Rango_52W_%"] = (
        (universo_limpio["52W High"] - universo_limpio["52W Low"]) / universo_limpio["52W Low"] * 100
    )

    universo_limpio["Periodos_Positivos"] = (universo_limpio[PERIOD_COLS] > 0).sum(axis=1)
    universo_limpio["Dispersion_Periodos"] = universo_limpio[PERIOD_COLS].std(axis=1)
    universo_limpio["Pasos_Crecientes"] = universo_limpio.apply(pasos_crecientes, axis=1)

    universo_limpio["s_sharpe"] = pct_rank(universo_limpio["Sharpe Ratio"])
    universo_limpio["s_treynor"] = pct_rank(universo_limpio["Treynor Ratio"])
    universo_limpio["s_1y"] = pct_rank(universo_limpio["Performance 1Y (%)"])
    universo_limpio["s_6m"] = pct_rank(universo_limpio["Performance 6M (%)"])
    universo_limpio["s_consistencia"] = pct_rank(universo_limpio["Periodos_Positivos"])
    universo_limpio["s_tendencia"] = pct_rank(universo_limpio["Pasos_Crecientes"])
    universo_limpio["s_baja_dispersion"] = pct_rank(universo_limpio["Dispersion_Periodos"], ascending=False)
    universo_limpio["s_bajo_beta"] = pct_rank(universo_limpio["Beta"].abs(), ascending=False)
    universo_limpio["s_cerca_maximo"] = pct_rank(universo_limpio["Dist_52W_High_%"])

    universo_limpio["Score"] = sum(universo_limpio[k] * w for k, w in WEIGHTS.items())

    # Ranking alternativo "Top 15 Simple" (pedido explicito del usuario
    # 2026-08-06, pestaña separada del ranking principal): considera
    # UNICAMENTE las 4 ventanas de rendimiento y el Beta, nada mas (sin
    # Sharpe/Treynor/consistencia/tendencia/dispersion/distancia al
    # maximo). Mismo universo depurado (universo_limpio) que el ranking
    # principal -- ETF, sin las 3 categorias excluidas, sin filas
    # incompletas, con liquidez minima, sin outliers por z-score MAD --
    # solo cambia la ponderacion. 5 factores a partes iguales (20% c/u):
    # las 4 ventanas (percentil ascendente, mayor retorno = mejor puntaje)
    # y el Beta (percentil descendente sobre |Beta|, misma convencion que
    # s_bajo_beta arriba: menor beta = mejor puntaje).
    SIMPLE_WEIGHTS = {
        "ss_5d": 0.20,
        "ss_1m": 0.20,
        "ss_6m": 0.20,
        "ss_1y": 0.20,
        "ss_bajo_beta": 0.20,
    }
    universo_limpio["ss_5d"] = pct_rank(universo_limpio[PERIOD_COLS[0]])
    universo_limpio["ss_1m"] = pct_rank(universo_limpio[PERIOD_COLS[1]])
    universo_limpio["ss_6m"] = pct_rank(universo_limpio[PERIOD_COLS[2]])
    universo_limpio["ss_1y"] = pct_rank(universo_limpio[PERIOD_COLS[3]])
    universo_limpio["ss_bajo_beta"] = pct_rank(universo_limpio["Beta"].abs(), ascending=False)
    universo_limpio["Score_Simple"] = sum(universo_limpio[k] * w for k, w in SIMPLE_WEIGHTS.items())

    ranking = universo_limpio.sort_values("Score", ascending=False).reset_index(drop=True)
    top15 = ranking.head(TOP_N).copy()

    ranking_simple = universo_limpio.sort_values("Score_Simple", ascending=False).reset_index(drop=True)
    top15_simple = ranking_simple.head(TOP_N).copy()

    carpeta_salida = os.path.dirname(os.path.abspath(ruta_csv))
    m_fecha = re.search(r"(\d{8})", os.path.basename(ruta_csv))
    fecha_tag = m_fecha.group(0) if m_fecha else "SINFECHA"

    for frame in (top15, ranking, top15_simple):
        frame["Fecha"] = fecha_tag

    top15_path = os.path.join(carpeta_salida, "etf_top15_recomendados.csv")
    ranking_path = os.path.join(carpeta_salida, "etf_ranking_completo.csv")
    excluidos_path = os.path.join(carpeta_salida, "etf_excluidos_outliers.csv")
    top15_simple_path = os.path.join(carpeta_salida, "etf_top15_simple.csv")

    top15.to_csv(top15_path, index=False)
    ranking.to_csv(ranking_path, index=False)
    cols_top15_simple = (
        ["Ticker", "Nombre", "Category"]
        + PERIOD_COLS
        + ["Beta", "ss_5d", "ss_1m", "ss_6m", "ss_1y", "ss_bajo_beta", "Score_Simple", "Fecha"]
    )
    top15_simple[cols_top15_simple].to_csv(top15_simple_path, index=False)

    cols_excluidos = (
        ["Ticker", "Nombre", "Category"]
        + PERIOD_COLS
        + ["Sharpe Ratio", "Beta", "AUM", "Volumen", "Ventana_Atipica", "Z_Score"]
    )
    if len(excluidos):
        excluidos_out = excluidos[cols_excluidos].copy()
    else:
        excluidos_out = pd.DataFrame(columns=cols_excluidos)
    excluidos_out["Fecha"] = fecha_tag
    excluidos_out.to_csv(excluidos_path, index=False)

    top1 = top15.iloc[0]
    actualizar_historial(
        carpeta_salida,
        fecha_tag,
        n_bruto=n_universo_bruto,
        n_depurado=len(universo_limpio),
        n_outliers=len(excluidos),
        sharpe_prom=universo_limpio["Sharpe Ratio"].mean(),
        beta_prom=universo_limpio["Beta"].mean(),
        top1_ticker=top1["Ticker"],
        top1_score=top1["Score"],
    )

    return {
        "fecha_tag": fecha_tag,
        "n_universo_bruto": n_universo_bruto,
        "n_universo_depurado": len(universo_limpio),
        "n_outliers": len(excluidos),
        "top15": top15,
        "ranking": ranking,
        "excluidos": excluidos_out,
        "top15_simple": top15_simple[cols_top15_simple],
    }
