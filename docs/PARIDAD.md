# Checklist de paridad por módulo

Un módulo se da por migrado cuando, con los mismos datos, el sitio nuevo muestra lo mismo que
el proyecto original. Las diferencias intencionales quedan anotadas.

## Correcciones cuantitativas (Fase 7, 2026-09-29)

Una revisión con criterios estándar de finanzas cuantitativas encontró tres errores de cálculo
heredados de los originales. Se corrigieron; **desde esta fecha esas salidas ya no son iguales
al original a propósito**. Los tests de paridad siguen comparando todo lo demás y verifican la
relación exacta entre el valor nuevo y el original.

| Módulo | Error en el original | Corrección | Cómo se verifica |
|---|---|---|---|
| EEUU · Black-Litterman | π = δΣw es retorno **en exceso** de rf, pero se usaba como retorno total y el optimizador de Sharpe le restaba rf otra vez. Con views neutras (score 50) no recuperaba los pesos de mercado (p. ej. 77/23/0 en vez de 67/22/11 con rf = 4 %). | π = δΣw + rf (igual que `pypfopt.market_implied_prior_returns`); Q y el posterior quedan en retornos totales. | `test_black_litterman`: π, Q y posterior = original + rf (exacto); `test_black_litterman_con_views_neutras_recupera_el_mercado`. La "mejor combinación BL" cambia en consecuencia. |
| Tendencia · proyección 6 m | `exp((μ − σ²/2)·T)` con μ = CAGR de `pypfopt.mean_historical_return`: trata el CAGR como tasa continua (sobrestima con μ alto) y descuenta la volatilidad dos veces (subestima con σ alta). | Mediana = P·(1+μ)^T; banda 90 % = mediana·exp(±1,645·σ·√T) (el ancho no cambia). | `test_parity`: todas las columnas iguales salvo la proyección, que cumple nuevo = original·exp((ln(1+μ) − μ + σ²/2)·T); `test_trend_forecast`. |
| ETF · beta (y Treynor) | Retornos del ETF y del mercado emparejados **por posición** (últimos N de cada serie): si faltaba un día, todo lo anterior quedaba desfasado y el beta se sesgaba hacia 0. | Emparejados **por fecha** (join interno). | `test_performance_y_riesgo` (serie con un día faltante → β ≈ 1). Afecta a la próxima corrida del screener; los CSV ya generados no cambian. |

Otras observaciones de la revisión que cambian la **metodología** (no son errores de cálculo)
quedaron en `docs/BACKLOG.md` (TD-MODEL-4 … TD-MODEL-9) para decidirlas con un backtest.

## Chile / IPSA (`Stock_Choice_Nacional`) — ✅ 2026-09-27

| Ítem | Cómo se verificó | Estado |
|---|---|---|
| Ranking (scoring) | `tests/cl_stocks/test_parity.py`: indicadores del 24-09 y del 25-09 → `ranking_acciones_*.csv` **byte a byte idéntico** al generado por `scoring_acciones.py` | ✅ |
| Orden y puesto (empates `min`) | Mismo test, fila a fila (30 acciones) | ✅ |
| Recálculo con pesos en el navegador | `src/lib/modules/cl/scoring.spec.ts`: con los pesos por defecto reproduce orden, puesto y score del backend (±0,001 por el redondeo a 4 decimales del CSV) | ✅ |
| Duplicados holding/filial | Test unitario + badge "grupo N" en la tabla y aviso en el resumen | ✅ |
| Dividendo sostenible (payout ≤ 100 %) | Test unitario | ✅ |
| Conversión de moneda del extractor | `tests/cl_stocks/test_extractor.py` con dobles de `yf.Ticker` (sin red) | ✅ |
| Endpoint ranking | `/api/ranking` → `/api/cl/ranking` (claves snake_case, incluye `modelo`) | ✅ |
| Endpoint indicadores | `/api/indicadores` → `/api/cl/indicadores` (catálogo de 61 columnas con unidad) | ✅ |
| KPIs (acciones, mejor score, cobertura, duplicados) | Visual, contra el original | ✅ |
| Panel de pesos (0–3, paso 0,25, % del score, Restablecer) | E2E `chile.e2e.ts` | ✅ |
| Búsqueda por símbolo/nombre y filtro por sector | E2E | ✅ |
| Detalle por acción (4 grupos de métricas) | E2E | ✅ |
| Pipeline completo contra Yahoo | Pendiente: correr en Windows (`uv run python -m fog_radar.jobs cl_stocks`) | ⏳ |

**Diferencias intencionales**

- El filtro de sector ya **no re-numera** el ranking: el puesto se calcula sobre todo el
  universo y el filtro solo oculta filas (en el original, filtrar recalculaba los puestos
  dentro del subconjunto).
