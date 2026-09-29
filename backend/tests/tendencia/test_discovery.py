from fog_radar.modules.tendencia.universe import discovery


def test_normalize_quote_extracts_expected_fields():
    quote = {
        "symbol": "TEST",
        "longName": "Test Company Inc",
        "marketCap": {"raw": 123456789.0, "fmt": "123.5M"},
        "currency": "USD",
        "sector": "Technology",
        "quoteType": "EQUITY",
    }
    norm = discovery._normalize_quote(quote, "EEUU")
    assert norm == {
        "symbol": "TEST",
        "name": "Test Company Inc",
        "country": "EEUU",
        "currency": "USD",
        "market_cap": 123456789.0,
        "sector": "Technology",
    }


def test_quote_sector_prefers_sector_disp_over_sector():
    quote = {"sectorDisp": "Technology", "sector": "technology-raw-code"}
    assert discovery._quote_sector(quote) == "Technology"


def test_quote_sector_falls_back_through_industry_fields():
    assert discovery._quote_sector({"industryDisp": "Semiconductors"}) == "Semiconductors"
    assert discovery._quote_sector({"industry": "Semiconductors"}) == "Semiconductors"
    assert discovery._quote_sector({}) == "Otro"
    # campo presente pero vacío/None no debe "ganar" sobre un candidato
    # posterior que sí tiene valor
    assert discovery._quote_sector({"sectorDisp": "", "sector": "Technology"}) == "Technology"
    assert discovery._quote_sector({"sectorDisp": None, "industry": "Software"}) == "Software"


def test_quote_sector_unwraps_raw_fmt_dict():
    quote = {"sectorDisp": {"raw": "Technology", "fmt": "Technology"}}
    assert discovery._quote_sector(quote) == "Technology"


def test_enrich_missing_sectors_fills_otro_via_ticker_info(monkeypatch):
    """
    El screener no trae sector (ver docstring de _enrich_missing_sectors);
    esta prueba simula que Ticker.info sí lo tiene y verifica que
    _enrich_missing_sectors lo usa para reemplazar "Otro".
    """
    import sys
    import types

    class FakeTicker:
        def __init__(self, symbol):
            self.symbol = symbol

        def get_info(self):
            info_by_symbol = {
                "AAA": {"sector": "Technology"},
                "BBB": {"industry": "Semiconductors"},  # sin 'sector', cae a industry
                "CCC": {},  # sin nada -> se queda en "Otro"
            }
            return info_by_symbol.get(self.symbol, {})

    fake_yf = types.SimpleNamespace(Ticker=FakeTicker)
    monkeypatch.setitem(sys.modules, "yfinance", fake_yf)
    monkeypatch.setattr(discovery.time, "sleep", lambda *a, **k: None)

    candidates = [
        {"symbol": "AAA", "sector": "Otro"},
        {"symbol": "BBB", "sector": "Otro"},
        {"symbol": "CCC", "sector": "Otro"},
        {"symbol": "DDD", "sector": "Ya tenía sector"},  # no debe tocarse
    ]
    discovery._enrich_missing_sectors(candidates, label="Test")

    by_symbol = {c["symbol"]: c["sector"] for c in candidates}
    assert by_symbol == {
        "AAA": "Technology",
        "BBB": "Semiconductors",
        "CCC": "Otro",
        "DDD": "Ya tenía sector",
    }


def test_enrich_missing_sectors_captures_hq_country_when_requested(monkeypatch):
    """
    El screener de Yahoo clasifica bajo la región 'us' cualquier
    instrumento que cotice ahí, no solo empresas con sede en EEUU (ver
    DISC-5). Con capture_hq_country=True, _enrich_missing_sectors debe
    guardar Ticker.info()["country"] en c["hq_country"] para que
    discover_us() pueda filtrar por sede real.
    """
    import sys
    import types

    class FakeTicker:
        def __init__(self, symbol):
            self.symbol = symbol

        def get_info(self):
            info_by_symbol = {
                "INTC": {"sector": "Technology", "country": "United States"},
                "TSM": {"sector": "Technology", "country": "Taiwan"},
            }
            return info_by_symbol.get(self.symbol, {})

    fake_yf = types.SimpleNamespace(Ticker=FakeTicker)
    monkeypatch.setitem(sys.modules, "yfinance", fake_yf)
    monkeypatch.setattr(discovery.time, "sleep", lambda *a, **k: None)

    candidates = [
        {"symbol": "INTC", "sector": "Otro"},
        {"symbol": "TSM", "sector": "Otro"},
    ]
    discovery._enrich_missing_sectors(candidates, label="Test", capture_hq_country=True)

    by_symbol = {c["symbol"]: c["hq_country"] for c in candidates}
    assert by_symbol == {"INTC": "United States", "TSM": "Taiwan"}


