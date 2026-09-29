"""
Datos macro de referencia (port de ``Stock_Tendencia/backend/fetch_macro_data.py``):
curva de tasas, spreads de crédito, VIX, proxies de PMI/ISM y postura de la Fed.

    uv run python -m fog_radar.jobs tendencia --macro     # macro + pipeline completo

La API key de FRED sale de ``FRED_API_KEY`` en ``backend/.env`` (``Settings.fred_api_key``).
El PMI/ISM oficial no tiene fuente gratuita: se usa el Chicago Fed National Activity
Index (CFNAI) y, en modo best-effort, encuestas regionales de la Fed.

Salida (en ``data/tendencia/macro/``):
    macro_data_<YYYYMMDD>.csv   -> último valor de cada serie
    fomc_latest_statement.txt   -> comunicado FOMC más reciente (best-effort)

Cambios respecto del original (solo integración): rutas desde ``config.MACRO_DIR``,
key desde ``Settings`` y errores con excepción en vez de ``sys.exit``.
"""

import os
import re
from datetime import datetime

import pandas as pd
import requests
from bs4 import BeautifulSoup

from fog_radar.modules.tendencia import config


def _output_dir():
    # Misma carpeta que lee config.latest_macro_file().
    return config.MACRO_DIR


# --- Series de FRED confirmadas y gratuitas ---
FRED_SERIES = {
    # Curva de tasas
    "Treasury 3M (%)": "DGS3MO",
    "Treasury 2Y (%)": "DGS2",
    "Treasury 10Y (%)": "DGS10",
    "Treasury 30Y (%)": "DGS30",
    "Spread 10Y-2Y (pp)": "T10Y2Y",
    "Spread 10Y-3M (pp)": "T10Y3M",
    # Spreads de crédito (ICE BofA option-adjusted spread)
    "Credit Spread High Yield (pp)": "BAMLH0A0HYM2",
    "Credit Spread Investment Grade (pp)": "BAMLC0A0CM",
    "Credit Spread BBB (pp)": "BAMLC0A4CBBB",
    # VIX (también disponible vía yfinance, se deja como respaldo)
    "VIX (FRED)": "VIXCLS",
    # Postura de la Fed
    "Fed Funds Rate efectiva (%)": "FEDFUNDS",
    "Fed Funds Target Upper (%)": "DFEDTARU",
    "Fed Funds Target Lower (%)": "DFEDTARL",
    "Fed Dot Plot mediana (%)": "FEDTARMD",
    # Proxy de actividad manufacturera/PMI (gratuito, no es el ISM oficial)
    "Chicago Fed National Activity Index (CFNAI)": "CFNAI",
}

# Proxies regionales adicionales de PMI: best-effort, algunos IDs pueden no existir
# o haber cambiado. Si fallan, el script los omite y sigue sin romperse.
FRED_SERIES_BEST_EFFORT = {
    "Philly Fed Manufacturing Index": "GACDISA066MSFRBPHI",
    "NY Fed Empire State Manufacturing Index": "GACDFSA066MSFRBNY",
    "Richmond Fed Manufacturing Index": "RSAFSMFTIX",
    "Dallas Fed Manufacturing Index": "BACTSAMFRBDAL",
    "Kansas City Fed Manufacturing Index": "MMSMFGCOMP",
}


def get_fred_client(api_key=None):
    from fredapi import Fred

    api_key = api_key or os.environ.get("FRED_API_KEY")
    if not api_key:
        raise RuntimeError(
            "No se encontró la variable de entorno FRED_API_KEY.\n"
            "Consigue una key gratis en https://fred.stlouisfed.org/docs/api/api_key.html\n"
            "y expórtala antes de correr el script (ver docstring del archivo)."
        )
    return Fred(api_key=api_key)


def fetch_fred_series(fred, series_dict, required=True):
    rows = []
    for label, series_id in series_dict.items():
        try:
            s = fred.get_series(series_id).dropna()
            if s.empty:
                raise ValueError("serie vacía")
            rows.append(
                {
                    "Indicador": label,
                    "Series ID": series_id,
                    "Último valor": s.iloc[-1],
                    "Fecha dato": s.index[-1].strftime("%Y-%m-%d"),
                }
            )
        except Exception as e:
            status = "ERROR" if required else "omitido (best-effort)"
            print(f"  [{status}] {label} ({series_id}): {e}")
    return rows


