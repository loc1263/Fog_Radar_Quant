"""
Tests unitarios de etf_ranking_analysis.py (version WEB).

Migrados sin cambios desde ETF_Choice/WEB/backend/analysis (Fase 5); solo cambian los imports.

Adaptado del test_etf_ranking_analysis.py que existia en la raiz del
proyecto (8 tests con unittest sobre mad_z_score/pct_rank/
pasos_crecientes/Score compuesto, ver CLAUDE.md seccion 5bis) -- se perdio
junto con la raiz (eliminada 2026-07-18) y nunca se habia portado a
WEB/backend/analysis/ (mejoras pendientes de WEB/, item 8, ahora
resuelto). Mismos casos sinteticos que la version original; lo unico que
cambia es el import (WEB/backend/analysis/etf_ranking_analysis en vez del
modulo de la raiz).

Todos los datasets son sinteticos y chicos, sin depender de ningun
etf_perf_data_*.csv real -- se pueden correr sin conexion a internet ni
datos de mercado descargados.

Correr con:
    cd WEB/backend/analysis && python -m unittest test_etf_ranking_analysis -v
o, si pytest esta instalado:
    cd WEB/backend/analysis && pytest test_etf_ranking_analysis.py -v
"""

import os
import tempfile
import unittest

import numpy as np
import pandas as pd

from fog_radar.modules.etf.analysis import etf_ranking_analysis as ranking_mod
from fog_radar.modules.etf.analysis.config import PERIOD_COLS
from fog_radar.modules.etf.analysis.etf_ranking_analysis import (
    WEIGHTS,
    mad_z_score,
    pasos_crecientes,
    pct_rank,
)


class TestMadZScore(unittest.TestCase):
    def test_valores_conocidos(self):
        # Serie simetrica: mediana=3, desvios absolutos = [2,1,0,1,2],
        # mediana de esos desvios (MAD) = 1. z = 0.6745 * (x-3)/1.
        serie = pd.Series([1, 2, 3, 4, 5])
        z, mediana, mad = mad_z_score(serie)
        self.assertEqual(mediana, 3)
        self.assertEqual(mad, 1)
        esperado = pd.Series([-1.349, -0.6745, 0.0, 0.6745, 1.349])
        pd.testing.assert_series_equal(z, esperado, check_exact=False, atol=1e-3)

    def test_mad_cero_no_rompe_ni_da_nan(self):
        # Serie constante: mediana=MAD=0. La division por cero debe estar
        # cubierta por la rama especial (devolver todo 0.0, no NaN/inf).
        serie = pd.Series([7.0, 7.0, 7.0, 7.0])
        z, mediana, mad = mad_z_score(serie)
        self.assertEqual(mad, 0)
        self.assertTrue((z == 0.0).all())
        self.assertFalse(z.isna().any())


class TestPctRank(unittest.TestCase):
    def test_ascendente_el_valor_mas_alto_llega_a_100(self):
        serie = pd.Series([10, 20, 30])
        resultado = pct_rank(serie, ascending=True)
        self.assertAlmostEqual(resultado.iloc[2], 100.0)
        # Ascendente: a mayor valor, mayor percentil.
        self.assertTrue(resultado.iloc[0] < resultado.iloc[1] < resultado.iloc[2])

    def test_descendente_el_valor_mas_bajo_llega_a_100(self):
        serie = pd.Series([10, 20, 30])
        resultado = pct_rank(serie, ascending=False)
        self.assertAlmostEqual(resultado.iloc[0], 100.0)
        # Descendente (ej. usado para Beta/Dispersion): a menor valor
        # crudo, mayor percentil -- "mejor" es tener menos de esa metrica.
        self.assertTrue(resultado.iloc[0] > resultado.iloc[1] > resultado.iloc[2])


class TestPasosCrecientes(unittest.TestCase):
    def test_todo_creciente(self):
        fila = pd.Series({col: valor for col, valor in zip(PERIOD_COLS, [1, 2, 3, 4], strict=True)})
        self.assertEqual(pasos_crecientes(fila), len(PERIOD_COLS) - 1)

    def test_todo_decreciente(self):
        fila = pd.Series({col: valor for col, valor in zip(PERIOD_COLS, [4, 3, 2, 1], strict=True)})
        self.assertEqual(pasos_crecientes(fila), 0)


class TestWeights(unittest.TestCase):
    def test_pesos_suman_uno(self):
        # Guarda de regresion: etf_ranking_analysis.py ya tiene un assert
        # equivalente a nivel de modulo, pero un test explicito deja
        # constancia clara de la razon (Score compuesto invalido si los
        # pesos no suman 1.0) y aparece en el reporte de tests, no solo
        # como un ImportError críptico si alguien lo rompe.
        self.assertAlmostEqual(sum(WEIGHTS.values()), 1.0, places=9)


