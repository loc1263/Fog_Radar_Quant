"""
Configuración central del pipeline Stock_Tendencia.

Rutas de datos/salida, parámetros de descarga, parámetros del modelo
Markowitz/tendencia, y la carga del universo de tickers (dinámico o de
respaldo). Las listas de respaldo en sí viven en fallback_universe.py
(son datos largos y estáticos; separarlas mantiene este archivo enfocado
en configuración "viva").

UNIVERSO DINÁMICO: desde que se agregó tendencia.universe.discovery
(antes step00_discover_universe.py), el universo de tickers YA NO es una
lista fija. Cada vez que corres el pipeline, ese módulo consulta el
screener de yfinance (capitalización de mercado en tiempo real, por
región) y guarda un snapshot fechado en data/universe/universe_<YYYYMMDD>.json.
Este archivo (se carga más abajo) determina ASIA_TICKERS/US_TICKERS/
SECTOR_MAP para esa corrida. Si ya existe un snapshot de HOY, se reutiliza
(no se vuelve a golpear la API); si cambia el día, se vuelve a descubrir.

IMPORTANTE para quien edite el pipeline: tendencia.pipeline corre el
descubrimiento del universo y hace `importlib.reload(settings)` ANTES de
correr los pasos siguientes, para que todos ellos vean el universo recién
descubierto (Python cachea módulos: sin el reload, los pasos posteriores
seguirían viendo el universo que había al importar `settings` por primera
vez). Todos los módulos del paquete importan este archivo como
`from fog_radar.modules.tendencia import config` (o análogo) y acceden a
sus atributos en tiempo de uso — nunca con `from fog_radar.modules.tendencia.settings
import ALGO` — precisamente para que ese reload se propague.
"""

import glob
import json
import os
import re
from dataclasses import dataclass
from datetime import datetime

from fog_radar.modules.tendencia import paths as _paths
from fog_radar.modules.tendencia.fallback_universe import (
    FALLBACK_ASIA_TICKERS,
    FALLBACK_SECTOR_MAP,
    FALLBACK_US_TICKERS,
)

# ---------------------------------------------------------------------------
# Rutas
# ---------------------------------------------------------------------------
# Fog Radar: todo el estado del módulo vive en data/tendencia/ (ver paths.py):
#   data/tendencia/{raw,processed,macro,universe}/  (= backend/data/ del original)
#   data/tendencia/output/<YYYYMMDD>_<NN>/          (= backend/output/)
#   data/tendencia/logs/, data/tendencia/history.db
BASE_DIR = str(_paths.root())
DATA_DIR = BASE_DIR
RAW_PRICES_DIR = os.path.join(DATA_DIR, "raw", "prices")
RAW_FX_DIR = os.path.join(DATA_DIR, "raw", "fx")
PROCESSED_DIR = os.path.join(DATA_DIR, "processed")
MACRO_DIR = os.path.join(DATA_DIR, "macro")
UNIVERSE_DIR = os.path.join(DATA_DIR, "universe")
OUTPUT_DIR = os.path.join(BASE_DIR, "output")
LOGS_DIR = os.path.join(BASE_DIR, "logs")

PRICES_USD_PATH = os.path.join(PROCESSED_DIR, "prices_usd.csv")

for _d in (RAW_PRICES_DIR, RAW_FX_DIR, PROCESSED_DIR, MACRO_DIR, UNIVERSE_DIR, OUTPUT_DIR, LOGS_DIR):
    os.makedirs(_d, exist_ok=True)


def latest_macro_file():
    """
    Ruta del macro_data_<YYYYMMDD>.csv más reciente que generó
    fetch_macro_data.py, o None si nunca se corrió. Se guarda en
    data/macro/; también se busca en la raíz de backend/, donde lo
    escribían las versiones anteriores del script.
    """
    files = glob.glob(os.path.join(MACRO_DIR, "macro_data_*.csv"))
    files += glob.glob(os.path.join(BASE_DIR, "macro_data_*.csv"))
    return max(files, key=os.path.basename) if files else None


