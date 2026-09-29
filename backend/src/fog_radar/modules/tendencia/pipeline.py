"""
Orquestador end-to-end del pipeline Stock_Tendencia.

Corre, en orden:
  0. universe.discovery    -> descubre dinámicamente las mayores empresas
                               de Asia y EEUU por capitalización de mercado
                               (o reutiliza el snapshot de hoy)
  1. ingestion.fetch_prices -> descarga precios semanales 5y del universo descubierto
  2. ingestion.fetch_fx     -> descarga FX para normalizar a USD
  3. ingestion.build_dataset -> arma dataset consolidado de retornos
  4. modeling.markowitz     -> selecciona las mejores acciones (TOP_N_SELECTION)
  5. modeling.trend_forecast -> proyecta tendencia a 6 meses
  6. modeling.momentum -> rankea el universo completo por momentum mensual y semanal
  7. reporting.pdf_report (opcional) -> informe PDF
  8. reporting.dashboard_data (opcional) -> exporta JSON para el frontend
                               (frontend/public/data/latest/, ver ese módulo)

Los pasos 4 a 8 escriben sus resultados dentro de una carpeta propia de la
corrida, output/<YYYYMMDD>_<NN> (ver tendencia.config.get_run_dir):
NN es un número de secuencia de dos dígitos, para que dos corridas del
mismo día queden en carpetas separadas en vez de pisarse los archivos.

Uso (vía run_pipeline.py en la raíz del proyecto):
    python run_pipeline.py
    python run_pipeline.py --skip-fetch         (reutiliza data/raw/ ya descargada)
    python run_pipeline.py --force-refresh      (ignora caché de precios, vuelve a descargar todo)
    python run_pipeline.py --force-discovery    (ignora el snapshot de hoy, vuelve a rankear el universo)
    python run_pipeline.py --static-universe    (no consulta el screener; usa la lista fija de respaldo)

IMPORTANTE: este pipeline necesita salida a internet hacia Yahoo Finance
(query1/query2.finance.yahoo.com) y, opcionalmente, FRED/federalreserve.gov
si corres fetch_macro_data.py. Debe ejecutarse en tu máquina local (o un
entorno con acceso libre a internet), no dentro de un sandbox con
allowlist de red restringido.
"""

import argparse
import importlib
import time

from fog_radar.modules.tendencia import config as _settings
from fog_radar.modules.tendencia.logging_utils import get_logger

logger = get_logger("pipeline", _settings.LOGS_DIR, log_filename="pipeline.log")


def build_arg_parser():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--skip-fetch",
        action="store_true",
        help="Omite los pasos 1 y 2 (descarga de precios/FX) y reutiliza data/raw/ existente.",
    )
    parser.add_argument(
        "--force-refresh",
        action="store_true",
        help="Ignora la caché de precios y vuelve a pedir todo a Yahoo Finance.",
    )
    parser.add_argument(
        "--force-discovery",
        action="store_true",
        help="Ignora el snapshot de universo de hoy y vuelve a consultar el screener.",
    )
    parser.add_argument(
        "--static-universe",
        action="store_true",
        help="No consulta el screener dinámico: usa directamente la lista fija de "
        "respaldo (fallback_universe.FALLBACK_ASIA_TICKERS / FALLBACK_US_TICKERS).",
    )
    return parser


