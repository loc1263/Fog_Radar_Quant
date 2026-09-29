"""
Descubrimiento DINÁMICO del universo de tickers.

En vez de una lista fija de "las mayores empresas", este módulo consulta
el screener de Yahoo Finance (vía yfinance.EquityQuery / yfinance.screen)
para obtener, en el momento en que corres el script, las empresas con
mayor capitalización de mercado en:

  - EEUU (región 'us'): por defecto TODAS las "mega-cap" (capitalización
    >= config.DISCOVERY_US_MEGACAP_MIN_MARKET_CAP, USD 200 mil millones
    por defecto — la definición estándar de la industria), no un top-N
    fijo. Con un top-N fijo, una mega-cap real (ej. Intel) puede quedar
    afuera por estar apenas debajo del corte. Ver discover_us() y
    _select_candidates(). Se puede volver al modo top-N fijo poniendo
    `DISCOVERY_US_MEGACAP_MIN_MARKET_CAP = None` en settings.py.

    Además, la región 'us' del screener trae cualquier instrumento que
    COTICE en el mercado de EEUU, no solo empresas con sede ahí: ADRs y
    certificados OTC de empresas extranjeras (TSMC, SK Hynix, Samsung,
    HSBC, Toyota, Nestlé, etc.) pasan el filtro de región igual que Intel
    o Apple — confirmado con datos reales (36 de 93 candidatas "mega-cap"
    en una corrida tenían sede fuera de EEUU). Por eso discover_us()
    también filtra por sede real vía Ticker.info
    (config.DISCOVERY_US_REQUIRE_DOMESTIC_HQ, default True) — ver
    _exclude_foreign_hq().
  - Asia: China, Hong Kong, Japón, Corea del Sur, Taiwán e India (regiones
    'cn', 'hk', 'jp', 'kr', 'tw', 'in'), rankeadas juntas por capitalización
    (sin cuota fija por país: si dentro de un año China tiene más empresas
    gigantes que Japón, el universo lo va a reflejar tal cual). Acá sí se
    usa un top-N fijo (config.DISCOVERY_ASIA_TOP_N).

Esto hace que el universo sea dinámico de verdad: si corres el pipeline
dentro de un año, va a descubrir las empresas más grandes de ESE momento,
no las de agosto 2026.

Caché por fecha: si ya existe un snapshot de HOY en
data/universe/universe_<YYYYMMDD>.json, se reutiliza (no se vuelve a
golpear el screener). Esto es intencional: coincide con la lógica de caché
que ya usa net.robust_yf para precios, y evita re-descubrir el universo en
cada corrida dentro del mismo día (lo cual además podría dejar inconsistente
el pipeline si ya bajaste precios para el universo de hoy y luego cambia a
mitad del día).

ADVERTENCIA: el screener de yfinance usa un endpoint interno de Yahoo
Finance NO documentado oficialmente (más propenso a cambiar de forma que la
descarga de precios históricos). Por eso este módulo tiene un fallback
robusto: si el screener falla (red, cambio de API, bloqueo, respuesta
vacía/incompleta), se usa automáticamente la lista estática de respaldo
definida en fallback_universe.py y se guarda igual un snapshot (marcado
como "fallback_static") para que el resto del pipeline tenga un universo
consistente con el que trabajar.

Sector/industria: el endpoint del screener (yf.screen) NO incluye sector
ni industria en su respuesta (confirmado inspeccionando quotes crudos
reales, no es un problema de nombre de campo distinto). Ese dato vive en
otro endpoint de Yahoo (quoteSummary/assetProfile), al que yfinance da
acceso vía Ticker.info. Por eso discover_asia()/discover_us() hacen una
segunda pasada (_enrich_missing_sectors) que consulta Ticker.info una vez
por cada ticker del universo ya recortado a top_n, y solo así se llena
sector_map con datos reales en vez de "Otro".

Ejecutar (vía wrapper de compatibilidad en la raíz del proyecto):
    python step00_discover_universe.py
    python step00_discover_universe.py --force        (ignora la caché de hoy)

Genera:
    data/universe/universe_<YYYYMMDD>.json
"""

import argparse
import json
import os
import random
import re
import time
from datetime import datetime

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.logging_utils import get_logger

logger = get_logger("step00", config.LOGS_DIR, log_filename="step00_discover_universe.log")

TODAY_STR = datetime.now().strftime("%Y%m%d")

