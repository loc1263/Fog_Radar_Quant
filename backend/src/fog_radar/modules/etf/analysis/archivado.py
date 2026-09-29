"""
Archivado de la corrida vigente de WEB/backend/data hacia
WEB/backend/data_hist (version WEB, adaptada de archivar_corrida.py de la
carpeta raiz -- ver CLAUDE.md seccion 7).

A diferencia de la version raiz (que decide "cual CSV es el nuevo" mirando
cuantos etf_perf_data_*.csv hay sueltos en la carpeta), en el flujo web
siempre hay como maximo UN CSV vigente en backend/data/: archivar_corrida()
se llama explicitamente ANTES de guardar el CSV recien subido, así que "lo
que hay ahora en data/" es siempre, por definicion, la corrida vieja a
archivar.

Contador de secuencia (agregado 2026-09-17, tras un incidente real): hasta
esta version, si ya existia en data_hist_dir un archivo con el mismo
nombre+fecha (dos corridas archivadas el mismo dia calendario -- ej. se
corrigio un bug y se re-corrio el pipeline el mismo dia), archivar_corrida()
se saltaba esa corrida SIN AVISAR ("nunca sobreescribir el historico"). El
incidente concreto: una corrida con un bug de datos (el scraper devolvia
casi todo el performance en NaN, ver etf_discovery/discovery.py) se archivo
primero; tras arreglar el bug se re-corrio el pipeline el mismo dia, y la
version corregida quedo instalada en backend/data/ pero JAMAS se archivo
(la corrida rota, archivada primero, ya ocupaba ese nombre) -- iba a
perderse sin aviso en el proximo archivado, y mientras tanto Rotacion/Racha
seguian comparando contra el snapshot roto indefinidamente.

Ahora, si el nombre+fecha ya existe en data_hist_dir, se agrega un contador
"_2", "_3", etc. (ver _destino_con_secuencia()) en vez de saltear el
archivado -- la corrida mas nueva para una fecha dada SIEMPRE queda
archivada, nunca se pierde. La primera corrida archivada de una fecha
sigue sin contador en el nombre (secuencia 1 implicita), por compatibilidad
con todo el historico ya archivado antes de este cambio. extraer_fecha_y_
secuencia() (usada por rotacion.py para elegir "la corrida previa mas
reciente" y para la Racha) es la contraparte de lectura: interpreta ese
mismo esquema de nombres.
"""

import glob
import os
import re
import shutil

OUTPUT_FILES_DATELESS = [
    "etf_top15_recomendados.csv",
    "etf_ranking_completo.csv",
    "etf_excluidos_outliers.csv",
    "etf_momentum_ranking_completo.csv",
    "etf_momentum_top15_score_comp.csv",
    "etf_categorias_ranking.csv",
    "etf_russell3000_portfolio.csv",
    "etf_russell3000_resumen_buckets.csv",
]

# Matchea el sufijo final "_<8 digitos>.<ext>" o "_<8 digitos>_<n>.<ext>"
# de un nombre de archivo archivado, ej.:
#   "etf_ranking_completo_20260917.csv"   -> fecha=20260917, secuencia=None
#   "etf_ranking_completo_20260917_2.csv" -> fecha=20260917, secuencia=2
_RE_FECHA_SECUENCIA = re.compile(r"_(\d{8})(?:_(\d+))?\.[^.]+$")


def extraer_fecha(nombre_archivo):
    m = re.search(r"(\d{8})", nombre_archivo)
    return m.group(1) if m else None


def extraer_fecha_y_secuencia(nombre_archivo):
    """Devuelve (fecha, secuencia) de un nombre de archivo archivado en
    data_hist, para poder ordenar corridas de la MISMA fecha calendario
    entre si. La primera corrida archivada de una fecha no lleva contador
    en el nombre (ver docstring del modulo) y se interpreta como secuencia
    1. Si el nombre no tiene una fecha de 8 digitos reconocible en la
    posicion esperada (ej. un archivo que no sigue esta convencion),
    devuelve ("00000000", 1) para que ordene antes que cualquier archivo
    con fecha real, en vez de romper la comparacion."""
    m = _RE_FECHA_SECUENCIA.search(nombre_archivo)
    if not m:
        return ("00000000", 1)
    fecha = m.group(1)
    secuencia = int(m.group(2)) if m.group(2) else 1
    return (fecha, secuencia)


def _destino_con_secuencia(data_hist_dir, base, fecha, ext):
    """Devuelve una ruta libre en data_hist_dir para "<base>_<fecha><ext>".
    Si ese nombre ya existe (ya se archivo una corrida de esa misma fecha
    calendario), prueba "<base>_<fecha>_2<ext>", "_3", etc. hasta encontrar
    uno libre -- nunca devuelve un nombre que ya exista, así que quien
    llama puede mover/copiar ahi sin riesgo de pisar un archivo historico."""
    destino = os.path.join(data_hist_dir, f"{base}_{fecha}{ext}")
    if not os.path.exists(destino):
        return destino
    n = 2
    while True:
        candidato = os.path.join(data_hist_dir, f"{base}_{fecha}_{n}{ext}")
        if not os.path.exists(candidato):
            return candidato
        n += 1


def archivar_corrida(data_dir, data_hist_dir):
    """Mueve el/los etf_perf_data_*.csv vigentes en data_dir (y los 8
    outputs sin fecha que los acompanan, ver OUTPUT_FILES_DATELESS) hacia
    data_hist_dir, agregando sufijo _yyyymmdd donde falte (y un contador de
    secuencia si esa fecha ya tiene una corrida archivada, ver docstring
    del modulo). Devuelve la lista de archivos movidos. Si data_dir no
    tiene ningun CSV, no hace nada (nada que archivar)."""
    os.makedirs(data_hist_dir, exist_ok=True)
    csvs = sorted(glob.glob(os.path.join(data_dir, "etf_perf_data_*.csv")))
    if not csvs:
        return []

    movidos = []
    fechas_validas = sorted(f for f in (extraer_fecha(os.path.basename(c)) for c in csvs) if f)
    fecha_outputs = fechas_validas[-1] if fechas_validas else None

    for csv_path in csvs:
        nombre = os.path.basename(csv_path)
        fecha = extraer_fecha(nombre)
        if fecha:
            destino = _destino_con_secuencia(data_hist_dir, "etf_perf_data", fecha, ".csv")
            shutil.move(csv_path, destino)
            movidos.append(destino)
        elif not os.path.exists(os.path.join(data_hist_dir, nombre)):
            # Sin fecha de 8 digitos reconocible en el nombre: no se puede
            # versionar con contador (caso no esperado hoy, el glob de
            # entrada ya exige el patron etf_perf_data_*.csv). Se conserva
            # el criterio previo a este cambio: si el destino sin fecha ya
            # existe, no se archiva (nunca se sobreescribe a ciegas).
            destino = os.path.join(data_hist_dir, nombre)
            shutil.move(csv_path, destino)
            movidos.append(destino)

    if fecha_outputs:
        for nombre in OUTPUT_FILES_DATELESS:
            origen = os.path.join(data_dir, nombre)
            if not os.path.exists(origen):
                continue
            base, ext = os.path.splitext(nombre)
            destino = _destino_con_secuencia(data_hist_dir, base, fecha_outputs, ext)
            shutil.move(origen, destino)
            movidos.append(destino)

    return movidos