# ---------------------------------------------------------------------------
# Carpeta de salida por corrida: output/<YYYYMMDD>_<NN>
# ---------------------------------------------------------------------------
# Cada corrida del pipeline (markowitz, tendencia, momentum, PDF, dashboard)
# escribe sus resultados en su propia carpeta dentro de output/, con la
# fecha y un número de secuencia de dos dígitos (NN) por si se corre más de
# una vez el mismo día — así una corrida nunca pisa los archivos de otra.
RUN_DIR_RE = re.compile(r"^(\d{8})_(\d{2})$")


def is_run_dir_name(name):
    """True si `name` tiene la forma de una carpeta de corrida (YYYYMMDD_NN)."""
    return bool(RUN_DIR_RE.match(name))


def _today_run_sequences():
    """Secuencias (NN) de las carpetas de corrida de HOY que ya existen en
    output/, ordenadas ascendente. Lista vacía si todavía no hay ninguna."""
    today = datetime.now().strftime("%Y%m%d")
    seqs = []
    if os.path.isdir(OUTPUT_DIR):
        for name in os.listdir(OUTPUT_DIR):
            m = RUN_DIR_RE.match(name)
            if m and m.group(1) == today and os.path.isdir(os.path.join(OUTPUT_DIR, name)):
                seqs.append(int(m.group(2)))
    return today, sorted(seqs)


def get_run_dir(new=False):
    """
    Devuelve (y crea si hace falta) la carpeta de la corrida actual:
    output/<YYYYMMDD>_<NN>.

    - new=False (por defecto): reutiliza la carpeta de la corrida más
      reciente de HOY si ya existe. Así, correr los pasos por separado
      (p. ej. para reanudar tras un bloqueo de Yahoo a mitad de pipeline)
      sigue escribiendo en la misma carpeta de esa corrida.
    - new=True: fuerza una carpeta nueva con la siguiente secuencia, aunque
      ya exista una de hoy. Lo usa `tendencia.pipeline` al arrancar
      una corrida completa, para que cada `python run_pipeline.py` quede
      en su propia carpeta.
    """
    today, seqs = _today_run_sequences()
    if seqs and not new:
        seq = seqs[-1]
    else:
        seq = (seqs[-1] + 1) if seqs else 1
    run_dir = os.path.join(OUTPUT_DIR, f"{today}_{seq:02d}")
    is_new_dir = not os.path.isdir(run_dir)
    os.makedirs(run_dir, exist_ok=True)
    if is_new_dir:
        manifest_path = os.path.join(run_dir, "run_manifest.json")
        with open(manifest_path, "w", encoding="utf-8") as f:
            json.dump({"created_at": datetime.now().isoformat()}, f, indent=2, ensure_ascii=False)
    return run_dir


# ---------------------------------------------------------------------------
# Descubrimiento dinámico del universo (tendencia.universe.discovery)
# ---------------------------------------------------------------------------
# Regiones Yahoo Finance consultadas para "Asia" (códigos de EquityQuery).
# China continental + Hong Kong + Japón + Corea del Sur + Taiwán + India:
# los mismos 5 mercados que cubría la selección manual original.
DISCOVERY_ASIA_REGIONS = {
    "cn": "China",
    "hk": "China (Hong Kong)",
    "jp": "Japón",
    "kr": "Corea del Sur",
    "tw": "Taiwán",
    "in": "India",
}
DISCOVERY_US_REGION = "us"

DISCOVERY_CANDIDATES_PER_REGION = 20  # cuántas se piden por región antes de rankear global
DISCOVERY_ASIA_TOP_N = 20  # tamaño final del universo asiático

# Universo de EEUU: por defecto se toman TODAS las "mega-cap" (definición
# estándar de la industria: capitalización de mercado >= USD 200 mil
# millones), en vez de un top-N fijo por ranking. Con un top-N fijo,
# empresas grandes de verdad (ej. Intel, ~USD 644 mil millones) pueden
# quedar afuera por estar apenas debajo del corte, o por competir por esos
# lugares con ADRs de empresas extranjeras que el screener de Yahoo
# clasifica bajo la región 'us' (ver universe/discovery._select_candidates).
# En USD, NO en millones/billones, para no depender de si "B" significa
# billion (10^9, inglés) o billón (10^12, español) en la config.
DISCOVERY_US_TOP_N = 21  # usado solo si DISCOVERY_US_MEGACAP_MIN_MARKET_CAP es None
DISCOVERY_US_MEGACAP_MIN_MARKET_CAP = 200_000_000_000  # USD 200 mil millones; None = volver al top-N fijo
DISCOVERY_US_MEGACAP_POOL_SIZE = 150  # candidatas crudas a pedirle al screener en modo mega-cap