def test_exclude_foreign_hq_removes_non_domestic_and_keeps_unknown(monkeypatch):
    from fog_radar.modules.tendencia import config

    monkeypatch.setattr(config, "DISCOVERY_US_HQ_COUNTRY", "United States")

    candidates = [
        {"symbol": "INTC", "hq_country": "United States"},
        {"symbol": "TSM", "hq_country": "Taiwan"},
        {"symbol": "SKHY", "hq_country": "South Korea"},
        {"symbol": "MYSTERY", "hq_country": None},  # Ticker.info falló: se conserva
    ]

    result = discovery._exclude_foreign_hq(candidates, label="Test")

    assert {c["symbol"] for c in result} == {"INTC", "MYSTERY"}


def test_exclude_foreign_hq_noop_when_all_domestic_or_unknown():
    candidates = [
        {"symbol": "INTC", "hq_country": "United States"},
        {"symbol": "MYSTERY", "hq_country": None},
    ]

    result = discovery._exclude_foreign_hq(candidates, label="Test")

    assert result is candidates  # sin exclusiones, devuelve la misma lista (no una copia)


def test_enrich_missing_sectors_noop_when_nothing_pending(monkeypatch):
    # No debe importar yfinance ni intentar red si no hay nada en "Otro";
    # si lo intentara, fallaría porque no hay yfinance mockeado en sys.modules
    # (o pegaría a la red real, que no está disponible en este entorno de test).
    candidates = [{"symbol": "AAA", "sector": "Technology"}]
    discovery._enrich_missing_sectors(candidates, label="Test")
    assert candidates == [{"symbol": "AAA", "sector": "Technology"}]


def test_warn_if_sector_still_missing_writes_debug_sample_when_enrichment_failed(
    tmp_path, monkeypatch, caplog
):
    import logging

    from fog_radar.modules.tendencia import config

    monkeypatch.setattr(config, "UNIVERSE_DIR", str(tmp_path))
    caplog.set_level(logging.WARNING, logger="step00")

    # Más del 30% siguió en "Otro" INCLUSO después de _enrich_missing_sectors
    # (a diferencia del screener, que siempre llega en "Otro" y no amerita aviso).
    candidates = [
        {"symbol": "A", "sector": "Otro"},
        {"symbol": "B", "sector": "Otro"},
        {"symbol": "C", "sector": "Tech"},
    ]
    raw_sample = {"symbol": "A", "someUnexpectedField": "value"}
    discovery._warn_if_sector_still_missing(candidates, raw_sample, label="EEUU")

    debug_file = tmp_path / "_debug_raw_tickerinfo_eeuu.json"
    assert debug_file.exists()
    assert "someUnexpectedField" in debug_file.read_text(encoding="utf-8")
    assert any("sector" in r.message.lower() for r in caplog.records)


def test_warn_if_sector_still_missing_stays_quiet_below_threshold(tmp_path, monkeypatch, caplog):
    import logging

    from fog_radar.modules.tendencia import config

    monkeypatch.setattr(config, "UNIVERSE_DIR", str(tmp_path))
    caplog.set_level(logging.WARNING, logger="step00")

    # Solo 1/5 (20%) siguió en "Otro" tras el enriquecimiento -> rate-limit
    # puntual esperable, no amerita WARNING ni volcado de diagnóstico.
    candidates = [
        {"symbol": "A", "sector": "Otro"},
        {"symbol": "B", "sector": "Tech"},
        {"symbol": "C", "sector": "Tech"},
        {"symbol": "D", "sector": "Tech"},
        {"symbol": "E", "sector": "Tech"},
    ]
    discovery._warn_if_sector_still_missing(candidates, {"symbol": "A"}, label="EEUU")

    assert not (tmp_path / "_debug_raw_tickerinfo_eeuu.json").exists()
    assert not any("sector" in r.message.lower() for r in caplog.records if r.levelno >= logging.WARNING)


