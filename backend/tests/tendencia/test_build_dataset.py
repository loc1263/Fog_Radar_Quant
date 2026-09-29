from fog_radar.modules.tendencia.ingestion import build_dataset


def test_load_fx_rates_only_warns_for_currencies_in_use(monkeypatch, tmp_path, caplog):
    """
    Regresión: antes de esta refactorización, load_fx_rates() recorría
    TODAS las monedas configuradas en CURRENCY_FX_TICKER (6 monedas)
    incluso si el universo activo solo usaba, por ejemplo, KRW y HKD —
    generando warnings espurios sobre pares FX "faltantes" que ningún
    ticker de la corrida actual necesitaba.
    """
    from fog_radar.modules.tendencia import config

    monkeypatch.setattr(
        config,
        "ALL_TICKERS",
        {
            "005930.KS": ("Samsung", "Corea del Sur", "KRW"),
            "AAPL": ("Apple", "EEUU", "USD"),
        },
    )
    monkeypatch.setattr(config, "RAW_FX_DIR", str(tmp_path))

    import logging

    caplog.set_level(logging.WARNING, logger="step03")

    fx_rates = build_dataset.load_fx_rates()

    assert fx_rates == {}
    warned_currencies = [r.message for r in caplog.records if "Falta el par FX" in r.message]
    assert len(warned_currencies) == 1
    assert "KRW" in warned_currencies[0]
    assert "HKD" not in warned_currencies[0]
    assert "TWD" not in warned_currencies[0]