# La región 'us' del screener trae cualquier instrumento que COTICE en el
# mercado de EEUU, no solo empresas con sede ahí: ADRs/certificados OTC de
# empresas extranjeras (TSMC, SK Hynix, Samsung, HSBC, Toyota, Nestlé,
# etc.) pasan el filtro de región igual que Intel o Apple — confirmado con
# datos reales, ver DISC-5 en mejoras_pendientes.txt. Con esto en True,
# discover_us() consulta Ticker.info (mismo request que ya hace para el
# sector, sin costo extra de red) y descarta las que no tengan sede en
# DISCOVERY_US_HQ_COUNTRY.
DISCOVERY_US_REQUIRE_DOMESTIC_HQ = True
DISCOVERY_US_HQ_COUNTRY = "United States"  # tal como lo devuelve Ticker.info(...)["country"]

# Pares de FX necesarios para convertir precios locales a USD.
# Convención Yahoo: "USDXXX=X" => 1 USD = X unidades de la moneda XXX.
# precio_USD = precio_moneda_local / tasa(USDXXX=X)
CURRENCY_FX_TICKER = {
    "KRW": "USDKRW=X",
    "HKD": "USDHKD=X",
    "CNY": "USDCNY=X",
    "JPY": "USDJPY=X",
    "INR": "USDINR=X",
    "TWD": "USDTWD=X",  # Taiwán — solo puede aparecer con el universo dinámico
}


def fx_ticker_for(currency):
    """Par FX de Yahoo para ``currency`` (None si es USD o no es una moneda válida).

    Fog Radar (TD-DATA-1): los pares de CURRENCY_FX_TICKER tienen prioridad;
    cualquier otra moneda ISO 4217 (o subunidad de Yahoo, p. ej. GBp) usa
    ``USDXXX=X`` vía ``fog_radar.core.fx``.
    """
    from fog_radar.core import fx as _fx

    if currency == "USD":
        return "USD"
    if currency in CURRENCY_FX_TICKER:
        return CURRENCY_FX_TICKER[currency]
    normal = _fx.normalizar_moneda(currency)
    if normal is None:
        return None
    iso, _ = normal
    return CURRENCY_FX_TICKER.get(iso) or _fx.par_usd(iso)


# Tickers auxiliares de referencia (no forman parte del universo invertible,
# se usan para régimen de mercado / tasa libre de riesgo de respaldo).
REFERENCE_TICKERS = {
    "^VIX": "VIX (volatilidad implícita S&P 500)",
    "^IRX": "13-Week T-Bill (proxy tasa libre de riesgo)",
}

# ---------------------------------------------------------------------------
# Parámetros de descarga
# ---------------------------------------------------------------------------
HISTORY_PERIOD = "5y"  # 5 años de historia
HISTORY_INTERVAL = "1wk"  # velas semanales

# Anti rate-limit / anti bloqueo de Yahoo Finance
MIN_SLEEP_BETWEEN_REQUESTS = 1.5  # segundos
MAX_SLEEP_BETWEEN_REQUESTS = 3.5  # segundos (jitter aleatorio)
MAX_RETRIES = 5
BACKOFF_BASE_SECONDS = 5  # backoff exponencial: base * 2^intento + jitter
BACKOFF_MAX_SECONDS = 120
REQUEST_TIMEOUT = 30

# ---------------------------------------------------------------------------
# Parámetros del modelo Markowitz / selección
# ---------------------------------------------------------------------------
TOP_N_SELECTION = 15  # número de acciones a seleccionar (universo Asia + EEUU)
WEEKS_PER_YEAR = 52
L2_GAMMA = 0.15  # regularización L2 para diversificar pesos
MAX_WEIGHT_PER_ASSET = 0.15  # tope de concentración por activo (universo completo)
MIN_WEEKS_REQUIRED = 200  # mínimo de observaciones semanales para incluir un ticker (~4 años)