- Se agregó: vista `/chile/indicadores` (el original tenía el endpoint pero no la vista),
  top 10 en gráfico, exportación CSV, fecha de los datos, modo oscuro.
- Las acciones sin ningún pilar con dato quedan con puesto vacío (en el original tomaban el
  número de fila).

## EEUU (`Stock_Choice_EEUU`) — ✅ 2026-09-27

| Ítem | Cómo se verificó | Estado |
|---|---|---|
| Lógica portada | Los 110 tests originales (`test_fundamentals`, `test_indicators`, `test_portfolio`, `test_scanner`, `test_scoring`, `test_ticker_eval`, `test_universe`) pasan sin cambios de aserciones | ✅ |
| Score de salud | `test_parity.py`: `evaluate_score` reproduce el score guardado de los 60 candidatos de la corrida 2026-09-25T21-00-01Z | ✅ |
| Momentum 12-1 y retornos | Recalculados desde `prices.json` = valores de `candidates.json` (rel. 1e-9) | ✅ |
| Black-Litterman (π, Q, posterior) | Pesos/retornos iguales a los del `portfolio.py` original sobre el top 15 real (`fixtures/portfolio_expected.json`) | ✅ |
| Markowitz, HRP, mejor combinación | Eliminados por decisión de producto (`docs/MODELO_NEW.md`): el portafolio EEUU es un modelo único (clusters + BL con tope de peso); sin original con el que comparar | — |
| Mejor combinación (3 métodos) | Mismas combinaciones, mismo orden y mismos pesos | ✅ |
| API | `test_api.py` (port de `test_main.py`): mismas respuestas, con prefijo `/api/us`, API key y errores `{detail, code}` | ✅ |
| Corridas históricas | Las 12 corridas del original validan contra los esquemas Pydantic | ✅ |
| Vistas | Top 15, Candidatos, Evaluación, Performance, Portafolio (modelo único) y evaluación en vivo: E2E `eeuu.e2e.ts` | ✅ |
| Corrida completa contra Yahoo / evaluación en vivo | Pendiente en Windows (sin acceso a Yahoo en esta sesión) | ⏳ |

**Diferencias intencionales**

- Sin corridas: **503 `no_data`** (antes 404). Ticker inválido: 422/404 sin llamar a Yahoo.
- `/evaluate` con caché de 5 min (`cached: true` en la respuesta) y rate limit.
- `risk_free_rate` acepta `null` (= 4 % por defecto).
- `prices` acepta `?days=` y `?tickers=`: las sparklines piden solo 60 días (antes se enviaban ~14 meses de todos los tickers).
- `k` por defecto 4 (antes 5): con el top 15, C(15,5) = 3.003 superaba el tope y el botón arrancaba deshabilitado.
- Nuevo: tabla π / Q / posterior de Black-Litterman, enlace "evaluar" en cada ticker, filtro "crecientes" en la URL, página `/eeuu/ticker` con buscador sin JavaScript.
- La vista "Performance" se llama "Rendimiento".

## ETF (`ETF_Choice`) — ✅ 2026-09-28

| Ítem | Cómo se verificó | Estado |
|---|---|---|
| Análisis (ranking, Top 15 simple, excluidos, momentum, categorías, Top 15 Cap., historial) | `test_parity.py`: con el `etf_perf_data_20260924.csv` real y el mismo histórico, los **10 CSV** de salida son **byte a byte idénticos** a los del original | ✅ |
| Rotación (mantienen / nuevos / salidas, racha, procedencia, destino) y diversificación | Mismo resultado que `rotacion.py` original corrido sobre sus carpetas `data` y `data_hist` (`fixtures/expected/rotacion.json`) | ✅ |
| Tests unitarios del original | `test_etf_ranking_analysis.py` migrado sin cambios de aserciones (`tests/etf/test_ranking_analysis.py`) | ✅ |
| Pipeline (staging → archivo con secuencia `_2` → instalación, respaldo a `data_old/`) | `test_pipeline.py` con el screener reemplazado por un doble | ✅ |
| API | 14 endpoints del original bajo `/api/etf` (`test_api.py`); `/api/status` → `/api/etf/resumen` | ✅ |
| PDF | `GET /api/etf/pdf`: mismas 8 secciones y columnas, Letter apaisado, estilo común `core/pdf` | ✅ |
| 13 secciones del panel | Rutas `/etf/*`, E2E `etf.e2e.ts` + revisión visual | ✅ |
| Screener contra Yahoo | Pendiente en Windows (`uv run python -m fog_radar.jobs etf`, ~30 min) | ⏳ |

**Diferencias intencionales**

