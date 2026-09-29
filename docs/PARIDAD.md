# Checklist de paridad por módulo

Un módulo se da por migrado cuando, con los mismos datos, el sitio nuevo muestra lo mismo que
el proyecto original. Las diferencias intencionales quedan anotadas.

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
| Markowitz, frontera, Black-Litterman, HRP | Pesos/retornos iguales a los del `portfolio.py` original sobre el top 15 real (`fixtures/portfolio_expected.json`) | ✅ |
| Mejor combinación (3 métodos) | Mismas combinaciones, mismo orden y mismos pesos | ✅ |
| API | `test_api.py` (port de `test_main.py`): mismas respuestas, con prefijo `/api/us`, API key y errores `{detail, code}` | ✅ |
| Corridas históricas | Las 12 corridas del original validan contra los esquemas Pydantic | ✅ |
| Vistas | Top 15, Candidatos, Evaluación, Performance, Markowitz/BL/HRP (+ mejor combinación) y evaluación en vivo: E2E `eeuu.e2e.ts` | ✅ |
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
