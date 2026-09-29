"""
Config compartida entre etf_ranking_analysis.py, etf_momentum_analysis.py,
backtest_metodologia.py y generar_informe_pdf.py (mejora pendiente #13
resuelta 2026-07-10).

Antes cada script repetia por su cuenta TOP_N y los nombres de columna de
las 4 ventanas de rendimiento / metricas de riesgo, con un comentario tipo
"debe coincidir con <otro script>". Si alguien cambiaba un valor en un
script y se olvidaba en otro, el backtest podia terminar comparando un
Top15 contra un Top-N distinto (o una columna de rendimiento distinta) sin
ningun error visible -- el riesgo concreto que motivo este modulo. Ahora
los 4 scripts importan estas constantes desde aca en vez de redefinirlas.

Lo que NO vive aca (a proposito): parametros que son especificos de un
solo script y no se repiten literalmente en ningun otro, como
LIQ_VOL_MIN/LIQ_AUM_MIN/Z_THRESHOLD/WEIGHTS/CATEGORIAS_EXCLUIR (solo
etf_ranking_analysis.py, esta ultima agregada 2026-07-24), PESO_M_5D..1Y/
CATEGORIAS_EXCLUIR (solo etf_momentum_analysis.py) y
CATEGORIAS_EXCLUIR_RANKING (solo etf_categoria_analysis.py, agregado
2026-07-18). Mover esos aca solo agregaria indireccion sin reducir ningun
riesgo real de desincronizacion.

Nota 2026-07-24: los 3 scripts de analisis tienen HOY su propia lista de
categorias excluidas, cada una con su propio nombre, criterio de curacion
y momento en que se aplica (etf_ranking_analysis.py sobre el CSV crudo
antes de rankear; etf_momentum_analysis.py sobre el CSV crudo tambien,
pero con una lista mas amplia de categorias defensivas; etf_categoria_
analysis.py sobre etf_ranking_completo.csv ya filtrado por el primero, ver
la nota en ese mismo script sobre por que su filtro quedo redundante -- ver
mejoras_web_historico.txt item 31). Las 3 comparten un nucleo comun de 3
categorias (Ultrashort Bond, Securitized Bond - Focused, Target Maturity)
desde que el usuario pidio sacarlas "de todos los analisis", pero
CATEGORIAS_EXCLUIR de etf_momentum_analysis.py tiene ademas ~15 categorias
defensivas mas que las otras 2 listas no tienen -- no son intercambiables,
sigue siendo intencional que cada una viva en su propio script.
"""

# Tamano del ranking final / Top N, usado por los 3 scripts de analisis
# (etf_ranking_analysis.py, etf_momentum_analysis.py) y por
# backtest_metodologia.py (para recortar el Top 15 de cada corrida
# historica al comparar contra el rendimiento realizado).
TOP_N = 15

# Columnas de las 4 ventanas de rendimiento que llegan en etf_perf_data,
# en orden 5D -> 1M -> 6M -> 1Y. Se usan tanto para el filtro de
# completitud de datos como para los componentes de tendencia/momentum de
# ambos analisis (principal y Momentum/Crecimiento).
PERIOD_COLS = [
    "Performance 5D (%)",
    "Performance 1M (%)",
    "Performance 6M (%)",
    "Performance 1Y (%)",
]
COL_5D, COL_1M, COL_6M, COL_1Y = PERIOD_COLS

# Columnas de metricas de riesgo-retorno provistas por el proveedor.
COL_SHARPE = "Sharpe Ratio"
COL_TREYNOR = "Treynor Ratio"
COL_BETA = "Beta"
RISK_COLS = [COL_SHARPE, COL_TREYNOR, COL_BETA]