# Análisis complementario "solo EEUU" (mismo modelo, restringido a
# config.US_TICKERS — ver modeling.markowitz.run_selection). Independiente
# del análisis combinado Asia + EEUU de arriba.
TOP_N_SELECTION_US = 15  # número de acciones a seleccionar dentro del universo EEUU
# Portafolios reducidos que se ofrecen en la sección "Portafolio" del
# dashboard: de las top_n seleccionadas se toman las N de mayor peso en el
# universo y se re-optimiza (máx. Sharpe) solo sobre ellas, para que
# repartan el 100% entre sí. Ver modeling.markowitz.run_reduced_portfolios.
PORTFOLIO_SIZE_OPTIONS = (5, 10)
MIN_US_TICKERS_FOR_SEPARATE_ANALYSIS = 5  # si hay menos tickers de EEUU con datos, se omite esta sección

# Fog Radar (TD-CFG-1): valores pasados por el CLI para esta corrida
# (``python -m fog_radar.jobs tendencia --top-n 20``). Se aplican antes de
# armar ANALYSES y de validar la configuración.
PARAMETROS_SOBRESCRITOS = _paths.overrides()
globals().update(PARAMETROS_SOBRESCRITOS)

# ---------------------------------------------------------------------------
# Parámetros del modelo de tendencia a 6 meses
# ---------------------------------------------------------------------------
FORECAST_HORIZON_WEEKS = 26  # 6 meses ≈ 26 semanas
SHORT_MA_WEEKS = 10
LONG_MA_WEEKS = 40
# Señal técnica de corto plazo ("Señal 3m"): misma regla de cruce de medias
# pero con ventanas más cortas (13 semanas ≈ 3 meses). Solo informativa:
# no interviene en Markowitz ni en la proyección GBM.
SHORT_MA_WEEKS_3M = 4
LONG_MA_WEEKS_3M = 13
MOMENTUM_LOOKBACK_WEEKS = 26

# ---------------------------------------------------------------------------
# Parámetros del análisis de momentum mensual (sobre el universo completo,
# independiente de la selección de Markowitz - ver modeling.momentum)
# ---------------------------------------------------------------------------
MOMENTUM_MONTHS_SHORT = 1  # momentum de 1 mes
MOMENTUM_MONTHS_MED = 3  # momentum de 3 meses
MOMENTUM_MONTHS_LONG = 6  # momentum de 6 meses
MOMENTUM_MONTHS_LONGEST = 12  # momentum de 12 meses
MIN_MONTHS_REQUIRED_MOMENTUM = 13  # 12 meses de retorno + 1 de margen
TOP_N_MOMENTUM = 10  # cuántas acciones reportar en el ranking (mensual y semanal)

# Momentum semanal: misma lógica que el mensual pero sobre velas semanales
# (5 días hábiles, la base nativa de prices_usd.csv) — ventanas de 1, 2, 3
# y 4 semanas, consistencia = proporción de semanas positivas en las
# últimas 4.
MOMENTUM_WEEKS_SHORT = 1  # momentum de 1 semana
MOMENTUM_WEEKS_MED = 2  # momentum de 2 semanas
MOMENTUM_WEEKS_LONG = 3  # momentum de 3 semanas
MOMENTUM_WEEKS_LONGEST = 4  # momentum de 4 semanas
MIN_WEEKS_REQUIRED_MOMENTUM = 5  # 4 semanas de retorno + 1 de margen

# ---------------------------------------------------------------------------
# Carga del universo dinámico (o fallback estático)
# ---------------------------------------------------------------------------
UNIVERSE_SOURCE = "fallback_static"  # se actualiza a "dynamic_yfinance_screener" si carga bien
UNIVERSE_DISCOVERED_AT = None


def _find_universe_file():
    files = sorted(glob.glob(os.path.join(UNIVERSE_DIR, "universe_*.json")))
    if not files:
        return None
    today_str = datetime.now().strftime("%Y%m%d")
    todays_files = [f for f in files if today_str in os.path.basename(f)]
    return todays_files[-1] if todays_files else files[-1]


def _load_universe():
    global UNIVERSE_SOURCE, UNIVERSE_DISCOVERED_AT
    path = _find_universe_file()
    if path is None:
        return FALLBACK_ASIA_TICKERS, FALLBACK_US_TICKERS, FALLBACK_SECTOR_MAP

    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        asia = {t: tuple(v) for t, v in data["asia_tickers"].items()}
        us = {t: tuple(v) for t, v in data["us_tickers"].items()}
        sector = dict(data["sector_map"])
        if not asia or not us:
            raise ValueError("snapshot de universo vacío")
        UNIVERSE_SOURCE = data.get("method", "dynamic_yfinance_screener")
        UNIVERSE_DISCOVERED_AT = data.get("discovered_at")
        return asia, us, sector
    except Exception as e:
        print(f"[settings] No se pudo cargar {path} ({e}); se usa el universo fallback estático.")
        return FALLBACK_ASIA_TICKERS, FALLBACK_US_TICKERS, FALLBACK_SECTOR_MAP