def test_warn_if_sector_still_missing_noop_when_all_resolved(tmp_path, monkeypatch):
    from fog_radar.modules.tendencia import config

    monkeypatch.setattr(config, "UNIVERSE_DIR", str(tmp_path))

    candidates = [
        {"symbol": "A", "sector": "Tech"},
        {"symbol": "B", "sector": "Finance"},
    ]
    discovery._warn_if_sector_still_missing(candidates, {"symbol": "A"}, label="EEUU")
    assert not (tmp_path / "_debug_raw_tickerinfo_eeuu.json").exists()


def test_normalize_quote_rejects_non_equity():
    quote = {"symbol": "BTC-USD", "marketCap": 1e9, "currency": "USD", "quoteType": "CRYPTOCURRENCY"}
    assert discovery._normalize_quote(quote, "EEUU") is None


def test_normalize_quote_rejects_missing_market_cap():
    quote = {"symbol": "TEST", "currency": "USD", "quoteType": "EQUITY"}
    assert discovery._normalize_quote(quote, "EEUU") is None


def test_dedupe_by_company_keeps_highest_market_cap():
    candidates = [
        {
            "symbol": "0001.HK",
            "name": "Big Bank Holdings Ltd",
            "market_cap": 100.0,
            "country": "China",
            "currency": "HKD",
            "sector": "Financial",
        },
        {
            "symbol": "601001.SS",
            "name": "Big Bank Corp",
            "market_cap": 150.0,
            "country": "China",
            "currency": "CNY",
            "sector": "Financial",
        },
        {
            "symbol": "OTHER",
            "name": "Totally Different Co",
            "market_cap": 50.0,
            "country": "China",
            "currency": "CNY",
            "sector": "Energy",
        },
    ]
    result = discovery._dedupe_by_company(candidates)
    symbols = {c["symbol"] for c in result}
    # Las dos primeras normalizan al mismo nombre ("BIG BANK") -> se queda
    # solo con la de mayor market cap (601001.SS); OTHER es distinta y se
    # conserva.
    assert symbols == {"601001.SS", "OTHER"}


def _candidate(symbol, market_cap):
    return {
        "symbol": symbol,
        "name": symbol,
        "market_cap": market_cap,
        "country": "EEUU",
        "currency": "USD",
        "sector": "Technology",
    }


def test_select_candidates_top_n_mode_takes_exactly_n_by_rank():
    candidates = [_candidate("A", 300), _candidate("B", 100), _candidate("C", 200)]

    result = discovery._select_candidates(candidates, top_n=2)

    assert [c["symbol"] for c in result] == ["A", "C"]


def test_select_candidates_megacap_mode_ignores_top_n_and_uses_threshold():
    candidates = [
        _candidate("HUGE", 3_000_000_000_000),
        _candidate("INTC_LIKE", 643_000_000_000),
        _candidate("SMALL", 50_000_000_000),
    ]

    # top_n=1 no debería limitar nada en modo mega-cap: entran todas las
    # que superen el piso, sin importar cuántas sean.
    result = discovery._select_candidates(candidates, top_n=1, min_market_cap=200_000_000_000)

    assert [c["symbol"] for c in result] == ["HUGE", "INTC_LIKE"]


def test_select_candidates_megacap_mode_can_return_more_than_default_top_n():
    candidates = [_candidate(f"MEGA{i}", 500_000_000_000) for i in range(30)]

    result = discovery._select_candidates(candidates, top_n=21, min_market_cap=200_000_000_000)

    assert len(result) == 30  # las 30 superan el piso; el tamaño no queda fijo en 21


def test_to_ticker_dict_excludes_unconfigured_currency(monkeypatch):
    from fog_radar.modules.tendencia import config

    monkeypatch.setattr(config, "CURRENCY_FX_TICKER", {"JPY": "USDJPY=X"})

    candidates = [
        {
            "symbol": "OK.T",
            "name": "Ok Co",
            "market_cap": 10.0,
            "country": "Japón",
            "currency": "JPY",
            "sector": "Tech",
        },
        {
            "symbol": "NOPE.XX",
            "name": "Nope Co",
            "market_cap": 10.0,
            "country": "Nowhere",
            "currency": "ZZZ",
            "sector": "Tech",
        },
    ]
    out = discovery._to_ticker_dict(candidates)
    assert "OK.T" in out
    assert "NOPE.XX" not in out