def _run(args):
    t0 = time.time()

    from fog_radar.modules.tendencia import config
    from fog_radar.modules.tendencia.universe import discovery

    # Cada corrida completa del pipeline escribe en su propia carpeta
    # dentro de output/ (fecha + secuencia, ver settings.get_run_dir). Se
    # fuerza acá una carpeta NUEVA (new=True) para que dos corridas el
    # mismo día no se pisen los archivos; los pasos individuales, corridos
    # sueltos, reutilizan por defecto la más reciente de hoy.
    run_dir = config.get_run_dir(new=True)
    logger.info(f"Carpeta de esta corrida: {run_dir}")

    logger.info("=== PASO 0/8: Descubrimiento del universo (Asia + EEUU por capitalización) ===")

    if args.static_universe:
        logger.info("--static-universe activo: se usa la lista fija de respaldo, sin consultar el screener.")
        discovery.save_snapshot(
            {t: list(v) for t, v in config.FALLBACK_ASIA_TICKERS.items()},
            {t: list(v) for t, v in config.FALLBACK_US_TICKERS.items()},
            dict(config.FALLBACK_SECTOR_MAP),
            method="fallback_static_forced",
        )
    else:
        discovery.main(force=args.force_discovery)

    # El universo se cargó en settings.py al importarlo (arriba), ANTES de
    # que discovery escribiera el snapshot de hoy. Hay que recargar
    # settings para que ASIA_TICKERS/US_TICKERS/ALL_TICKERS/SECTOR_MAP
    # reflejen el snapshot recién generado — si no, todos los pasos
    # siguientes (que hacen "from fog_radar.modules.tendencia import config as
    # config" y ya lo encuentran cacheado en sys.modules) seguirían viendo
    # el universo viejo, porque Python solo ejecuta un módulo una vez por
    # proceso. `importlib.reload` muta el mismo objeto de módulo en
    # sys.modules, así que todos los "alias" a él (en cualquier submódulo
    # ya importado) ven los valores nuevos sin volver a importar nada.
    importlib.reload(config)
    logger.info(
        f"Universo activo: {len(config.ASIA_TICKERS)} Asia + {len(config.US_TICKERS)} EEUU "
        f"(fuente: {config.UNIVERSE_SOURCE})"
    )

    if not args.skip_fetch:
        logger.info("=== PASO 1/8: Descarga de precios (universo descubierto + referencia) ===")
        from fog_radar.modules.tendencia.ingestion import fetch_prices

        if args.force_refresh:
            from fog_radar.modules.tendencia.net.robust_yf import download_universe

            all_tickers = list(config.ALL_TICKERS.keys()) + list(config.REFERENCE_TICKERS.keys())
            download_universe(
                all_tickers, cache_dir=config.RAW_PRICES_DIR, force_refresh=True, label="[precios-refresh]"
            )
        else:
            fetch_prices.main()

        logger.info("=== PASO 2/8: Descarga de tipos de cambio (FX) ===")
        from fog_radar.modules.tendencia.ingestion import fetch_fx

        fetch_fx.main()
    else:
        logger.info(
            "--skip-fetch activo: se omiten pasos 1 y 2, se usa data/raw/ existente. "
            "Si el universo descubierto hoy incluye tickers nuevos sin precio ya "
            "descargado, van a quedar excluidos (ver data/processed/excluded_tickers.txt)."
        )

    logger.info("=== PASO 3/8: Construcción del dataset consolidado (USD, retornos) ===")
    from fog_radar.modules.tendencia.ingestion import build_dataset

    build_dataset.main()

    logger.info("=== PASO 4/8: Modelo de Markowitz / covarianza - selección de acciones ===")
    from fog_radar.modules.tendencia.modeling import markowitz

    markowitz.main()

    logger.info("=== PASO 5/8: Proyección de tendencia a 6 meses ===")
    from fog_radar.modules.tendencia.modeling import trend_forecast

    trend_forecast.main()

    logger.info("=== PASO 6/8: Momentum mensual y semanal (universo completo) ===")
    from fog_radar.modules.tendencia.modeling import momentum

    momentum.main()

    logger.info("=== PASO 7/8 (opcional): Informe PDF ===")
    try:
        from fog_radar.modules.tendencia.reporting import pdf_report

        pdf_report.main()
    except Exception as e:
        logger.warning(
            f"No se pudo generar el informe PDF ({e}). "
            "Puedes correrlo aparte con: python step06_generate_report.py"
        )

    logger.info("=== PASO 8/8 (opcional): Exportar datos del dashboard (frontend) ===")
    try:
        from fog_radar.modules.tendencia.reporting import dashboard_data

        dashboard_data.main()
    except Exception as e:
        logger.warning(
            f"No se pudo exportar el JSON del dashboard ({e}). "
            "Puedes correrlo aparte con: python step07_export_dashboard_data.py"
        )

    elapsed = time.time() - t0
    logger.info(f"Pipeline completo en {elapsed / 60:.1f} minutos.")
    logger.info(f"Resultados en: {run_dir}")
    logger.info(
        "  (selección, pesos y performance de Markowitz — incluidos los portafolios "
        "reducidos —, proyección a 6 meses, rankings de momentum, informe PDF y "
        "dashboard_data.json; ver backend/README.md para el detalle de cada archivo)"
    )
    return run_dir


def run(
    settings,
    *,
    skip_fetch=False,
    force_refresh=False,
    force_discovery=False,
    static_universe=False,
    macro=False,
    backfill=False,
):
    """Punto de entrada de Fog Radar (``python -m fog_radar.jobs tendencia``).

    Mismas opciones que ``run_pipeline.py`` del original, más ``--macro``
    (descarga los datos macro de FRED antes de correr) y ``--backfill`` (solo
    carga en ``history.db`` las corridas de ``output/`` que falten). Devuelve un
    resumen para el registro de jobs.
    """
    import json
    import os

    from fog_radar.modules.tendencia import config, paths

    paths.set_root(settings.module_data_dir("tendencia"))
    importlib.reload(config)

    problemas = config.validar_configuracion()
    if problemas:
        raise ValueError("Configuración de Tendencia no factible: " + " ".join(problemas))

    if backfill:
        from fog_radar.modules.tendencia import backfill as backfill_mod

        return backfill_mod.main()

    partes = []
    if macro:
        from fog_radar.modules.tendencia import macro as macro_mod

        key = settings.fred_api_key.get_secret_value() if settings.fred_api_key else None
        out = macro_mod.main(api_key=key)
        partes.append(f"macro {os.path.basename(out)}")

    args = argparse.Namespace(
        skip_fetch=skip_fetch,
        force_refresh=force_refresh,
        force_discovery=force_discovery,
        static_universe=static_universe,
    )
    run_dir = _run(args)

    dashboard = os.path.join(run_dir, "dashboard_data.json")
    if not os.path.exists(dashboard):
        raise RuntimeError(
            f"La corrida {os.path.basename(run_dir)} no generó dashboard_data.json (ver logs/pipeline.log)."
        )
    with open(dashboard, encoding="utf-8") as f:
        payload = json.load(f)
    partes.insert(0, f"corrida {payload['run_dir']}")
    for a in payload["analyses"]:
        perf = a["performance"]["portafolio_15_finales"]
        top = ", ".join(h["ticker"] for h in a["holdings"][:3])
        partes.append(f"{a['label']}: Sharpe {perf['sharpe_ratio']:.2f}, top {top}")
    return " · ".join(partes)


def main():
    args = build_arg_parser().parse_args()
    _run(args)


if __name__ == "__main__":
    main()