# Sufijos societarios a eliminar al normalizar nombres de empresa para
# deduplicación (ver _dedupe_by_company). \b = límite de palabra, para no
# empatar parcialmente dentro de otra palabra (ej. "CO" dentro de "CORP").
_NAME_SUFFIX_RE = re.compile(
    r"\b(LTD|LIMITED|CORPORATION|CORP|CO|INC|HOLDINGS|HOLDING|GROUP|CLASS\s+[AH]|\(ADR\))\b"
)


def _today_snapshot_path():
    return os.path.join(config.UNIVERSE_DIR, f"universe_{TODAY_STR}.json")


def _screen_region(region_code, size, max_retries=3):
    """
    Pide al screener de yfinance las 'size' empresas de mayor capitalización
    de mercado en una región, con reintentos/backoff (mismo espíritu que
    net.robust_yf, pero más corto: hay hasta 7 regiones por corrida, así que
    un backoff largo por región puede volverse muy lento si Yahoo está caído
    o bloqueado por completo). Devuelve la lista cruda de 'quotes' (dicts) de
    Yahoo, o [] si falla tras agotar los reintentos.
    """
    import yfinance as yf

    query = yf.EquityQuery("eq", ["region", region_code])
    last_error = None
    for attempt in range(max_retries):
        try:
            result = yf.screen(
                query,
                offset=0,
                size=size,
                sortField="intradaymarketcap",
                sortAsc=False,
            )
            quotes = result.get("quotes", []) if isinstance(result, dict) else []
            if not quotes:
                raise ValueError("respuesta sin 'quotes' (posible cambio de API o región vacía)")
            time.sleep(random.uniform(config.MIN_SLEEP_BETWEEN_REQUESTS, config.MAX_SLEEP_BETWEEN_REQUESTS))
            return quotes
        except Exception as e:
            last_error = e
            logger.warning(f"[región {region_code}] intento {attempt + 1}/{max_retries} falló: {e}")
            if attempt < max_retries - 1:
                time.sleep(3 * (2**attempt) + random.uniform(0, 2))
    logger.error(f"[región {region_code}] descubrimiento falló tras {max_retries} intentos: {last_error}")
    return []


def _quote_value(quote, *keys, default=None):
    """Yahoo a veces anida valores numéricos como {'raw': x, 'fmt': '...'}."""
    for key in keys:
        if key in quote and quote[key] is not None:
            v = quote[key]
            if isinstance(v, dict):
                v = v.get("raw", default)
            return v
    return default


def _quote_sector(quote):
    """
    Extrae el sector de un quote crudo del screener, SI es que viene
    incluido (ver nota más abajo: en la práctica, a 2026, el endpoint del
    screener (yf.screen) no trae sector/industria para ningún candidato
    probado — se confirmó inspeccionando quotes crudos reales de Asia y
    EEUU, volcados por _warn_if_sector_schema_looks_broken en
    data/universe/_debug_raw_quote_*.json: ninguno de los dos trae
    "sector", "sectorDisp", "industry" ni "industryDisp"). Esta función se
    mantiene como intento gratuito (sin red) por si Yahoo llega a incluir
    el campo en el futuro; el enriquecimiento real ocurre en
    _enrich_missing_sectors(), que consulta Ticker.info (endpoint
    quoteSummary/assetProfile, distinto al del screener) para los
    candidatos que sigan en "Otro" tras este intento.
    """
    for key in ("sectorDisp", "sector", "industryDisp", "industry"):
        v = quote.get(key)
        if isinstance(v, dict):
            v = v.get("raw") or v.get("fmt")
        if v:
            return v
    return "Otro"


def _normalize_quote(quote, country_label):
    symbol = quote.get("symbol")
    if not symbol:
        return None
    name = quote.get("longName") or quote.get("shortName") or quote.get("displayName") or symbol
    market_cap = _quote_value(quote, "marketCap", "intradaymarketcap")
    currency = quote.get("currency")
    sector = _quote_sector(quote)
    quote_type = quote.get("quoteType", "EQUITY")
    if market_cap is None or currency is None:
        return None
    if quote_type not in ("EQUITY", None):
        return None
    return {
        "symbol": symbol,
        "name": name,
        "country": country_label,
        "currency": currency,
        "market_cap": float(market_cap),
        "sector": sector,
    }