class TestRunScoreCompuestoYOutliers(unittest.TestCase):
    """Test de integracion chico: corre run() sobre un CSV sintetico (20
    ETFs, liquidez y datos siempre completos) con UN valor extremo en
    Performance 1Y (%) para verificar que el filtro de outliers (MAD
    z-score) lo detecta y excluye, y que el Score compuesto se calcula y
    ordena el ranking/top15 correctamente sobre el universo depurado."""

    def setUp(self):
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_dir.cleanup)

        n = 20
        tickers = [f"T{i:02d}" for i in range(n)]
        perf_1y = np.linspace(5, 30, n)
        perf_1y[10] = 500.0  # outlier deliberado, muy por fuera del resto

        self.df = pd.DataFrame(
            {
                "Ticker": tickers,
                "Nombre": [f"ETF Sintetico {i}" for i in range(n)],
                "Category": ["Diversificada"] * n,
                "TipoInstrumento": ["ETF"] * n,
                "Performance 5D (%)": np.linspace(-5, 5, n),
                "Performance 1M (%)": np.linspace(-2, 8, n),
                "Performance 6M (%)": np.linspace(0, 20, n),
                "Performance 1Y (%)": perf_1y,
                "Sharpe Ratio": np.linspace(0.2, 2.0, n),
                "Treynor Ratio": np.linspace(0.5, 5.0, n),
                "Beta": np.linspace(0.5, 1.5, n),
                "52W High": [110.0] * n,
                "52W Low": [90.0] * n,
                "Precio": [100.0] * n,
                "Volumen": np.linspace(20_000, 200_000, n),
                "AUM": np.linspace(5e7, 2e8, n),
            }
        )

        self.ruta_csv = os.path.join(self.tmp_dir.name, "etf_perf_data_20260101.csv")
        self.df.to_csv(self.ruta_csv, index=False)

    def test_outlier_excluido_y_score_ordenado(self):
        resultado = ranking_mod.run(self.ruta_csv)

        self.assertEqual(resultado["n_universo_bruto"], 20)
        self.assertEqual(resultado["n_outliers"], 1)
        self.assertEqual(resultado["n_universo_depurado"], 19)

        # El outlier (T10) no debe aparecer en el universo depurado.
        ranking = resultado["ranking"]
        self.assertNotIn("T10", set(ranking["Ticker"]))
        self.assertIn("T10", set(resultado["excluidos"]["Ticker"]))

        # El ranking completo queda ordenado por Score descendente.
        scores = ranking["Score"].tolist()
        self.assertEqual(scores, sorted(scores, reverse=True))

        # El Top15 es un prefijo del ranking completo (mismo orden).
        top15 = resultado["top15"]
        self.assertEqual(len(top15), min(15, resultado["n_universo_depurado"]))
        self.assertListEqual(top15["Ticker"].tolist(), ranking["Ticker"].tolist()[: len(top15)])

        # Los outputs quedan escritos en la misma carpeta que el CSV de
        # entrada (no en una ruta fija hardcodeada).
        for nombre in (
            "etf_top15_recomendados.csv",
            "etf_ranking_completo.csv",
            "etf_excluidos_outliers.csv",
            "historial_corridas.csv",
        ):
            self.assertTrue(
                os.path.exists(os.path.join(self.tmp_dir.name, nombre)),
                f"Deberia existir {nombre} en la carpeta del CSV de entrada",
            )


