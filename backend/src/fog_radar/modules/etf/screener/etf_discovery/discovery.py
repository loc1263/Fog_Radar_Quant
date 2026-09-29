"""Descubrimiento y análisis automático de ETFs."""

import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

import pandas as pd
import yfinance as yf

from fog_radar.core.logging import get_logger

from ..analyzers.performance import PerformanceAnalyzer
from ..analyzers.risk_metrics import RiskAnalyzer
from ..config import (
    BATCH_PAUSE_TIME,
    BATCH_SIZE,
    DEFAULT_PROVIDER,
    HISTORY_PERIODS,
    MARKET_INDEX,
    NA_VALUE,
    PROVIDER_KEYWORDS,
    SEPARATOR_LINE,
)
from ..etf_discovery.validators import ETFValidator
from ..file_handlers.reader import CSVReader
from ..models import ReturnsData, RiskMetrics
from ..utils import incidencias

log = get_logger("etf.screener")

# Umbral (segundos) para registrar una incidencia LENTO: si procesar un
# ticker completo (validacion + enriquecimiento) tarda mas que esto, es
# una senal temprana de que Yahoo puede estar empezando a limitar, aunque
# todavia no haya tirado un error concreto.
UMBRAL_LENTO_SEGUNDOS = 10

# Campos que, ademas de Nombre, exige el filtro de completitud del
# ranking (etf_ranking_analysis.py de la raiz / WEB/backend/analysis/,
# columnas PERIOD_COLS + RISK_COLS + OTHER_REQUIRED salvo Precio, que
# nunca queda N/A porque ya se exigio en la validacion). Si a un ticker
# le falta alguno de estos, es muy probable que quede afuera del universo
# depurado del ranking en esta corrida, aunque haya "validado" aca.
# Mantener sincronizado a mano con esas listas si cambian.
CAMPOS_CRITICOS_RANKING = [
    "Performance 5D (%)",
    "Performance 1M (%)",
    "Performance 6M (%)",
    "Performance 1Y (%)",
    "Sharpe Ratio",
    "Treynor Ratio",
    "Beta",
    "52W High",
    "52W Low",
    "Volumen",
    "AUM",
]

# Normaliza las etiquetas de la columna "Company" de List_All_ETF.csv
# (hasta 2026-07-23 se llamaba "Asset Managers") a los mismos nombres de
# Proveedor que ya usa PROVIDER_KEYWORDS (config.py), para que no
# aparezcan como proveedores distintos por una simple diferencia de
# escritura -- ej. la columna trae "iShare" (singular) mientras el resto
# del script usa "iShares".
# Si un valor de Company no esta en este mapa, se usa tal cual (no se
# descarta) en vez de perderlo -- asi que compañías nuevas agregadas al
# CSV (ej. Charles Schwab, JPMorgan) funcionan aunque no esten aca todavia;
# este mapa solo hace falta para unificar variantes de escritura.
NORMALIZACION_COMPANY = {
    "ishare": "iShares",
    "ishares": "iShares",
    "blackrock": "iShares",
    "vanguard": "Vanguard",
    "spdr": "SPDR",
    "state street": "SPDR",
    "invesco": "Invesco",
    "global x": "Global X",
    "vaneck": "VanEck",
    "charles schwab": "Charles Schwab",
    "schwab": "Charles Schwab",
    "jpmorgan": "JPMorgan",
    "jp morgan": "JPMorgan",
    "j.p. morgan": "JPMorgan",
}

