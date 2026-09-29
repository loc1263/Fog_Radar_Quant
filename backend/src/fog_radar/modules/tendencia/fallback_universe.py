"""
Universo de tickers — listas de RESPALDO (fallback estático, agosto 2026).

Formato: ticker -> (nombre, país, moneda_local). Moneda "USD" = ya cotiza
en dólares (ADR o empresa listada en EEUU), no requiere conversión de FX.

Estas listas (verificadas vía búsqueda web, agosto 2026: rankings de
StockTitan para EEUU y Hantec Markets/FinanceCharts para Asia) solo se
usan si el descubrimiento dinámico (tendencia.universe.discovery)
falla, o si corres el pipeline con --static-universe. Se excluyó Saudi
Aramco (Medio Oriente) y SpaceX (no cotiza en bolsa) también en el
descubrimiento dinámico, porque no se consulta la región 'sa' y las
empresas no públicas no aparecen en el screener.

Separado de settings.py para que la configuración "viva" (rutas,
parámetros del modelo) no quede mezclada en el mismo archivo que estos
datos de referencia estáticos, que rara vez cambian y son largos.
"""

FALLBACK_ASIA_TICKERS = {
    "TSM": ("Taiwan Semiconductor (TSMC)", "Taiwán", "USD"),
    "005930.KS": ("Samsung Electronics", "Corea del Sur", "KRW"),
    "TCEHY": ("Tencent Holdings (ADR)", "China", "USD"),
    "000660.KS": ("SK Hynix", "Corea del Sur", "KRW"),
    "1398.HK": ("ICBC", "China", "HKD"),
    "BABA": ("Alibaba Group (ADR)", "China", "USD"),
    "601288.SS": ("Agricultural Bank of China", "China", "CNY"),
    "0939.HK": ("China Construction Bank", "China", "HKD"),
    "TM": ("Toyota Motor (ADR)", "Japón", "USD"),
    "600519.SS": ("Kweichow Moutai", "China", "CNY"),
    "0857.HK": ("PetroChina", "China", "HKD"),
    "0941.HK": ("China Mobile", "China", "HKD"),
    "TCS.NS": ("Tata Consultancy Services", "India", "INR"),
    "HDB": ("HDFC Bank (ADR)", "India", "USD"),
    "3988.HK": ("Bank of China", "China", "HKD"),
    "SONY": ("Sony Group (ADR)", "Japón", "USD"),
    "MUFG": ("Mitsubishi UFJ Financial Group (ADR)", "Japón", "USD"),
    "6861.T": ("Keyence", "Japón", "JPY"),
    "RELIANCE.NS": ("Reliance Industries", "India", "INR"),
    "PDD": ("PDD Holdings (ADR)", "China", "USD"),
}

FALLBACK_US_TICKERS = {
    "NVDA": ("Nvidia", "EEUU", "USD"),
    "AAPL": ("Apple", "EEUU", "USD"),
    "GOOGL": ("Alphabet", "EEUU", "USD"),
    "MSFT": ("Microsoft", "EEUU", "USD"),
    "AMZN": ("Amazon", "EEUU", "USD"),
    "AVGO": ("Broadcom", "EEUU", "USD"),
    "META": ("Meta Platforms", "EEUU", "USD"),
    "TSLA": ("Tesla", "EEUU", "USD"),
    "BRK-B": ("Berkshire Hathaway", "EEUU", "USD"),
    "LLY": ("Eli Lilly", "EEUU", "USD"),
    "MU": ("Micron Technology", "EEUU", "USD"),
    "JPM": ("JPMorgan Chase", "EEUU", "USD"),
    "WMT": ("Walmart", "EEUU", "USD"),
    "AMD": ("Advanced Micro Devices", "EEUU", "USD"),
    "V": ("Visa", "EEUU", "USD"),
    "XOM": ("ExxonMobil", "EEUU", "USD"),
    "JNJ": ("Johnson & Johnson", "EEUU", "USD"),
    "INTC": ("Intel", "EEUU", "USD"),
    "MA": ("Mastercard", "EEUU", "USD"),
    "CSCO": ("Cisco Systems", "EEUU", "USD"),
    "BAC": ("Bank of America", "EEUU", "USD"),
}

FALLBACK_SECTOR_MAP = {
    # Asia
    "TSM": "Semiconductores",
    "005930.KS": "Semiconductores",
    "TCEHY": "Tecnología / Internet",
    "000660.KS": "Semiconductores",
    "1398.HK": "Financiero / Banca",
    "BABA": "Consumo discrecional / E-commerce",
    "601288.SS": "Financiero / Banca",
    "0939.HK": "Financiero / Banca",
    "TM": "Consumo discrecional / Automotriz",
    "600519.SS": "Consumo básico / Bebidas",
    "0857.HK": "Energía",
    "0941.HK": "Telecomunicaciones",
    "TCS.NS": "Tecnología / Servicios TI",
    "HDB": "Financiero / Banca",
    "3988.HK": "Financiero / Banca",
    "SONY": "Tecnología / Electrónica y entretenimiento",
    "MUFG": "Financiero / Banca",
    "6861.T": "Industrial / Automatización",
    "RELIANCE.NS": "Energía / Conglomerado",
    "PDD": "Consumo discrecional / E-commerce",
    # EEUU
    "NVDA": "Semiconductores",
    "AAPL": "Tecnología / Hardware de consumo",
    "GOOGL": "Tecnología / Internet",
    "MSFT": "Tecnología / Software",
    "AMZN": "Consumo discrecional / E-commerce",
    "AVGO": "Semiconductores",
    "META": "Tecnología / Internet",
    "TSLA": "Consumo discrecional / Automotriz",
    "BRK-B": "Financiero / Conglomerado y seguros",
    "LLY": "Salud / Farmacéutica",
    "MU": "Semiconductores",
    "JPM": "Financiero / Banca",
    "WMT": "Consumo básico / Retail",
    "AMD": "Semiconductores",
    "V": "Financiero / Medios de pago",
    "XOM": "Energía",
    "JNJ": "Salud / Farmacéutica",
    "INTC": "Semiconductores",
    "MA": "Financiero / Medios de pago",
    "CSCO": "Tecnología / Hardware de redes",
    "BAC": "Financiero / Banca",
}
