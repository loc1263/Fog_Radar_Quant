"""Escritura de archivos CSV y Excel."""

import os
from datetime import datetime

import pandas as pd

from ..config import DEFAULT_NUM_ETFS, INPUT_CSV, PERFORMANCE_COLUMNS_FOR_SELECTION
from ..file_handlers.reader import CSVReader

# Fog Radar: la carpeta de salida (output_dir) la pasa el pipeline en cada
# llamada en vez de ser una constante global de config.py.


class CSVWriter:
    """Escribe datos en archivos CSV y Excel."""

    @staticmethod
    def save_performance_data(df, timestamp, output_dir):
        """Guarda datos de performance en CSV y Excel."""
        result = {"csv": None, "xlsx": None, "choice_csv": None, "choice_xlsx": None, "found_count": 0}

        # Agregar columna 'fecha' con la fecha de ejecución
        fecha_hoy = datetime.now().strftime("%Y-%m-%d")
        df = df.copy()
        df["Fecha"] = fecha_hoy
        csv_file = os.path.join(output_dir, f"etf_perf_data_{timestamp}.csv")
        df.to_csv(csv_file, index=False, encoding="utf-8-sig")
        result["csv"] = csv_file

        excel_file = os.path.join(output_dir, f"etf_perf_data_{timestamp}.xlsx")
        try:
            df.to_excel(excel_file, index=False)
            result["xlsx"] = excel_file
        except Exception:
            result["xlsx"] = None

        choice_result = CSVWriter.save_top_etfs_by_growth(csv_file, timestamp, output_dir)
        result["choice_csv"] = choice_result.get("csv")
        result["choice_xlsx"] = choice_result.get("xlsx")
        result["found_count"] = choice_result.get("found_count")

        return result

    @staticmethod
    def save_top_etfs_by_growth(csv_filename, timestamp, output_dir, num_etfs=DEFAULT_NUM_ETFS):
        """Selecciona los mejores ETFs usando la lógica de ETFChoiceOnly.py (solo TipoInstrumento=='ETF', performance creciente y no negativa)."""
        result = {"csv": None, "xlsx": None, "found_count": 0}
        try:
            df = pd.read_csv(csv_filename, sep=",", engine="python")
        except Exception:
            return result

        # Filtrar solo ETFs
        if "TipoInstrumento" not in df.columns:
            return result
        df_etf = df[df["TipoInstrumento"] == "ETF"].copy()

        # Definir los periodos a considerar (usar PERFORMANCE_COLUMNS_FOR_SELECTION)
        periodos = [col for col in PERFORMANCE_COLUMNS_FOR_SELECTION if col in df_etf.columns]
        if len(periodos) < 2:
            return result

        # Filtrar ETFs con rendimientos crecientes y no negativos
        def es_creciente_no_negativo(row):
            try:
                valores = [float(row[p]) for p in periodos]
            except Exception:
                return False
            return all(v >= 0 for v in valores) and all(
                valores[i] <= valores[i + 1] for i in range(len(valores) - 1)
            )

        df_filtrado = df_etf[df_etf.apply(es_creciente_no_negativo, axis=1)]

        # Seleccionar los mejores por el último periodo (por defecto, mayor Performance 5Y)
        sort_col = periodos[-1]
        top_n = df_filtrado.sort_values(sort_col, ascending=False).head(num_etfs)
        result["found_count"] = len(top_n)

        output_csv = os.path.join(output_dir, f"etf_choice_{timestamp}.csv")
        output_excel = os.path.join(output_dir, f"etf_choice_{timestamp}.xlsx")

        # Agregar columna 'fecha' con la fecha de ejecución
        fecha_hoy = datetime.now().strftime("%Y-%m-%d")
        top_n = top_n.copy()
        top_n["Fecha"] = fecha_hoy
        top_n.to_csv(output_csv, sep=";", index=False, encoding="utf-8-sig")
        result["csv"] = output_csv

        try:
            top_n.to_excel(output_excel, index=False)
            result["xlsx"] = output_excel
        except Exception as e:
            result["xlsx"] = f"Error: {e}"

        return result

    @staticmethod
    def save_excel_file(df, timestamp, output_dir):
        """Guarda datos en formato Excel."""
        # Agregar columna 'fecha' con la fecha de ejecución
        fecha_hoy = datetime.now().strftime("%Y-%m-%d")
        df = df.copy()
        df["Fecha"] = fecha_hoy
        excel_file = os.path.join(output_dir, f"etf_perf_data_{timestamp}.xlsx")
        try:
            df.to_excel(excel_file, index=False)
            return excel_file
        except Exception as e:
            return f"Error: {e}"

    @staticmethod
    def save_valid_tickers(df, timestamp, output_dir, input_csv=INPUT_CSV):
        """Guarda lista de tickers válidos."""
        try:
            original_df = CSVReader.read_original_etf_list(input_csv)
            valid_tickers = df["Ticker"].tolist()
            valid_etf_df = original_df[original_df["Ticker"].isin(valid_tickers)].copy()

            if len(valid_etf_df) < len(original_df):
                today_str = datetime.now().strftime("%Y%m%d")
                last_col = valid_etf_df.columns[-1]
                valid_etf_df[last_col] = today_str
                valid_etf_csv = os.path.join(output_dir, f"List_All_ETF_valid_{timestamp}.csv")
                valid_etf_df.to_csv(valid_etf_csv, index=False, sep=";", encoding="utf-8-sig")
                return valid_etf_csv
            else:
                return None
        except Exception as e:
            return f"Error: {e}"
