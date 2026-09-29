"""
Rotacion del Top 15 vs. corrida anterior + Diversificacion por Sector
(version WEB) - portado desde generar_datos_rotacion() /
_calcular_racha() / _listar_runs_top15() / _encontrar_archivo_previo() /
tabla_diversificacion() en generar_informe_pdf.py de la carpeta raiz
(ver CLAUDE.md seccion 3, "Rotacion del Top 15 vs. Corrida Anterior").

Misma logica que la version raiz, adaptada a la disposicion de carpetas de
WEB/backend: la corrida vigente vive en backend/data/ y las corridas
archivadas en backend/data_hist/ (equivalente a <carpeta>/old/ en la raiz).
"""

import glob
import os

import pandas as pd

# ranking_mod.CATEGORIAS_EXCLUIR explica en "Salidas" cuando un ticker dejo el
# Top 15 porque su categoria quedo excluida del analisis (item 34 del original).
# archivado_mod.extraer_fecha_y_secuencia(): mismo esquema de nombres
# "_<fecha>" / "_<fecha>_<n>" que usa archivar_corrida() para versionar
# corridas de la MISMA fecha calendario.
from . import archivado as archivado_mod
from . import etf_ranking_analysis as ranking_mod


def _encontrar_archivo_previo(data_hist_dir, base_name):
    """Devuelve el archivo archivado mas reciente que matchee base_name,
    ordenando por (fecha, secuencia) parseados del propio nombre -- no por
    fecha de modificacion del filesystem (criterio hasta 2026-09-17): dos
    corridas archivadas el mismo dia calendario (ver archivado_mod, "el
    incidente concreto") se distinguen por el contador "_2"/"_3" del
    nombre, que es la fuente de verdad de cual se archivo despues, en vez
    de depender de que el mtime del archivo no haya cambiado por otro
    motivo (una copia, un backup, etc.)."""
    raiz, ext = os.path.splitext(base_name)
    patron = os.path.join(data_hist_dir, raiz + "*" + ext)
    candidatos = glob.glob(patron)
    if not candidatos:
        return None

    candidatos.sort(key=lambda ruta: archivado_mod.extraer_fecha_y_secuencia(os.path.basename(ruta)))
    return candidatos[-1]


def _listar_runs_top15(data_dir, data_hist_dir):
    rutas = glob.glob(os.path.join(data_hist_dir, "etf_top15_recomendados*.csv"))
    # Orden por (fecha, secuencia) del nombre ANTES de leer el contenido:
    # si dos corridas archivadas comparten la misma fecha calendario (y por
    # lo tanto la misma columna Fecha interna), el diccionario de abajo las
    # indexa bajo la misma clave y se queda con la ULTIMA que procese --
    # este orden garantiza que sea la de mayor secuencia (la corrida real
    # mas reciente para esa fecha), no la que haya devuelto el glob primero
    # (orden del filesystem, arbitrario).
    rutas.sort(key=lambda ruta: archivado_mod.extraer_fecha_y_secuencia(os.path.basename(ruta)))
    vigente = os.path.join(data_dir, "etf_top15_recomendados.csv")
    if os.path.exists(vigente):
        rutas.append(vigente)

    runs = {}
    for ruta in rutas:
        try:
            df = pd.read_csv(ruta)
        except Exception:
            continue
        if "Fecha" not in df.columns or df.empty:
            continue
        fecha = str(df["Fecha"].iloc[0])
        runs[fecha] = set(df["Ticker"])
    return sorted(runs.items(), key=lambda kv: kv[0])


def _calcular_racha(runs, tickers_actual):
    racha = {}
    for t in tickers_actual:
        count = 0
        for _, tickers_run in reversed(runs):
            if t in tickers_run:
                count += 1
            else:
                break
        racha[t] = max(count, 1)
    return racha