def _dedupe_by_company(candidates):
    """
    Evita que la misma empresa aparezca dos veces (ej. doble listado
    A-shares/H-shares en China+HK). Heurística simple: normaliza el nombre
    (primeros ~20 caracteres, sin sufijos comunes) y se queda con el
    listado de mayor capitalización por grupo.
    """

    def norm_name(n):
        # Usa \b (límite de palabra) en vez de str.replace() con substrings:
        # con str.replace(), el sufijo " CO" también empataba dentro de
        # " CORP" (ej. "Big Bank Corp" -> "Big BankRP" en vez de
        # "Big Bank"), rompiendo la deduplicación entre listados de la
        # misma empresa cuyo nombre difiere solo en el sufijo societario.
        n = n.upper()
        n = _NAME_SUFFIX_RE.sub("", n)
        n = re.sub(r"[.,]", "", n)
        return re.sub(r"\s+", " ", n).strip()[:20]

    best_by_name = {}
    for c in candidates:
        key = norm_name(c["name"])
        if key not in best_by_name or c["market_cap"] > best_by_name[key]["market_cap"]:
            best_by_name[key] = c
    return list(best_by_name.values())


def _warn_if_sector_still_missing(candidates, raw_sample, label):
    """
    A diferencia de una versión anterior de este módulo, YA NO se avisa
    si el screener (yf.screen) devuelve "Otro" para el sector: se
    confirmó (inspeccionando quotes crudos reales, ver docstring de
    _enrich_missing_sectors) que ESE endpoint nunca trae sector/industria
    — es el comportamiento normal y esperado en cada corrida, no un
    cambio de schema, así que avisar ahí generaba un WARNING ruidoso e
    inevitable en el 100% de las corridas sin aportar ninguna señal (el
    dato real siempre se resuelve después, vía Ticker.info).

    La señal que sí vale la pena vigilar es esta: si DESPUÉS de
    _enrich_missing_sectors() (que sí consulta el endpoint que
    normalmente tiene el sector) una fracción grande de candidatos sigue
    en "Otro", algo real está fallando (bloqueo/rate-limit persistente,
    tickers deslistados, o un cambio de schema en Ticker.info esta vez).
    Eso es lo que reporta este chequeo, con un volcado de una respuesta
    cruda de Ticker.info de ejemplo para diagnosticar.
    """
    if not candidates:
        return
    otro = sum(1 for c in candidates if c["sector"] == "Otro")
    if otro == 0:
        return
    frac = otro / len(candidates)
    if frac <= 0.3:
        # Unos pocos tickers sueltos fallando es esperable (rate-limit puntual,
        # símbolo recién agregado sin perfil todavía, etc.); no amerita alarma.
        logger.info(
            f"[{label}] {otro}/{len(candidates)} tickers quedaron sin sector tras "
            "consultar Ticker.info (probablemente rate-limit puntual en esos símbolos)."
        )
        return

    logger.warning(
        f"[{label}] {otro}/{len(candidates)} tickers quedaron con sector 'Otro' INCLUSO "
        "después de consultar Ticker.info — a diferencia del screener (que nunca trae "
        "sector, eso es normal), este endpoint sí debería tenerlo. Posible bloqueo/rate-limit "
        "persistente o cambio de schema en Ticker.info (ver _enrich_missing_sectors en "
        "universe/discovery.py). El resto de los datos (ticker, nombre, market cap, moneda) "
        "no se ve afectado."
    )
    if raw_sample is None:
        return
    debug_path = os.path.join(config.UNIVERSE_DIR, f"_debug_raw_tickerinfo_{label.lower()}.json")
    try:
        with open(debug_path, "w", encoding="utf-8") as f:
            json.dump(raw_sample, f, indent=2, ensure_ascii=False, default=str)
        logger.warning(
            f"[{label}] Se guardó una respuesta cruda de Ticker.info en {debug_path} "
            "para diagnosticar qué está pasando."
        )
    except Exception as e:
        logger.warning(f"[{label}] No se pudo guardar la respuesta de diagnóstico: {e}")


