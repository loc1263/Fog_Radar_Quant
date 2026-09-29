"""Lectura de archivos CSV."""

import pandas as pd

from fog_radar.core.logging import get_logger

log = get_logger("etf.screener")


class CSVReader:
    """Lee datos desde archivos CSV."""

    @staticmethod
    def read_tickers_from_csv(csv_file, ticker_column="Ticker"):
        """Lee tickers de ETFs desde un CSV."""
        try:
            df = pd.read_csv(csv_file, sep=";", engine="python")

            if ticker_column not in df.columns:
                log.info(f"El archivo {csv_file} no tiene columna '{ticker_column}'")
                return []

            tickers = df[ticker_column].dropna().unique().tolist()
            log.info(f"Importados {len(tickers)} tickers desde {csv_file}")
            return tickers

        except Exception as e:
            log.info(f"Error importando tickers desde {csv_file}: {e}")
            return []

    @staticmethod
    def read_etf_data_from_csv(csv_file, sep=","):
        """Lee datos de ETF desde un CSV."""
        try:
            df = pd.read_csv(csv_file, sep=sep, engine="python")
            return df
        except Exception as e:
            log.info(f"Error leyendo {csv_file}: {e}")
            return pd.DataFrame()

    @staticmethod
    def read_original_etf_list(csv_file):
        """Lee la lista original de ETFs."""
        return CSVReader.read_etf_data_from_csv(csv_file, sep=";")

    @staticmethod
    def read_ticker_metadata(
        csv_file,
        ticker_column="Ticker",
        name_column="Name",
        company_column="Company",
        category_column="Category",
    ):
        """Lee Ticker -> {"Name": ..., "Company": ..., "Category": ...} desde
        el CSV de entrada (List_All_ETF.csv). Se usa como respaldo local en
        discovery.py cuando Yahoo no devuelve longName/shortName, categoria
        o un proveedor confiable -- List_All_ETF.csv trae Name/Company/
        Category curados a mano por el usuario y, a diferencia de Yahoo,
        sin huecos (verificado 2026-07-17: 0 nulos en Name/Company sobre
        1692 tickers; esquema con Company/Category vigente desde
        2026-07-23, reemplaza a la columna "Asset Managers").

        Devuelve {} (sin lanzar) si el archivo no existe o no tiene las
        columnas Ticker/Name/Company (Category es opcional: si falta, esa
        clave queda ausente del dict de metadata pero no invalida el resto)
        -- es un dato de respaldo, no debe hacer fallar la lectura de
        tickers si falta o cambia de formato."""
        try:
            df = pd.read_csv(csv_file, sep=";", engine="python")
        except Exception as e:
            log.info(f"Aviso: no se pudo leer metadata local desde {csv_file}: {e}")
            return {}

        columnas_necesarias = {ticker_column, name_column, company_column}
        if not columnas_necesarias.issubset(df.columns):
            faltan = columnas_necesarias - set(df.columns)
            log.info(
                f"Aviso: {csv_file} no tiene columna(s) {faltan}; no habra "
                "respaldo local de Nombre/Proveedor para esta corrida."
            )
            return {}

        tiene_category = category_column in df.columns

        metadata = {}
        for _, fila in df.iterrows():
            ticker = fila[ticker_column]
            if pd.isna(ticker):
                continue
            metadata[ticker] = {
                "Name": fila[name_column] if pd.notna(fila[name_column]) else None,
                "Company": fila[company_column] if pd.notna(fila[company_column]) else None,
                "Category": (
                    fila[category_column] if tiene_category and pd.notna(fila[category_column]) else None
                ),
            }
        return metadata