def generar_datos_rotacion(data_dir, data_hist_dir, top15, excl, ranking):
    """Devuelve dict con df_mantienen / df_nuevos / df_salidas, o None si no
    hay corrida anterior completa archivada en data_hist para comparar."""
    prev_top15_path = _encontrar_archivo_previo(data_hist_dir, "etf_top15_recomendados.csv")
    prev_ranking_path = _encontrar_archivo_previo(data_hist_dir, "etf_ranking_completo.csv")
    prev_excl_path = _encontrar_archivo_previo(data_hist_dir, "etf_excluidos_outliers.csv")
    if not (prev_top15_path and prev_ranking_path and prev_excl_path):
        return None

    prev_top15 = pd.read_csv(prev_top15_path).reset_index(drop=True)
    prev_ranking = pd.read_csv(prev_ranking_path).reset_index(drop=True)
    prev_excl = pd.read_csv(prev_excl_path)

    tickers_actual_lista = list(top15["Ticker"])
    tickers_prev_lista = list(prev_top15["Ticker"])
    rank_actual_top15 = {t: i + 1 for i, t in enumerate(tickers_actual_lista)}
    rank_previo_top15 = {t: i + 1 for i, t in enumerate(tickers_prev_lista)}
    rank_actual_universo = {t: i + 1 for i, t in enumerate(ranking["Ticker"])}
    rank_previo_universo = {t: i + 1 for i, t in enumerate(prev_ranking["Ticker"])}
    score_actual = dict(zip(ranking["Ticker"], ranking["Score"], strict=True))
    score_previo = dict(zip(prev_ranking["Ticker"], prev_ranking["Score"], strict=True))

    tickers_actual = set(tickers_actual_lista)
    tickers_prev = set(tickers_prev_lista)
    excl_prev_tickers = set(prev_excl["Ticker"]) if len(prev_excl) else set()
    excl_actual_tickers = set(excl["Ticker"]) if len(excl) else set()

    runs = _listar_runs_top15(data_dir, data_hist_dir)
    racha = _calcular_racha(runs, tickers_actual)

    filas_mant = []
    for t in tickers_actual & tickers_prev:
        row = top15[top15["Ticker"] == t].iloc[0]
        filas_mant.append(
            {
                "Ticker": t,
                "Nombre": row["Nombre"],
                "Category": row["Category"],
                "Racha": racha.get(t, 1),
                "Rank_actual": rank_actual_top15[t],
                "Rank_previo": rank_previo_top15[t],
                "Score": score_actual.get(t, float("nan")),
                "Delta_Score": score_actual.get(t, float("nan")) - score_previo.get(t, float("nan")),
                "Delta_Rank": rank_previo_top15[t] - rank_actual_top15[t],
                # Precio/52W High/52W Low agregados 2026-08-03 (misma barra de
                # posicion en el rango de 52 semanas que ya se ve en Top15Table
                # y MomentumTable). "row" viene de top15 (corrida vigente), que
                # ya trae estas 3 columnas sin filtrar (ver
                # etf_ranking_analysis.py).
                "Precio": row.get("Precio"),
                "52W High": row.get("52W High"),
                "52W Low": row.get("52W Low"),
            }
        )
    df_mant = (
        pd.DataFrame(filas_mant).sort_values("Rank_actual").reset_index(drop=True)
        if filas_mant
        else pd.DataFrame()
    )

    filas_nuevos = []
    for t in tickers_actual - tickers_prev:
        row = top15[top15["Ticker"] == t].iloc[0]
        if t in excl_prev_tickers:
            procedencia = "Estaba excluido por outlier en la corrida anterior"
        elif t in rank_previo_universo:
            procedencia = f"Ya estaba en el ranking depurado anterior, puesto #{rank_previo_universo[t]}"
        else:
            procedencia = "No estaba en el universo depurado ni en outliers de la corrida anterior"
        filas_nuevos.append(
            {
                "Ticker": t,
                "Nombre": row["Nombre"],
                "Category": row["Category"],
                "Rank_actual": rank_actual_top15[t],
                "Score": score_actual.get(t, float("nan")),
                "Procedencia": procedencia,
                "Precio": row.get("Precio"),
                "52W High": row.get("52W High"),
                "52W Low": row.get("52W Low"),
            }
        )
    df_nuevos = (
        pd.DataFrame(filas_nuevos).sort_values("Rank_actual").reset_index(drop=True)
        if filas_nuevos
        else pd.DataFrame()
    )

    filas_salidas = []
    for t in tickers_prev - tickers_actual:
        row = prev_top15[prev_top15["Ticker"] == t].iloc[0]
        if t in excl_actual_tickers:
            destino = "Paso a la tabla de excluidos por outlier en esta corrida"
        elif row["Category"] in ranking_mod.CATEGORIAS_EXCLUIR:
            # Mismo motivo por el que no aparece en excl_actual_tickers ni
            # en rank_actual_universo: una categoria excluida ni siquiera
            # entra al filtro de outliers, se saca del universo antes de
            # eso (ver etf_ranking_analysis.py). Sin este chequeo caia en
            # el "else" generico de mas abajo, que no explicaba la causa
            # real (mejoras pendientes de WEB/, item 34).
            destino = f'Categoria "{row["Category"]}" excluida del analisis por decision de curacion'
        elif t in rank_actual_universo:
            destino = f"Sigue en el ranking depurado, pero bajo al puesto #{rank_actual_universo[t]}"
        else:
            destino = "Ya no aparece en el universo depurado vigente"
        filas_salidas.append(
            {
                "Ticker": t,
                "Nombre": row["Nombre"],
                "Category": row["Category"],
                "Rank_previo": rank_previo_top15[t],
                "Score_previo": score_previo.get(t, float("nan")),
                "Destino": destino,
                # A diferencia de mantienen/nuevos, aca "row" viene de
                # prev_top15 (la corrida archivada anterior) -- es la posicion
                # en el rango de 52 semanas que tenia el ETF EN ESE MOMENTO,
                # no la actual (una vez que sale del Top 15 dejamos de
                # trackearlo). Es la unica lectura disponible y sigue siendo
                # util para ver por que salio (ej. estaba cerca del maximo,
                # se enfrio despues).
                "Precio": row.get("Precio"),
                "52W High": row.get("52W High"),
                "52W Low": row.get("52W Low"),
            }
        )
    df_salidas = (
        pd.DataFrame(filas_salidas).sort_values("Rank_previo").reset_index(drop=True)
        if filas_salidas
        else pd.DataFrame()
    )

    return {"mantienen": df_mant, "nuevos": df_nuevos, "salidas": df_salidas}


def diversificacion_top15(top15):
    """Tabla de diversificacion por Categoria dentro del Top 15 (portado de
    tabla_diversificacion() en generar_informe_pdf.py, seccion 4)."""
    conteo = top15["Category"].value_counts().reset_index()
    conteo.columns = ["Category", "Conteo"]
    conteo["Pct"] = conteo["Conteo"] / len(top15) * 100
    return conteo