def _enrich_missing_sectors(candidates, label="", capture_hq_country=False):
    """
    Completa el sector real de cada candidato que quedó en "Otro" después
    de _quote_sector(), consultando yfinance Ticker.info (endpoint
    quoteSummary/assetProfile de Yahoo) UNA vez por ticker.

    Por qué esto es necesario: el screener (yf.screen, usado en
    _screen_region) y el endpoint de perfil de empresa (Ticker.info) son
    dos endpoints distintos de Yahoo. Se confirmó inspeccionando quotes
    crudos reales del screener (histórico en
    data/universe/_debug_raw_quote_*.json de corridas anteriores) que el
    screener NO incluye sector/industria en absoluto — no es un problema
    de nombre de campo, el dato simplemente no está ahí, y eso es
    esperado en TODAS las corridas (no amerita WARNING, ver
    _warn_if_sector_still_missing). Por eso hace falta esta segunda
    pasada contra Ticker.info.

    Se aplica solo sobre 'candidates' YA recortado a top_n (no sobre
    todos los resultados crudos del screener antes de deduplicar/rankear)
    para minimizar la cantidad de requests adicionales a Yahoo. Es
    secuencial con jitter (mismo criterio anti-bloqueo que net.robust_yf)
    porque Ticker.info es tan sensible a rate-limiting como la descarga
    de históricos.

    Devuelve una respuesta cruda de Ticker.info de ejemplo (el primer
    `get_info()` que no haya lanzado excepción), para que el llamador
    pueda volcarla a disco si el enriquecimiento deja demasiados
    candidatos sin sector (ver _warn_if_sector_still_missing).

    Si `capture_hq_country=True`, además de sector, guarda en cada
    candidato `c["hq_country"]` = `info.get("country")` (la sede real de
    la empresa, ej. "United States", "Taiwan", "China" — un campo
    distinto de `c["country"]`, que es la REGIÓN CONSULTADA en el
    screener, no la sede). Lo usa discover_us() para filtrar ADRs/
    certificados de empresas extranjeras que el screener de Yahoo
    clasifica bajo la región 'us' (por dónde cotiza el instrumento) aunque
    la empresa no sea estadounidense — ver DISC-5 en mejoras_pendientes.txt.
    """
    pending = [
        c for c in candidates if c["sector"] == "Otro" or (capture_hq_country and "hq_country" not in c)
    ]
    if not pending:
        return None

    import yfinance as yf

    logger.info(
        f"[{label}] enriqueciendo sector/industria de {len(pending)} tickers "
        "vía Ticker.info (endpoint separado del screener)..."
    )
    raw_sample = None
    for i, c in enumerate(pending, start=1):
        symbol = c["symbol"]
        sector = "Otro"
        for attempt in range(2):
            try:
                info = yf.Ticker(symbol).get_info()
                if raw_sample is None:
                    raw_sample = info
                sector = info.get("sector") or info.get("industry") or "Otro"
                if capture_hq_country:
                    c["hq_country"] = info.get("country")
                break
            except Exception as e:
                logger.warning(f"[{label}] [sector] {symbol}: intento {attempt + 1}/2 falló: {e}")
                if attempt == 0:
                    time.sleep(2 + random.uniform(0, 1))
        c["sector"] = sector
        if capture_hq_country:
            # Si las 2 llamadas fallaron, no hay dato: se deja en None
            # ("desconocido") en vez de asumir que es extranjera — mejor
            # conservar una candidata dudosa que descartar una empresa de
            # EEUU real por una falla transitoria de red.
            c.setdefault("hq_country", None)
        logger.info(f"[{label}] [sector] ({i}/{len(pending)}) {symbol}: {sector}")
        time.sleep(random.uniform(config.MIN_SLEEP_BETWEEN_REQUESTS, config.MAX_SLEEP_BETWEEN_REQUESTS))
    return raw_sample


def _select_candidates(candidates, top_n, min_market_cap=None):
    """
    Ordena `candidates` (dicts con "market_cap", ya deduplicados) por
    capitalización descendente y decide cuáles quedan en el universo
    final:

      - `min_market_cap` dado (modo "mega-cap"): TODAS las que superen ese
        piso, sin importar cuántas sean — el tamaño del universo deja de
        ser fijo, depende de cuántas empresas reales superen el piso ese
        día (ver settings.DISCOVERY_US_MEGACAP_MIN_MARKET_CAP).
      - `min_market_cap=None` (modo top-N, comportamiento clásico): las
        `top_n` de mayor capitalización, ni una más ni una menos.

    Extraída de discover_us()/discover_asia() como función pura (sin red)
    para poder testearla con listas de dicts a mano.
    """
    ranked = sorted(candidates, key=lambda c: c["market_cap"], reverse=True)
    if min_market_cap is not None:
        return [c for c in ranked if c["market_cap"] >= min_market_cap]
    return ranked[:top_n]