- Sin corrida: **503 `no_data`** (antes 404); lock ocupado: **503 `busy`**.
- `POST /api/etf/run` re-analiza el CSV vigente en *staging* (antes escribía directo en `data/`) y queda en `/api/jobs` con `trigger: api`; rate limit 3/min; 409 si ya hay una corrida.
- El screener corre **dentro del mismo proceso** (antes, subproceso por el choque de nombres `config.py`); rutas por parámetro.
- Claves snake_case en la API (`perf_1y`, `sharpe`, `score_comp`…); los parámetros del modelo (pesos, umbrales, categorías excluidas) viajan en `/api/etf/resumen`.
- Cada sección es una URL propia (`/etf/top15`, `/etf/rotacion`…) en vez de `#hash`.
- Todas las tablas ordenables y con CSV (antes solo "Top 15 Vanguard" ordenaba; Excluidos y Categorías no exportaban). Universo completo: 50 filas por página y búsqueda.
- La lista de tickers de una categoría se abre con clic (antes, hover).
- PDF con tildes y encabezado "ETFs — Informe Ejecutivo".

## Tendencia (`Stock_Tendencia`) — ✅ 2026-09-28

| Ítem | Cómo se verificó | Estado |
|---|---|---|
| Markowitz (universo, top 15, portafolios de 5 y 10), proyección GBM 6 m, momentum mensual y semanal, notas de régimen | `test_parity.py`: con `data/processed` + universo + macro de la corrida real **20260926_01**, los 23 CSV/JSON/TXT de salida son iguales al original (rtol 1e-7: Windows y Linux difieren en el último dígito de algunos floats). Pesos redondeados y textos: idénticos | ✅ |
| `dashboard_data.json` (payload completo, 2 análisis) | Igual al archivado por el original (salvo `generated_at` / `run_dir`) | ✅ |
| Armado del dataset (`build_dataset`, paso 3) | Verificado a mano con `data/raw` completo del original: `prices_usd.csv`, `weekly_returns.csv`, `risk_free_rate.txt` y `excluded_tickers.txt` iguales (no se versiona: 3,3 MB de precios crudos) | ✅ |
| Tests del original | Los 12 archivos de `backend/tests` migrados (**73 tests**; se quitaron `test_health` y `test_cors`, cubiertos por los tests comunes) | ✅ |
| API histórica | `/analyses`, `/runs`, `/runs/{run_dir}`, `/history/*` bajo `/api/tendencia` con el mismo SQL sobre `history.db` (6 corridas copiadas) | ✅ |
| Última corrida sin JSON estático | `GET /api/tendencia/latest` (reemplaza `frontend/public/data/latest/dashboard_data.json`) | ✅ |
| PDFs / archivos de `output/` | `GET /api/tendencia/runs/{run_dir}/files/{nombre}` (reemplaza el servidor estático en :8000) | ✅ |
| Pipeline | `python -m fog_radar.jobs tendencia` con las opciones del original + `--macro` y `--backfill`; test de integración sin Yahoo (PDF incluido) | ✅ |
| Vistas | resumen, portafolio (5/10/15), tendencia 6 m, sectores, momentum, limitaciones, historial, descargar: E2E `tendencia.e2e.ts` + revisión visual | ✅ |
| Corrida completa contra Yahoo / FRED | Pendiente en Windows (`uv run python -m fog_radar.jobs tendencia --macro`, ~10 min) | ⏳ |

**Diferencias intencionales**

- Datos en `data/tendencia/` (`raw`, `processed`, `macro`, `universe`, `output`, `logs`, `history.db`); `settings.py` pasa a `config.py` y su raíz sale de `Settings` (`paths.py`).
- El pipeline ya no copia el JSON a `frontend/public`: el sitio lo pide a la API. Sin corridas: **503 `no_data`**.
- Un nombre de corrida que no es `YYYYMMDD_NN` → 422 (antes 404); archivos solo `.pdf/.csv/.json/.txt` de la carpeta de la corrida.
- Carlito vive en `core/pdf/fonts` (compartida con el PDF de ETF).
- Nuevo: `/api/tendencia/excluidos` y aviso visible de tickers excluidos por moneda (TD-OPS-2, TD-DATA-1); validación de factibilidad de la configuración antes de correr (TD-MODEL-3).
- La sección "Tendencia 6m" vive en `/tendencia/proyeccion`; el análisis y el tamaño del portafolio van en la URL (`?analisis=us&n=5`); los filtros del historial también (formulario GET, sin JavaScript).
- Gráficos del historial por corrida (dos corridas del mismo día se distinguen: `24/09 #2`).
- Fase 7: la proyección a 6 meses usa la mediana P·(1+CAGR)^T (ver "Correcciones cuantitativas"); tablas del PDF con encabezado claro, números alineados a la derecha y textos ≥ 10 pt; cualquier moneda ISO 4217 tiene par FX (antes solo 6).