def fetch_vix_yfinance():
    try:
        import yfinance as yf

        hist = yf.Ticker("^VIX").history(period="5d")
        if hist.empty:
            raise ValueError("sin datos")
        last = hist["Close"].iloc[-1]
        last_date = hist.index[-1].strftime("%Y-%m-%d")
        return {
            "Indicador": "VIX (Yahoo Finance, spot)",
            "Series ID": "^VIX",
            "Último valor": last,
            "Fecha dato": last_date,
        }
    except Exception as e:
        print(f"  [ERROR] VIX vía yfinance: {e}")
        return None


def fetch_latest_fomc_statement():
    """Best-effort: busca el comunicado FOMC más reciente en federalreserve.gov."""
    year = datetime.now().year
    index_url = f"https://www.federalreserve.gov/newsevents/pressreleases/{year}-press-fomc.htm"
    try:
        resp = requests.get(index_url, timeout=15, headers={"User-Agent": "Mozilla/5.0"})
        resp.raise_for_status()
        soup = BeautifulSoup(resp.text, "html.parser")

        # Los comunicados de decisión de tasa terminan en ...YYYYMMDDa.htm
        # (el sufijo "b" suele ser la nota de implementación técnica)
        candidates = sorted(
            {
                a["href"]
                for a in soup.find_all("a", href=True)
                if re.search(r"/newsevents/pressreleases/monetary\d{8}a\.htm$", a["href"])
            },
            reverse=True,
        )
        if not candidates:
            raise ValueError(f"no se encontró un enlace de comunicado monetario en {index_url}")

        full_url = "https://www.federalreserve.gov" + candidates[0]
        stmt_resp = requests.get(full_url, timeout=15, headers={"User-Agent": "Mozilla/5.0"})
        stmt_resp.raise_for_status()
        stmt_resp.encoding = "utf-8"
        stmt_soup = BeautifulSoup(stmt_resp.text, "html.parser")
        article = stmt_soup.find("div", {"id": "article"}) or stmt_soup.find(
            "div", class_="col-xs-12 col-sm-8 col-md-8"
        )
        text = article.get_text("\n", strip=True) if article else stmt_soup.get_text("\n", strip=True)

        out_path = os.path.join(_output_dir(), "fomc_latest_statement.txt")
        with open(out_path, "w", encoding="utf-8") as f:
            f.write(f"Fuente: {full_url}\nDescargado: {datetime.now().isoformat()}\n\n{text}")
        print(f"  Comunicado FOMC guardado en: {out_path}")
        return full_url
    except Exception as e:
        print(f"  [ERROR] No se pudo obtener el comunicado FOMC: {e}")
        return None


def main(api_key=None):
    os.makedirs(_output_dir(), exist_ok=True)
    print("=== Descargando datos macro ===\n")

    print("[FRED] series principales (curva, spreads, VIX, Fed, CFNAI)...")
    fred = get_fred_client(api_key)
    rows = fetch_fred_series(fred, FRED_SERIES, required=True)

    print("\n[FRED] proxies regionales de PMI (best-effort)...")
    rows += fetch_fred_series(fred, FRED_SERIES_BEST_EFFORT, required=False)

    print("\n[Yahoo Finance] VIX spot...")
    vix_row = fetch_vix_yfinance()
    if vix_row:
        rows.append(vix_row)

    print("\n[federalreserve.gov] comunicado FOMC más reciente...")
    fetch_latest_fomc_statement()

    if not rows:
        raise RuntimeError("No se pudo obtener ningún dato macro. Revisa FRED_API_KEY y la conexión.")

    df = pd.DataFrame(rows)
    out_csv = os.path.join(_output_dir(), f"macro_data_{datetime.now().strftime('%Y%m%d')}.csv")
    df.to_csv(out_csv, index=False)

    print(f"\n=== Resumen ({len(df)} series obtenidas) ===")
    print(df.to_string(index=False))
    print(f"\nGuardado en: {out_csv}")
    return out_csv