def discover_asia(top_n=None, per_region=None):
    top_n = top_n or config.DISCOVERY_ASIA_TOP_N
    per_region = per_region or config.DISCOVERY_CANDIDATES_PER_REGION

    candidates = []
    for region_code, country_label in config.DISCOVERY_ASIA_REGIONS.items():
        quotes = _screen_region(region_code, per_region)
        for q in quotes:
            norm = _normalize_quote(q, country_label)
            if norm:
                candidates.append(norm)
        logger.info(f"[Asia] región '{region_code}' ({country_label}): {len(quotes)} resultados crudos")

    if not candidates:
        return []

    candidates = _dedupe_by_company(candidates)
    top = _select_candidates(candidates, top_n)
    raw_sample = _enrich_missing_sectors(top, label="Asia")
    _warn_if_sector_still_missing(top, raw_sample, label="Asia")
    return top


def discover_us(top_n=None, size=None, min_market_cap=None):
    """
    `min_market_cap=None` (default) usa
    `config.DISCOVERY_US_MEGACAP_MIN_MARKET_CAP`: si esa config también es
    None, se comporta como antes (top `top_n` por ranking); si tiene un
    valor (USD, por defecto 200 mil millones = "mega-cap"), devuelve TODAS
    las empresas de EEUU con capitalización igual o mayor a ese piso —
    ver _select_candidates.

    Independientemente del modo de arriba, si
    `config.DISCOVERY_US_REQUIRE_DOMESTIC_HQ` es True (default), después
    de seleccionar se descartan las candidatas cuya sede real (vía
    Ticker.info) no sea `config.DISCOVERY_US_HQ_COUNTRY` — ver
    _exclude_foreign_hq. El screener clasifica bajo la región 'us'
    cualquier instrumento que cotice ahí (ADRs de TSMC, SK Hynix, Samsung,
    etc.), no solo empresas con sede en EEUU.
    """
    top_n = top_n or config.DISCOVERY_US_TOP_N
    if min_market_cap is None:
        min_market_cap = config.DISCOVERY_US_MEGACAP_MIN_MARKET_CAP

    if size is None:
        size = (
            max(config.DISCOVERY_US_MEGACAP_POOL_SIZE, config.DISCOVERY_CANDIDATES_PER_REGION)
            if min_market_cap is not None
            else max(top_n + 10, config.DISCOVERY_CANDIDATES_PER_REGION)
        )

    quotes = _screen_region(config.DISCOVERY_US_REGION, size)
    candidates = []
    for q in quotes:
        norm = _normalize_quote(q, "EEUU")
        if norm:
            candidates.append(norm)

    candidates = _dedupe_by_company(candidates)
    top = _select_candidates(candidates, top_n, min_market_cap)
    if min_market_cap is not None:
        logger.info(
            f"[EEUU] modo mega-cap (capitalización >= ${min_market_cap:,.0f}): "
            f"{len(top)} empresas de {len(candidates)} candidatas evaluadas"
        )

    raw_sample = _enrich_missing_sectors(
        top, label="EEUU", capture_hq_country=config.DISCOVERY_US_REQUIRE_DOMESTIC_HQ
    )
    _warn_if_sector_still_missing(top, raw_sample, label="EEUU")

    if config.DISCOVERY_US_REQUIRE_DOMESTIC_HQ:
        top = _exclude_foreign_hq(top, label="EEUU")

    return top


def _exclude_foreign_hq(candidates, label=""):
    """
    Saca del universo de EEUU las empresas cuya sede real (Ticker.info,
    campo "country") no es config.DISCOVERY_US_HQ_COUNTRY — el screener
    de Yahoo clasifica bajo la región 'us' cualquier instrumento que
    COTICE en el mercado de EEUU, no solo empresas estadounidenses: ADRs
    y certificados OTC de compañías extranjeras (TSMC, SK Hynix, Samsung,
    HSBC, Toyota, Nestlé, etc.) pasan el filtro de región igual que Intel
    o Apple. Confirmado con datos reales: de 93 candidatas "mega-cap" de
    una corrida real, 36 eran en realidad sedes fuera de EEUU (quedaron
    57 genuinamente estadounidenses, Intel incluida) — ver DISC-5 en
    mejoras_pendientes.txt.

    Los candidatos sin país conocido (`hq_country` es None, ej. porque
    Ticker.info falló las 2 veces) se CONSERVAN — mejor una candidata
    dudosa que descartar una empresa de EEUU real por una falla
    transitoria de red.
    """
    foreign = [c for c in candidates if c.get("hq_country") not in (None, config.DISCOVERY_US_HQ_COUNTRY)]
    if not foreign:
        return candidates

    logger.info(
        f"[{label}] excluidas {len(foreign)} empresas con sede fuera de EEUU (ADR/instrumento "
        "en USD de una empresa extranjera que el screener clasifica bajo la región 'us'; ver "
        f"DISC-5 en mejoras_pendientes.txt): "
        f"{[(c['symbol'], c.get('hq_country')) for c in foreign]}"
    )
    foreign_symbols = {c["symbol"] for c in foreign}
    return [c for c in candidates if c["symbol"] not in foreign_symbols]


