"""
Descarga los pares de tipo de cambio (FX) semanales necesarios para
convertir a USD los tickers asiáticos que cotizan en moneda local
(KRW, HKD, CNY, JPY, INR, TWD).

Ejecutar (vía wrapper de compatibilidad en la raíz del proyecto):
    python step02_fetch_fx.py

Genera:
    data/raw/fx/<PAR>.csv
"""

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.logging_utils import get_logger
from fog_radar.modules.tendencia.net.robust_yf import download_universe

logger = get_logger("step02", config.LOGS_DIR, log_filename="step02_fetch_fx.log")


def main():
    currencies_in_use = {currency for _, (_, _, currency) in config.ASIA_TICKERS.items() if currency != "USD"}
    fx_tickers = [config.CURRENCY_FX_TICKER[c] for c in currencies_in_use]
    logger.info(f"Monedas a convertir: {sorted(currencies_in_use)}")
    logger.info(f"Pares FX a descargar: {fx_tickers}")

    results, failed = download_universe(fx_tickers, cache_dir=config.RAW_FX_DIR, label="[fx]")

    logger.info(f"Descarga FX completa: {len(results)}/{len(fx_tickers)} exitosos")
    if failed:
        logger.warning(
            f"Pares FX que fallaron: {failed}. Sin estos pares, los tickers en esa "
            "moneda no podrán normalizarse a USD y se excluirán del dataset."
        )

    return results, failed


if __name__ == "__main__":
    main()
