"""Registro de incidencias durante una corrida del Screener.

Agregado a pedido del usuario tras diagnosticar dos situaciones puntuales
al correr el Screener a mano, y ampliado despues (mismo pedido: "que mas
se pueda informar para depurar y mejorar el script de recoleccion") para
cubrir toda la superficie de fallos conocida entre la validacion inicial
y el enriquecimiento de cada ticker. Tipos posibles (columna `Tipo`):

  - RATE_LIMIT: cada vez que validate_and_enrich() hace el backoff de
    "Rate limit alcanzado para <ticker>. Esperando <n>s..." (ver
    etf_discovery/validators.py). Como se diagnostico, ese mensaje se
    dispara para cualquier excepcion que NO sea un 429 HTTP limpio, asi
    que en la practica cubre tanto bloqueos reales de Yahoo como errores
    transitorios (timeouts, JSON invalido, etc.) -- el `Detalle` incluye
    el tipo de excepcion y el status HTTP si esta disponible, para poder
    distinguirlos.
  - INFO_INCOMPLETA: cuando Yahoo devuelve el precio (suficiente para que
    el ticker "valide") pero con campos criticos faltantes en el
    `.info()` -- Nombre y/o cualquiera de los campos que exige el filtro
    de completitud del ranking (Performance 5D/1M/6M/1Y, Sharpe/Treynor/
    Beta, 52W High/Low, Volumen, AUM). El caso mas visible es Nombre
    faltante, que hace que Proveedor caiga al default "Otro" aunque el
    fondo sea de un proveedor conocido (ver etf_discovery/discovery.py,
    _build_result_dict / _get_provider) -- pero cualquiera de estos
    campos faltando probablemente saque al ticker del universo depurado
    en el analisis de ranking (dropna de completitud).
  - ERROR_HISTORIAL: alguna de las 6 llamadas de historial por periodo
    (5d/1m/6m/1y/5y/ytd) o la del indice de mercado (^GSPC) fallo dentro
    de _build_etf_result(). A diferencia de la validacion inicial, estas
    llamadas no tienen reintento/backoff propio -- se registra cual
    periodo especifico fallo y con que excepcion, y despues se re-lanza
    (el comportamiento del Screener no cambia, solo se agrega visibilidad).
  - TICKER_DESCARTADO: el ticker se descarto definitivamente, ya sea
    porque se agotaron los reintentos en la validacion inicial, o porque
    fallo en el enriquecimiento posterior (ERROR_HISTORIAL) y la
    excepcion llego sin manejar hasta _process_batch(). Es el resumen de
    "por que este ticker no aparece en el CSV final".
  - LENTO: el ticker tardo mas de UMBRAL_LENTO_SEGUNDOS en procesarse por
    completo (validacion + enriquecimiento), haya terminado bien o mal --
    puede ser una senal temprana de que Yahoo esta empezando a limitar
    antes de que aparezca un error concreto.

El registro es thread-safe (AutoETFDiscovery.validate_all corre en un
ThreadPoolExecutor) y se acumula en memoria durante toda la corrida; recien
al final, main.py llama a guardar() para volcarlo a un CSV. Si no hubo
ninguna incidencia, guardar() no crea archivo (para no ensuciar la carpeta
con logs vacios en corridas limpias).
"""

import csv
import os
import threading
from datetime import datetime

_lock = threading.Lock()
_incidencias = []

CAMPOS = ["Timestamp", "Tipo", "Ticker", "Detalle"]


def registrar(tipo, ticker, detalle):
    """Registra una incidencia de forma thread-safe.

    tipo: "RATE_LIMIT" o "INFO_INCOMPLETA".
    ticker: el ticker afectado.
    detalle: descripcion legible del evento puntual (intento, tiempo de
        espera, o que campos vinieron faltantes).
    """
    with _lock:
        _incidencias.append(
            {
                "Timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
                "Tipo": tipo,
                "Ticker": ticker,
                "Detalle": detalle,
            }
        )


def cantidad():
    """Cantidad de incidencias acumuladas hasta ahora (thread-safe)."""
    with _lock:
        return len(_incidencias)


def guardar(output_dir, timestamp):
    """Vuelca las incidencias acumuladas a
    <output_dir>/incidencias_<timestamp>.csv (mismo timestamp que usan
    etf_perf_data_<timestamp>.csv/etf_choice_<timestamp>.csv, para poder
    correlacionar facilmente que incidencias corresponden a que corrida).

    Devuelve la ruta del archivo escrito, o None si no hubo ninguna
    incidencia en esta corrida (no se crea archivo vacio)."""
    with _lock:
        incidencias = list(_incidencias)

    if not incidencias:
        return None

    os.makedirs(output_dir, exist_ok=True)
    ruta = os.path.join(output_dir, f"incidencias_{timestamp}.csv")
    with open(ruta, "w", newline="", encoding="utf-8-sig") as f:
        writer = csv.DictWriter(f, fieldnames=CAMPOS)
        writer.writeheader()
        writer.writerows(incidencias)
    return ruta


def reset():
    """Limpia el registro en memoria. Util si el modulo se reutiliza mas
    de una vez en el mismo proceso (ej. tests); el flujo normal (Screener
    corriendo como script/subproceso nuevo cada vez) no lo necesita."""
    with _lock:
        _incidencias.clear()


def describir_excepcion(exc, max_len=300):
    """Devuelve una descripcion legible y corta de una excepcion para
    usar en el campo Detalle: tipo de excepcion, status HTTP si la
    excepcion lo trae adjunto (ej. exc.response.status_code, como en
    requests/urllib3), y el mensaje truncado a max_len caracteres.

    El truncado es deliberado: algunas excepciones de yfinance/requests
    incluyen el cuerpo HTML completo de la respuesta de error de Yahoo,
    que no aporta nada al diagnostico y hace la fila del CSV ilegible."""
    tipo = type(exc).__name__
    status = getattr(getattr(exc, "response", None), "status_code", None)
    mensaje = str(exc).replace("\n", " ").replace("\r", " ")
    if len(mensaje) > max_len:
        mensaje = mensaje[:max_len] + "..."
    if status is not None:
        return f"{tipo} (HTTP {status}): {mensaje}"
    return f"{tipo}: {mensaje}"