def _to_ticker_dict(candidates):
    out = {}
    for c in candidates:
        currency = (
            c["currency"] if c["currency"] == "USD" or c["currency"] in config.CURRENCY_FX_TICKER else None
        )
        if currency is None:
            logger.warning(
                f"  excluido {c['symbol']} ({c['name']}): moneda '{c['currency']}' sin par FX configurado"
            )
            continue
        out[c["symbol"]] = (c["name"], c["country"], currency)
    return out


def _sector_map(candidates):
    return {c["symbol"]: c["sector"] for c in candidates}


def build_dynamic_universe():
    logger.info("Descubriendo universo asiático (China, Hong Kong, Japón, Corea, Taiwán, India)...")
    asia_candidates = discover_asia()
    logger.info(
        f"Universo asiático descubierto: {len(asia_candidates)} empresas "
        f"(objetivo {config.DISCOVERY_ASIA_TOP_N})"
    )

    logger.info("Descubriendo universo EEUU...")
    us_candidates = discover_us()
    if config.DISCOVERY_US_MEGACAP_MIN_MARKET_CAP is not None:
        logger.info(
            f"Universo EEUU descubierto: {len(us_candidates)} empresas "
            f"(todas las mega-cap, capitalización >= "
            f"${config.DISCOVERY_US_MEGACAP_MIN_MARKET_CAP:,.0f} — tamaño variable, "
            "no un top-N fijo)"
        )
    else:
        logger.info(
            f"Universo EEUU descubierto: {len(us_candidates)} empresas (objetivo {config.DISCOVERY_US_TOP_N})"
        )

    min_viable = 10  # si el screener devuelve muy pocas, se considera fallo parcial
    if len(asia_candidates) < min_viable or len(us_candidates) < min_viable:
        raise RuntimeError(
            f"Descubrimiento insuficiente (Asia={len(asia_candidates)}, EEUU={len(us_candidates)}); "
            "se usa el universo de respaldo."
        )

    asia_tickers = _to_ticker_dict(asia_candidates)
    us_tickers = _to_ticker_dict(us_candidates)
    sector_map = {**_sector_map(asia_candidates), **_sector_map(us_candidates)}

    return asia_tickers, us_tickers, sector_map


def save_snapshot(asia_tickers, us_tickers, sector_map, method):
    snapshot = {
        "discovered_at": datetime.now().isoformat(),
        "method": method,
        "asia_tickers": asia_tickers,
        "us_tickers": us_tickers,
        "sector_map": sector_map,
    }
    path = _today_snapshot_path()
    with open(path, "w", encoding="utf-8") as f:
        json.dump(snapshot, f, indent=2, ensure_ascii=False)
    logger.info(
        f"Snapshot de universo guardado: {path} "
        f"({len(asia_tickers)} Asia + {len(us_tickers)} EEUU, método={method})"
    )
    return path


def main(force=False):
    if not force and os.path.exists(_today_snapshot_path()):
        logger.info(
            f"Ya existe un snapshot de hoy ({_today_snapshot_path()}); se reutiliza. "
            "Usa --force para volver a descubrir."
        )
        return _today_snapshot_path()

    try:
        asia_tickers, us_tickers, sector_map = build_dynamic_universe()
        path = save_snapshot(asia_tickers, us_tickers, sector_map, method="dynamic_yfinance_screener")
    except Exception as e:
        logger.error(
            f"Descubrimiento dinámico falló ({e}). Se guarda un snapshot con el "
            "universo de respaldo (fallback_universe.py) para que el pipeline siga funcionando."
        )
        asia_tickers = {t: list(v) for t, v in config.FALLBACK_ASIA_TICKERS.items()}
        us_tickers = {t: list(v) for t, v in config.FALLBACK_US_TICKERS.items()}
        sector_map = dict(config.FALLBACK_SECTOR_MAP)
        path = save_snapshot(asia_tickers, us_tickers, sector_map, method="fallback_static")

    return path


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--force", action="store_true", help="Ignora el snapshot de hoy y vuelve a consultar el screener."
    )
    args = parser.parse_args()
    main(force=args.force)