ASIA_TICKERS, US_TICKERS, SECTOR_MAP = _load_universe()
ALL_TICKERS = {**ASIA_TICKERS, **US_TICKERS}


# ---------------------------------------------------------------------------
# Análisis paralelos (modeling.markowitz corre el mismo modelo una vez por
# cada entrada de ANALYSES; modeling.trend_forecast/momentum y
# reporting.pdf_report/dashboard_data iteran esta misma lista para generar
# la proyección/momentum/sección de informe de cada uno). Agregar un
# análisis nuevo (p. ej. "solo Asia") es agregar un AnalysisSpec acá — no
# hace falta tocar modeling/* ni reporting/*.
#
# Va DESPUÉS de ALL_TICKERS a propósito: pipeline.py hace
# `importlib.reload(settings)` tras el descubrimiento del universo (ver
# pipeline.py), lo cual re-ejecuta este archivo completo de punta a punta
# — así ANALYSES queda reconstruida con el universo del día, igual que
# ALL_TICKERS, sin ningún mecanismo adicional.
# ---------------------------------------------------------------------------
@dataclass(frozen=True)
class AnalysisSpec:
    key: str  # sufijo de archivo: "" para el análisis principal, "_us"/"_asia"/... para otros
    label: str  # texto para logs e informes: "Asia + EEUU", "solo EEUU"
    tickers: dict  # universo de tickers de este análisis (subconjunto de ALL_TICKERS)
    top_n: int  # cuántas acciones seleccionar dentro de ese universo
    min_tickers: int = 1  # si hay menos tickers con datos disponibles, se omite el análisis


ANALYSES = [
    AnalysisSpec(key="", label="Asia + EEUU", tickers=ALL_TICKERS, top_n=TOP_N_SELECTION, min_tickers=1),
    AnalysisSpec(
        key="_us",
        label="solo EEUU",
        tickers=US_TICKERS,
        top_n=TOP_N_SELECTION_US,
        min_tickers=MIN_US_TICKERS_FOR_SEPARATE_ANALYSIS,
    ),
]


# ---------------------------------------------------------------------------
# Fog Radar (TD-MODEL-3): factibilidad de la configuración, antes de correr.
# ---------------------------------------------------------------------------
def validar_configuracion():
    """Lista de problemas de la configuración actual (vacía si es factible).

    - Cada análisis necesita al menos ``ceil(1 / MAX_WEIGHT_PER_ASSET)`` tickers para
      que los pesos con tope puedan sumar 100 %, y ``top_n`` no puede superar su universo.
    - ``MAX_WEIGHT_PER_ASSET * top_n >= 1`` (si no, el top_n no alcanza el 100 %).
    - Los tamaños de portafolio reducido tienen que ser menores que ``top_n``.
    """
    problemas = []
    if not 0 < MAX_WEIGHT_PER_ASSET <= 1:
        problemas.append(f"MAX_WEIGHT_PER_ASSET={MAX_WEIGHT_PER_ASSET} debe estar en (0, 1].")
    for spec in ANALYSES:
        if spec.top_n * MAX_WEIGHT_PER_ASSET < 1:
            problemas.append(
                f"[{spec.label}] top_n={spec.top_n} × MAX_WEIGHT_PER_ASSET={MAX_WEIGHT_PER_ASSET} < 1: "
                "los pesos no pueden sumar 100 %."
            )
        if len(spec.tickers) < spec.top_n and len(spec.tickers) >= spec.min_tickers:
            problemas.append(
                f"[{spec.label}] top_n={spec.top_n} es mayor que el universo ({len(spec.tickers)} tickers)."
            )
        for n in PORTFOLIO_SIZE_OPTIONS:
            if n >= spec.top_n:
                problemas.append(f"[{spec.label}] portafolio reducido de {n} >= top_n={spec.top_n}.")
    return problemas