class TestCategoriaExcluida(unittest.TestCase):
    """Item 33 de mejoras_web_pendientes.txt (resuelto 2026-07-24): guarda
    de regresion para CATEGORIAS_EXCLUIR, agregado a etf_ranking_analysis.py
    el 2026-07-24 -- hasta este test ningun caso cubria que un ETF de una
    categoria excluida (Ultrashort Bond, Securitized Bond - Focused,
    Target Maturity) nunca llegue al ranking, sin importar que tan buenas
    sean sus metricas."""

    def setUp(self):
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_dir.cleanup)

        n = 20
        tickers = [f"T{i:02d}" for i in range(n)]
        categorias = ["Diversificada"] * n

        # ETF adicional de una categoria excluida, con metricas MEJORES
        # que todo el resto del universo (Sharpe/Treynor mas altos, Beta
        # mas bajo) -- si el filtro de categoria no funcionara, este
        # ticker deberia liderar el ranking por Score. El test confirma
        # que ni siquiera participa.
        tickers.append("VRIG")
        categorias.append("Ultrashort Bond")

        df = pd.DataFrame(
            {
                "Ticker": tickers,
                "Nombre": [f"ETF Sintetico {t}" for t in tickers],
                "Category": categorias,
                "TipoInstrumento": ["ETF"] * (n + 1),
                "Performance 5D (%)": list(np.linspace(-5, 5, n)) + [10.0],
                "Performance 1M (%)": list(np.linspace(-2, 8, n)) + [10.0],
                "Performance 6M (%)": list(np.linspace(0, 20, n)) + [10.0],
                "Performance 1Y (%)": list(np.linspace(5, 30, n)) + [10.0],
                "Sharpe Ratio": list(np.linspace(0.2, 2.0, n)) + [10.0],
                "Treynor Ratio": list(np.linspace(0.5, 5.0, n)) + [50.0],
                "Beta": list(np.linspace(0.5, 1.5, n)) + [0.01],
                "52W High": [110.0] * (n + 1),
                "52W Low": [90.0] * (n + 1),
                "Precio": [100.0] * (n + 1),
                "Volumen": list(np.linspace(20_000, 200_000, n)) + [100_000.0],
                "AUM": list(np.linspace(5e7, 2e8, n)) + [1e8],
            }
        )

        self.ruta_csv = os.path.join(self.tmp_dir.name, "etf_perf_data_20260101.csv")
        df.to_csv(self.ruta_csv, index=False)

    def test_categoria_excluida_nunca_aparece(self):
        resultado = ranking_mod.run(self.ruta_csv)

        # El universo bruto SI cuenta a VRIG (se mide antes del filtro de
        # categoria, ver n_universo_bruto en run()) -- solo el ranking, el
        # top15 y los excluidos deben quedar sin el.
        self.assertEqual(resultado["n_universo_bruto"], 21)

        for tabla in ("ranking", "top15", "excluidos"):
            self.assertNotIn(
                "VRIG",
                set(resultado[tabla]["Ticker"]),
                f"VRIG (categoria excluida) no deberia aparecer en '{tabla}'",
            )


class TestOrdenExcluidosPorZScoreConSigno(unittest.TestCase):
    """Item 33 (resuelto 2026-07-24): guarda de regresion para el orden de
    la tabla de excluidos -- debe quedar ordenada por Z_Score CON signo,
    descendente (positivos extremos primero, negativos extremos al final),
    no por |Z_Score| (cambio del 2026-07-24, ver CLAUDE.md seccion 2 y
    mejoras_web_historico.txt)."""

    def setUp(self):
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp_dir.cleanup)

        n = 20
        tickers = [f"T{i:02d}" for i in range(n)]
        perf_1y = np.linspace(5, 30, n)
        # 2 outliers deliberados en la misma ventana (1Y): uno MUY por
        # encima del resto (z positivo grande) y otro MUY por debajo (z
        # negativo grande, de menor magnitud absoluta que el positivo).
        # Si el orden fuera por |Z_Score| en vez de por signo, no habria
        # garantia de que todos los positivos queden antes que todos los
        # negativos -- con signo, si.
        perf_1y[5] = 300.0  # outlier positivo extremo
        perf_1y[15] = -100.0  # outlier negativo extremo (|z| menor que el positivo)

        df = pd.DataFrame(
            {
                "Ticker": tickers,
                "Nombre": [f"ETF Sintetico {i}" for i in range(n)],
                "Category": ["Diversificada"] * n,
                "TipoInstrumento": ["ETF"] * n,
                "Performance 5D (%)": np.linspace(-5, 5, n),
                "Performance 1M (%)": np.linspace(-2, 8, n),
                "Performance 6M (%)": np.linspace(0, 20, n),
                "Performance 1Y (%)": perf_1y,
                "Sharpe Ratio": np.linspace(0.2, 2.0, n),
                "Treynor Ratio": np.linspace(0.5, 5.0, n),
                "Beta": np.linspace(0.5, 1.5, n),
                "52W High": [110.0] * n,
                "52W Low": [90.0] * n,
                "Precio": [100.0] * n,
                "Volumen": np.linspace(20_000, 200_000, n),
                "AUM": np.linspace(5e7, 2e8, n),
            }
        )

        self.ruta_csv = os.path.join(self.tmp_dir.name, "etf_perf_data_20260101.csv")
        df.to_csv(self.ruta_csv, index=False)

    def test_orden_descendente_con_signo(self):
        resultado = ranking_mod.run(self.ruta_csv)
        z_scores = resultado["excluidos"]["Z_Score"].tolist()

        self.assertGreaterEqual(len(z_scores), 2, "Se esperaban al menos 2 outliers en este dataset")
        # Orden descendente CON signo: cada valor >= al siguiente.
        self.assertEqual(z_scores, sorted(z_scores, reverse=True))
        # En particular, todos los positivos deben quedar antes que todos
        # los negativos -- lo que NO esta garantizado si se ordenara por
        # |Z_Score| en vez de por el valor con signo.
        primer_negativo = next((i for i, z in enumerate(z_scores) if z < 0), len(z_scores))
        self.assertTrue(all(z >= 0 for z in z_scores[:primer_negativo]))
        self.assertTrue(all(z < 0 for z in z_scores[primer_negativo:]))


if __name__ == "__main__":
    unittest.main()
