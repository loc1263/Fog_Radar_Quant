"""
Descarga el histórico semanal (5 años) de todos los tickers del universo
(Asia + EEUU) más los tickers de referencia (^VIX, ^IRX).

Ejecutar (vía wrapper de compatibilidad en la raíz del proyecto):
    python step01_fetch_prices.py

Genera:
    data/raw/prices/<TICKER>.csv   (uno por ticker)
"""

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.logging_utils import get_logger
from fog_radar.modules.tendencia.net.robust_yf import download_universe

logger = get_logger("step01", config.LOGS_DIR, log_filename="step01_fetch_prices.log")


def main():
    all_tickers = list(config.ALL_TICKERS.keys()) + list(config.REFERENCE_TICKERS.keys())
    logger.info(f"Universo total a descargar: {len(all_tickers)} tickers")
    logger.info(
        f"  Asia: {len(config.ASIA_TICKERS)} | EEUU: {len(config.US_TICKERS)} | "
        f"Referencia: {len(config.REFERENCE_TICKERS)}"
    )

    results, failed = download_universe(all_tickers, cache_dir=config.RAW_PRICES_DIR, label="[precios]")

    logger.info(f"Descarga completa: {len(results)}/{len(all_tickers)} exitosos")
    if failed:
        logger.warning(
            f"Tickers que fallaron tras {config.MAX_RETRIES} reintentos: {failed}\n"
            "Puedes volver a correr este script: los tickers ya descargados se "
            "leen de caché y no se vuelven a pedir."
        )

    return results, failed


if __name__ == "__main__":
    main()