# Cache en memoria del historial 1y del indice de mercado (^GSPC).
#
# Antes de este cache, _build_etf_result() pedia yf.Ticker(MARKET_INDEX)
# .history(period="1y") UNA VEZ POR CADA ticker procesado -- para calcular
# Beta/Treynor de cada ETF se necesita el mismo historial del S&P 500, que
# no cambia entre tickers dentro de la misma corrida. Con ~1600 tickers
# validados por corrida, eso son ~1600 pedidos identicos y redundantes a
# Yahoo por el mismo dato exacto: un contribuyente real a la carga total
# sobre la API (y, por lo tanto, a los rate limits que dispara
# validators.py). Detectado 2026-07-17 revisando el log de incidencias.
#
# precargar_historial_mercado() (llamada desde validate_all() ANTES de
# crear el ThreadPoolExecutor, cuando todavia no hay ningun worker
# corriendo) hace el pedido una unica vez de forma sincronica y sin
# competencia -- por eso queda garantizado exactamente 1 pedido real por
# corrida, no "un puñado" como en una primera version de este cache que
# poblaba el cache de forma perezosa (el primer ticker en llegar disparaba
# el pedido) y podia dejar que varios threads arrancaran la carrera al
# mismo tiempo si llegaban practicamente juntos al arranque de la corrida.
_cache_mercado_lock = threading.Lock()
_cache_mercado = {"datos": None}


def precargar_historial_mercado():
    """Pide el historial 1y del indice de mercado UNA sola vez, de forma
    sincronica (se llama desde el thread principal, antes de que exista
    ningun worker concurrente), y lo deja cacheado para toda la corrida.

    Se llama explicitamente al arrancar validate_all(), no de forma
    perezosa: si este pedido falla, toda la corrida esta comprometida (sin
    historial de mercado no se puede calcular Beta/Treynor de NINGUN
    ETF), asi que conviene fallar rapido y con un mensaje claro ahora, en
    vez de dejar que cada ticker lo descubra por su cuenta uno por uno
    (que era el comportamiento antes de este cache: si el indice fallaba,
    CADA ticker terminaba con su propio TICKER_DESCARTADO, todos por la
    misma causa raiz)."""
    datos = yf.Ticker(MARKET_INDEX).history(period=HISTORY_PERIODS["1y"])
    with _cache_mercado_lock:
        _cache_mercado["datos"] = datos
    return datos


def _obtener_historial_mercado():
    """Devuelve el historial de mercado ya cacheado por
    precargar_historial_mercado(). Sirve tambien como red de seguridad
    (con el mismo patron de lock de siempre) por si algo llama a
    _build_etf_result() sin haber precargado antes (ej. los tests
    unitarios que llaman a _build_etf_result() de forma aislada)."""
    with _cache_mercado_lock:
        if _cache_mercado["datos"] is not None:
            return _cache_mercado["datos"]

    return precargar_historial_mercado()


def _reset_cache_mercado():
    """Limpia el cache del indice de mercado. Util si se corre mas de una
    vez en el mismo proceso (ej. tests); el flujo normal (Screener
    corriendo como script/subproceso nuevo cada vez) no lo necesita."""
    with _cache_mercado_lock:
        _cache_mercado["datos"] = None


class AutoETFDiscovery:
    """Descubre y analiza ETFs automáticamente."""

    def __init__(self):
        self.validated = []
        self.invalid = []
        self.tickers = []
        self.metadata_local = {}

    def get_etf_tickers_from_csv(self, csv_file):
        """Obtiene tickers de ETFs desde un CSV. De paso, guarda el
        Nombre/Company/Category que ese mismo archivo trae para cada ticker
        (columnas Name/Company/Category de List_All_ETF.csv) en
        self.metadata_local -- se usan como respaldo/prioridad en
        _build_result_dict()/_get_provider() cuando Yahoo no devuelve esos
        datos, o directamente en vez de pedirlos a Yahoo (ver
        CSVReader.read_ticker_metadata)."""
        self.metadata_local = CSVReader.read_ticker_metadata(csv_file)
        return CSVReader.read_tickers_from_csv(csv_file)

    def validate_all(self, tickers):
        """Valida todos los tickers usando concurrencia."""
        _reset_cache_mercado()
        log.info(f"Precargando historial de {MARKET_INDEX} (una sola vez para toda la corrida)...")
        precargar_historial_mercado()
        log.info("[OK] Historial de mercado precargado.")

        self.tickers = tickers
        total = len(self.tickers)

        with ThreadPoolExecutor() as executor:
            processed = 0
            for batch_start in range(0, total, BATCH_SIZE):
                batch = self.tickers[batch_start : batch_start + BATCH_SIZE]
                self._process_batch(executor, batch, processed, total)
                processed += len(batch)

                if batch_start + BATCH_SIZE < total:
                    log.info("⏸️  Pausa de 30 segundos para evitar bloqueo...")
                    time.sleep(BATCH_PAUSE_TIME)

        log.info(SEPARATOR_LINE)
        self.print_summary()

        return pd.DataFrame(self.validated)

    def _process_batch(self, executor, batch, processed, total):
        """Procesa un lote de tickers."""
        futures = {executor.submit(self._validate_and_build, ticker): ticker for ticker in batch}

        for future in as_completed(futures):
            ticker = futures[future]
            processed += 1
            percent = processed / total * 100

            try:
                result = future.result()
                if result:
                    self.validated.append(result)
                    log.info(
                        f"[{processed}/{total} | {percent:5.1f}%] "
                        f"{result['Ticker']:<8} "
                        f"{result['Proveedor']:<10} "
                        f"{result['Nombre'][:35]}"
                    )
                else:
                    self.invalid.append(ticker)
                    log.info(f"[{processed}/{total} | {percent:5.1f}%] ✗ {ticker}")
            except Exception as e:
                self.invalid.append(ticker)
                incidencias.registrar(
                    "TICKER_DESCARTADO",
                    ticker,
                    f"fallo durante el enriquecimiento (historial/mercado), despues "
                    f"de pasar la validacion inicial. {incidencias.describir_excepcion(e)}",
                )
                log.info(f"[{processed}/{total} | {percent:5.1f}%] Error {ticker}: {e}")

    def _validate_and_build(self, ticker):
        """Valida un ETF y construye su resultado completo. Mide el tiempo
        total (validacion + enriquecimiento) y registra una incidencia
        LENTO si supera UMBRAL_LENTO_SEGUNDOS, haya terminado bien o mal --
        una senal util de throttling de Yahoo antes de que se traduzca en
        un error concreto."""
        inicio = time.time()
        try:
            etf_data = ETFValidator.validate_and_enrich(ticker)
            if not etf_data:
                return None

            return self._build_etf_result(etf_data)
        finally:
            duracion = time.time() - inicio
            if duracion > UMBRAL_LENTO_SEGUNDOS:
                incidencias.registrar(
                    "LENTO",
                    ticker,
                    f"proceso completo tardo {duracion:.1f}s (umbral: {UMBRAL_LENTO_SEGUNDOS}s)",
                )

    def _build_etf_result(self, etf_data):
        """Construye el diccionario de resultado del ETF."""
        # etf_data.price (no etf_data.hist["Close"].iloc[-1]) -- Yahoo a
        # veces agrega al historial una fila del dia en curso con Close en
        # NaN (el cierre todavia no esta publicado cuando corre el
        # Screener), lo que dejaba current_price en NaN y, con eso, las 6
        # columnas de Performance vacias para el ticker entero (detectado
        # 2026-09-17: 912 ETFs de esa corrida con Performance NaN pero
        # Sharpe/Beta validos, ya que esos se calculan con hist_1y, un
        # historial separado que no tiene este problema). etf_data.price
        # viene de etf_info (regularMarketPrice/currentPrice/
        # previousClose), la misma fuente que ya usa el campo "Precio" del
        # CSV y que nunca tuvo este problema.
        current_price = etf_data.price

        especificacion_periodos = [
            ("hist_5d", "5d"),
            ("hist_1m", "1mo"),
            ("hist_6m", HISTORY_PERIODS["6mo"]),
            ("hist_1y", HISTORY_PERIODS["1y"]),
            ("hist_5y", HISTORY_PERIODS["5y"]),
            ("hist_ytd", HISTORY_PERIODS["ytd"]),
        ]
        hist_periods = {}
        for clave, periodo in especificacion_periodos:
            try:
                hist_periods[clave] = yf.Ticker(etf_data.ticker).history(period=periodo)
            except Exception as e:
                # No hay reintento/backoff aca (a diferencia de la
                # validacion inicial) -- se deja constancia de exactamente
                # que periodo fallo y se re-lanza sin cambiar el
                # comportamiento: _process_batch va a atrapar esto igual
                # que antes y descartar el ticker.
                incidencias.registrar(
                    "ERROR_HISTORIAL",
                    etf_data.ticker,
                    f"fallo history(period={periodo!r}) para {clave}. {incidencias.describir_excepcion(e)}",
                )
                raise

        perf = PerformanceAnalyzer.get_performance_metrics(current_price, hist_periods)

        hist_1y = hist_periods["hist_1y"]
        try:
            market_history_data = _obtener_historial_mercado()
        except Exception as e:
            incidencias.registrar(
                "ERROR_HISTORIAL",
                etf_data.ticker,
                f"fallo history() del indice de mercado {MARKET_INDEX}. {incidencias.describir_excepcion(e)}",
            )
            raise

        returns_data = ReturnsData(*perf)
        risk_metrics = RiskMetrics(
            sharpe_ratio=RiskAnalyzer.calculate_sharpe_ratio(hist_1y),
            treynor_ratio=RiskAnalyzer.calculate_treynor_ratio(hist_1y, market_history_data),
            beta=RiskAnalyzer.calculate_beta(hist_1y, market_history_data),
        )

        return self._build_result_dict(etf_data, returns_data, risk_metrics)

    def _build_result_dict(self, etf_data, returns_data, risk_metrics):
        """Construye el diccionario completo de resultados."""
        etf_info = etf_data.info
        tipo_instrumento = etf_info.get("quoteType", "")
        # En Yahoo Finance, ETFs suelen reportar el fee en netExpenseRatio.
        # annualReportExpenseRatio aparece con mayor frecuencia en fondos mutuos.
        expense_ratio = (
            etf_info.get("netExpenseRatio")
            or etf_info.get("annualReportExpenseRatio")
            or etf_info.get("totalExpenseRatio")
            or etf_info.get("expenseRatio")
            or NA_VALUE
        )
        nombre = etf_info.get("longName", etf_info.get("shortName", NA_VALUE))
        metadata_ticker = self.metadata_local.get(etf_data.ticker, {})

        # Respaldo: si Yahoo no trajo el nombre, usar el que ya viene en
        # List_All_ETF.csv (columna Name) -- ese archivo es 100% completo
        # para los tickers que el propio Screener procesa (son los mismos
        # tickers que salen de ahi), a diferencia de Yahoo que
        # intermitentemente no devuelve longName/shortName para ETFs bien
        # conocidos (ver seccion "Log de incidencias" del README).
        if nombre in (NA_VALUE, None):
            nombre_local = metadata_ticker.get("Name")
            if nombre_local:
                nombre = nombre_local

        # Category: prioridad a la columna Category de List_All_ETF.csv
        # (curada a mano por el usuario) por sobre etf_info["category"] de
        # Yahoo -- mismo criterio que ya se aplica a Nombre/Proveedor mas
        # arriba/abajo: preferir el dato local cuando esta disponible, y
        # solo pedirselo a la API cuando el CSV no lo trae.
        category_local = metadata_ticker.get("Category")
        category = category_local if category_local else etf_info.get("category", NA_VALUE)

        result = {
            "Ticker": etf_data.ticker,
            "TipoInstrumento": tipo_instrumento,
            "Nombre": nombre,
            "Precio": round(etf_data.price, 2),
            "Volumen": etf_info.get("volume", NA_VALUE),
            "AUM": etf_info.get("totalAssets", NA_VALUE),
            "Expense Ratio": expense_ratio,
            "Inception Date": etf_info.get("fundInceptionDate", NA_VALUE),
            "Category": category,
            "Performance 5D (%)": PerformanceAnalyzer.format_performance(returns_data.perf_5d),
            "Performance 1M (%)": PerformanceAnalyzer.format_performance(returns_data.perf_1m),
            "Performance 6M (%)": PerformanceAnalyzer.format_performance(returns_data.perf_6m),
            "Performance YTD (%)": PerformanceAnalyzer.format_performance(returns_data.perf_ytd),
            "Performance 1Y (%)": PerformanceAnalyzer.format_performance(returns_data.perf_1y),
            "Performance 5Y (%)": PerformanceAnalyzer.format_performance(returns_data.perf_5y),
            "52W High": etf_info.get("fiftyTwoWeekHigh", NA_VALUE),
            "52W Low": etf_info.get("fiftyTwoWeekLow", NA_VALUE),
            "Sharpe Ratio": (
                risk_metrics.sharpe_ratio if risk_metrics.sharpe_ratio is not None else NA_VALUE
            ),
            "Treynor Ratio": (
                risk_metrics.treynor_ratio if risk_metrics.treynor_ratio is not None else NA_VALUE
            ),
            "Beta": (risk_metrics.beta if risk_metrics.beta is not None else NA_VALUE),
        }
        result["Proveedor"] = self._get_provider(result["Nombre"], metadata_ticker.get("Company"))

        # CAMPOS_CRITICOS_RANKING solo aplica si el instrumento es un ETF:
        # son justamente los campos que exige el filtro de completitud del
        # ranking, que de entrada solo mira TipoInstrumento=="ETF" (ver
        # etf_ranking_analysis.py). Un MUTUALFUND legitimamente no tiene
        # Volumen (no cotiza en bolsa, se compra/rescata a NAV) y una
        # EQUITY legitimamente no tiene AUM (no es un fondo) -- verificado
        # 2026-07-17 contra una corrida real: el chequeo sin este scope
        # generaba 743 incidencias, de las cuales 554 eran MUTUALFUND/
        # EQUITY marcados por campos que nunca les iban a aplicar (ruido
        # puro, no un problema real de recoleccion). Nombre en cambio se
        # revisa siempre, sea cual sea el TipoInstrumento.
        #
        # in (NA_VALUE, None) y no solo "== NA_VALUE": Volumen/AUM/52W
        # High/Low se arman con etf_info.get(clave, NA_VALUE), que solo
        # aplica el default si la CLAVE no esta -- si Yahoo la devuelve
        # presente pero en None, el resultado queda en None (no en "N/A"),
        # y igual termina como NaN al leer el CSV con pandas mas adelante
        # (ambos son valores default de pandas para celdas faltantes).
        campos_a_revisar = ["Nombre"]
        if tipo_instrumento == "ETF":
            campos_a_revisar += CAMPOS_CRITICOS_RANKING
        campos_faltantes = [campo for campo in campos_a_revisar if result.get(campo) in (NA_VALUE, None)]
        if campos_faltantes:
            incidencias.registrar(
                "INFO_INCOMPLETA",
                etf_data.ticker,
                "Yahoo no devolvio: " + ", ".join(campos_faltantes) + ". Salvo "
                "Nombre, estos son campos que exige el filtro de completitud "
                "del ranking -- es probable que este ticker quede afuera del "
                "universo depurado en esta corrida.",
            )

        return result

    def _get_provider(self, nombre, company_local=None):
        """Obtiene el proveedor del ETF.

        Prioridad: 1) la columna Company de List_All_ETF.csv, si esta
        disponible para este ticker -- es un dato curado por el usuario,
        mas confiable que adivinar por palabra clave dentro del nombre que
        devuelve Yahoo (que ademas puede venir faltante, ver
        _build_result_dict). 2) keyword-matching contra Nombre (el
        comportamiento original), como respaldo si el ticker no esta en
        el CSV de entrada o no tiene Company cargado."""
        if company_local:
            normalizado = NORMALIZACION_COMPANY.get(company_local.strip().lower())
            return normalizado if normalizado else company_local

        name_lower = nombre.lower()
        for keyword, provider in PROVIDER_KEYWORDS.items():
            if keyword in name_lower:
                return provider
        return DEFAULT_PROVIDER

    def print_summary(self):
        """Imprime resumen de validación."""
        log.info(f"Válidos: {len(self.validated)}")
        log.info(f"Inválidos: {len(self.invalid)}")
        if self.invalid:
            log.info(f"Tickers inválidos: {', '.join(self.invalid[:10])}")
            if len(self.invalid) > 10:
                log.info(f"    ... y {len(self.invalid) - 10} más")
