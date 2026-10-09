# Fog_Radar_Quant — Roadmap de migración del backend a Go

*Fecha: 2026-10-08 · Alcance: `backend/` (FastAPI + pipelines batch, ~17 200 líneas Python, 55 endpoints, 4 módulos).
Complementa `docs/ANALISIS_MIGRACION_GO.md` (2026-10-04), que recomendaba mantener Python. Aquí se
asume la decisión ya tomada: **migrar todo a Go usando [`gohoo`](https://github.com/loc1263/gohoo)** como cliente de Yahoo Finance.*

## 1. Principios (no relitigar)

1. **Estrangulador, no big bang.** Python sigue sirviendo todo lo que Go aún no migró; Caddy enruta por prefijo
   (`/api/cl`, `/api/us`, `/api/etf`, `/api/tendencia`). Se apaga Python solo al final.
2. **El contrato no cambia.** Mismo JSON, mismas rutas y mismo OpenAPI: el frontend (SvelteKit, BFF) no se toca.
   `npm run gen:api` debe producir un `schema.d.ts` equivalente al actual (diff revisado por módulo).
3. **Python es el oráculo.** Cada módulo se da por migrado cuando sus tests de paridad (`tests/<modulo>/fixtures/` +
   `test_parity.py`) pasan en Go con tolerancia explícita por métrica. Las diferencias intencionales se documentan en
   `docs/PARIDAD.md` ("Correcciones cuantitativas").
4. **Formato en disco intacto** (`data/<modulo>/`, CSV/JSON/xlsx del original, `history.db`, ADR 0003). Python y Go
   conviven leyendo/escribiendo los mismos archivos durante la transición, con el mismo lock.
5. **Medir antes y después.** La Fase 0 fija una línea base; cada fase reporta la mejora contra ella.

## 2. Qué se gana y qué no (expectativa realista)

El análisis previo concluyó que los pipelines tardan 10–30 min porque `core/yahoo.py` **espacia a propósito** los
requests (anti-429) y los corre en serie (`lock_pipelines`). Go no acelera eso por sí solo. Sí acelera:

- cómputo (scoring, rankings, Markowitz, forecast, backfills): decenas de veces sin pandas/GIL;
- latencia y memoria de la API (binario único, ~20 MB de imagen vs. >600 MB);
- paralelismo de lo que **no** usa red (re-análisis ETF, PDF, backfills) mientras otro job descarga.

**Entregable de la Fase 0:** perfil que diga cuánto del "lento" es espera de Yahoo y cuánto es CPU/pandas/IO. Si el
grueso es espera, la mejora de wall-clock vendrá del paralelismo controlado (ver Fase 2) y no del lenguaje; hay que
saberlo antes de gastar semanas.

## 3. Análisis de `gohoo` frente a lo que usa el backend

Lo que `gohoo` ofrece (según su README): `NewClient`/opciones (`WithRetryPolicy`, `WithLazyBootstrap`…),
`QuoteSummary`, `FundamentalsTimeseries`, `Chart`/`ChartSeries`/`ChartRange`, `Monthly()`, `ComputeIndicators`
(SMA, EMA, RSI, MACD, Bollinger, ATR…), helpers JSON, errores tipados (`ErrRateLimit`, `Retry-After`), cliente
concurrente-seguro, sin dependencias externas, MIT.

| Uso en el backend Python | Cobertura gohoo | Brecha / acción |
|---|---|---|
| `Ticker.history` / `yf.download` (precios) | ✅ `ChartSeries`/`ChartRange`, `Monthly` | Falta caché a disco (`yahoo.descargar_historial`) y descarga por lote → adaptador propio (Fase 2) |
| `Ticker.info` (cl_stocks, ETF, US, Tendencia) | ✅ `QuoteSummary` (price, statistics…) | Mapear campos de `info` de yfinance → módulos de quoteSummary; test de equivalencia por campo |
| `quarterly_income_stmt`, `quarterly_cashflow` (Chile) | ✅ `FundamentalsTimeseries` | Verificar tipos/claves equivalentes y signos |
| FX (`core/fx.py`: `EURUSD=X`, `GBp`, `ZAc`, `ILA`) | ⚠️ no documentado | Spike: `Chart` sobre tickers `XXXUSD=X`; la lógica de subunidades es nuestra |
| `yf.screen` / `EquityQuery` (Tendencia, descubrimiento por región) | ❌ | **Contribuir a gohoo** el endpoint de screener, o portar directo en `core` |
| Sesión `curl_cffi` (imita TLS de navegador) | ❌ usa `net/http` | **Riesgo alto:** Yahoo puede bloquear la huella TLS de Go. Spike en Fase 0; plan B: `utls`/`tls-client` vía `WithHTTPClient` |
| Espaciado global + backoff 429 (`Espaciador`, `ControlTrafico`) | ⚠️ solo reintentos | Se reimplementa en `core/yahoo` Go sobre `RetryPolicy` + limitador global |
| Indicadores (`us_stocks/indicators.py`) | ✅ `ComputeIndicators` | Reusar; paridad sobre SMA/RSI/etc. |
| Búsqueda de tickers, noticias, opciones | ❌ | Solo si algún endpoint lo usa (verificar en Fase 0) |

**Acciones sobre gohoo (repo propio, así que se corrige en origen):**
- Etiquetar una versión (hoy no hay releases) y fijarla en `go.mod`.
- Añadir: screener (`EquityQuery`), helper de FX, caché opcional, y `time/tzdata` embebido (nota de Windows).
- Mantener sus tests con `httptest` y añadir fixtures de respuestas reales de Yahoo para detectar cambios.

## 4. Inventario y mapa Python → Go

| Python | Go (propuesta) |
|---|---|
| FastAPI + Pydantic + OpenAPI | `huma` sobre `chi` (genera OpenAPI 3.1 desde structs) |
| pydantic-settings / `.env` | `koanf` o `envconfig` (+ `godotenv`) |
| `limits` / `cachetools` | `x/time/rate` (por IP) / `ristretto` o `otter`; semáforo para `limite_concurrente` (503 `busy`) |
| `sqlite3` (`histdb`, jobs) | `modernc.org/sqlite` (sin CGO), mismas migraciones |
| `filelock` | `gofrs/flock` (mismo archivo de lock que Python durante la transición) |
| pandas / numpy | structs + slices tipados, `gonum` (stat, mat), helpers propios (`winsorize`, z-score, MAD, percentiles, ranking estable) |
| scipy (`optimize.minimize`, `linkage`) / PyPortfolioOpt | `core/quant`: QP/SLSQP propio sobre `gonum`, `linkage` (HRP) propio |
| lxml / bs4 / `read_html` | `goquery` |
| openpyxl / `to_excel` | `excelize` |
| reportlab + matplotlib (`core/pdf`) | `maroto`/`gofpdf` + `go-chart` o SVG→PDF; fuentes Carlito embebidas |
| fredapi / requests | `net/http` |
| pypdf (macro) | `pdfcpu` o `ledongthuc/pdf` |
| `python -m fog_radar.jobs` | `cmd/fogradar` (subcomandos `api`, `job`, `worker`, `backfill`) |

## 5. Estructura objetivo

```
backend-go/
  cmd/fogradar/            # main: api | job <modulo> | worker | backfill | crontab
  internal/
    core/                  # settings, logging, auth, ratelimit, jobs, histdb, fx, yahoo, pdf, quant
    modules/{etf,us,cl,tendencia}/   # scoring, extractor, storage, schemas, service, router, pipeline, hub
    transversal/           # /api/hub, /api/buscar
  testdata/<modulo>/       # fixtures del original (copiadas de tests/<modulo>/fixtures)
  go.mod                   # github.com/loc1263/gohoo pinneado
```

Convenciones: una carpeta por módulo igual que ADR 0002; JSON snake_case; fechas con `Settings.timezone`;
todo endpoint con tipo de respuesta explícito (equivale a `response_model`); `go vet`, `staticcheck`, `golangci-lint`.

## 6. Fases

Duraciones para 1 desarrollador. Total estimado: **≈ 15–19 semanas** (gohoo recorta ~1 semana de cliente Yahoo, pero
el screener, TLS y los optimizadores añaden trabajo de contingencia).

### Fase 0 — Línea base y spikes de riesgo (1 sem) — ✅ ejecutada el 2026-10-08 (resultados en `docs/BASELINE_GO.md`; quedan 2 pendientes que dependen de acciones fuera de este equipo)
- Perfilar los 4 pipelines y los endpoints más usados: tiempo en espera de Yahoo vs. CPU vs. disco; p95 de la API;
  memoria por proceso. Guardar como `docs/BASELINE_GO.md`.
- **Spike A — TLS/huella:** con `gohoo` descargar 200 tickers seguidos con el espaciado actual; medir tasa de 429/403
  frente a `curl_cffi`. Decide si hace falta `utls`.
- **Spike B — Cobertura de datos:** comparar `info`, históricos y fundamentales de ~30 tickers (EEUU, Chile `.SN`, ETF,
  FX) entre yfinance y gohoo; listar campos faltantes.
- **Spike C — Optimización:** reproducir max-Sharpe con L2 y topes (PyPortfolioOpt) con un QP en gonum sobre un fixture
  real; medir diferencia de pesos.
- Congelar **golden outputs** del Python actual (respuestas JSON de cada endpoint y archivos de cada pipeline en modo
  `--offline`) como oráculo de paridad.
- *Salida:* informe de go/no-go por riesgo; si el Spike A o C fallan, el plan B correspondiente se adelanta a la Fase 2/4.

### Fase 1 — Fundaciones del servicio Go (2 sem) — 🟡 núcleo ejecutado el 2026-10-08 (ver «Estado de la Fase 1» al final)
- Esqueleto `backend-go/`, CI (build, vet, lint, tests), Dockerfile multi-stage (imagen final distroless/scratch).
- `core`: settings/`.env`, logging estructurado, auth por API key (comparación en tiempo constante), `rate_limit` por
  IP, `limite_concurrente`, manejo de errores con el mismo envelope que Python.
- `GET /api/status`, `/api/jobs` (registro JSONL existente, solo lectura), `histdb` con migraciones compatibles
  (mismas `MIGRACIONES`; Go nunca edita una publicada) y endpoints `/history/*` solo lectura.
- OpenAPI generado por huma; script que compara contra el OpenAPI de FastAPI y lista diferencias.
- Caddy: enrutamiento por prefijo con *fallback* a Python.
- *Criterio:* `/api/status` y `/history/*` de un módulo responden idéntico desde Go.

### Fase 2 — `core/yahoo`, `core/fx` y caché de históricos (2 sem) — ✅ ejecutada (ver §13)
- Adaptador sobre `gohoo`: espaciador global con jitter, backoff por 429/`Retry-After`, modo `Interactivo()` sin
  backoff largo, estadísticas (requests, 429) hacia el registro de jobs.
- `DescargarHistorial` con caché a disco (frescura 20 h) y descarga por lote **con concurrencia acotada** (primer
  punto donde Go puede reducir el tiempo real: pipeline de descarga/cálculo en paralelo respetando el límite global).
- `core/fx`: spot `FxRates`, `SerieAUSD`, subunidades GBp/ZAc/ILA; tests de paridad con `tests/test_core_fx.py`.
- Contribuciones a gohoo identificadas en la Fase 0 (screener, tz, versión).
- *Criterio:* port de `test_core_yahoo.py` y `test_core_fx.py` en verde; informe de tasa de 429 ≤ la de Python.

### Fase 3 — `core/quant`: biblioteca numérica y utilidades de datos (2 sem) — ✅ ejecutada salvo `core/pdf` (ver §13)
Se hace **antes** de los módulos porque EEUU, ETF y Tendencia dependen de ella.
- Estadística: retornos, volatilidad, covarianza (`sample_cov`), winsorizado, z-score, MAD, percentiles, ranking con
  desempate **estable y documentado** (pandas no garantiza estabilidad).
- Optimización: max-Sharpe con L2 y topes, frontera eficiente, Black-Litterman, HRP (`linkage` + `fcluster`).
- Lectura/escritura de CSV con el **mismo formato de floats** que pandas (ETF exige byte a byte).
- `core/pdf` (estilo común: paleta, Carlito, tablas, encabezado/pie, textos ≥ 10 pt) y gráficos.
- *Criterio:* tests de paridad numérica contra fixtures de scipy/PyPortfolioOpt (pesos a 1e-6 o la tolerancia firmada).

### Fase 4 — Módulo piloto: Chile (`/api/cl`) (1,5 sem) — ✅ ejecutada (ver §13)
- Extractor (gohoo: `FundamentalsTimeseries` + `QuoteSummary`), `scoring`, `columnas`, `storage` (xlsx con `excelize`),
  `history`, `hub`, 6 endpoints, pipeline + CLI.
- Primer módulo que se enruta a Go en Caddy (canary: 10 % → 100 %, con comparación de respuestas Go vs. Python).
- *Criterio:* `tests/cl_stocks` portados y en verde; `PARIDAD.md` firmado; E2E `chile.e2e.ts` en verde.

### Fase 5 — Módulo EEUU (`/api/us`) (3 sem) — ✅ ejecutada (ver §13)
- Universo (Wikipedia con `goquery`), `scanner`, `fundamentals`, `indicators` (gohoo), `scoring`, `portfolio`
  (Markowitz/BL/HRP sobre `core/quant`), evaluación en vivo de un ticker (única ejecución de Yahoo dentro de un request:
  `Interactivo()`, caché con TTL 5 min, `rate_limit` + `limite_concurrente`).
- 11 endpoints, `history`, `hub`, pipeline.
- *Criterio:* `test_parity.py`, `test_portfolio.py`, `test_api.py` portados; `eeuu.e2e.ts` en verde.

### Fase 6 — Módulo ETF (`/api/etf`) (3 sem) — ✅ ejecutada (ver §13)
- Screener/discovery (validadores, rendimiento por periodos, beta vs. índice con caché), `writer` xlsx/CSV byte a byte,
  4 análisis (ranking, momentum, categoría, Russell 3000), rotación, archivado, `storage.bloqueo_datos`.
- Informe PDF (`informe_pdf`), `POST /run` (lock global `lock_pipelines`), 19 endpoints, `history` (el más grande, 572 líneas).
- Aquí aparece el mayor beneficio de paralelismo en cómputo (re-análisis sin red).
- *Criterio:* paridad byte a byte de los CSV; `etf.e2e.ts` y tests de pipeline en verde.

### Fase 7 — Módulo Tendencia (`/api/tendencia`) (3–4 sem) — 🟡 ejecutada salvo el screener de universo (ver §13)
- Ingesta (`build_dataset`, precios en USD vía `core/fx`), descubrimiento de universo (screener por región — depende de
  la contribución a gohoo), macro (FRED + VIX + lectura de comunicado), `markowitz`, `momentum`, `trend_forecast`
  (GBM con la corrección ya documentada en PARIDAD), `dashboard_data`, informe PDF con gráficos, `run_files`,
  `history_writer`, `backfill`.
- 14 endpoints, `hub`, pipeline.
- *Criterio:* pesos de Markowitz dentro de tolerancia; PDF revisado visualmente contra el estándar ejecutivo; E2E verde.

### Fase 8 — Jobs, CLI, cron y transversales (1,5 sem) — 🟡 ejecutada salvo la cola con workers (ver §13)
- `fogradar job <modulo> [--offline --solo --todos --programa --trigger --pausa --reintentos --top-n --max-weight]`
  con las mismas opciones y códigos de salida (0 ok, 1 falla, 2 uso, 3 ocupado).
- Runner con lock global, reintentos, pausa entre pipelines, alertas por webhook, registro de jobs.
- `crontab` generado, `GET /api/hub`, `GET /api/buscar`.
- Opcional (de `ANALISIS_MIGRACION_GO.md`): cola en SQLite y 2 workers (**red**, 1 a la vez; **cómputo**, en paralelo)
  con locks por módulo. Esta es la ganancia de wall-clock más grande y natural en Go.
- *Criterio:* `--todos --offline` produce los mismos artefactos que Python; `test_jobs_cli`/`test_jobs_runner` portados.

### Fase 9 — Endurecimiento, corte y retiro de Python (1,5 sem) — ⏳ pendiente: requiere corrida en sombra y despliegue (ver §13)
- Corrida **en sombra** 1–2 semanas: cron ejecuta Go y Python en directorios separados y un script compara resultados.
- Pruebas de carga y de 429; revisión de seguridad (auth, rate limit, rutas/paths, SSRF en descargas).
- Docker + Caddy de producción (2 servidores: frontend Node / backend Go), `deploy/`, crontab, `.bat` locales
  (`iniciar.bat`, `correr_pipelines.bat`, `iniciar_dev.py`).
- Actualizar `CLAUDE.md` (sección Backend), `docs/METODOLOGIAS.md`, `docs/PARIDAD.md`, `ROADMAP.md`, `BACKLOG.md` y
  crear `docs/ADR/000N-backend-go.md`. Mover `backend/` a `_backups/` y retirar `uv`/Ruff/pytest del flujo.
- *Criterio:* una semana sin diferencias materiales; rollback documentado (reactivar el prefijo en Caddy).

## 7. Cronograma

| Fase | Semanas | Acumulado |
|---|---|---|
| 0 Línea base y spikes | 1 | 1 |
| 1 Fundaciones | 2 | 3 |
| 2 Yahoo / FX / caché | 2 | 5 |
| 3 Quant + PDF | 2 | 7 |
| 4 Chile (piloto) | 1,5 | 8,5 |
| 5 EEUU | 3 | 11,5 |
| 6 ETF | 3 | 14,5 |
| 7 Tendencia | 3–4 | 17,5–18,5 |
| 8 Jobs / CLI / cron | 1,5 | 19–20 |
| 9 Corte | 1,5 | 20,5–21,5 |

Las fases 4–7 se pueden solapar parcialmente con 2 personas; con 1 persona, el total realista es **≈ 15–19 semanas**
si la Fase 8 se simplifica (sin cola de 2 workers) y se superponen pruebas con desarrollo.

## 8. Riesgos y mitigaciones

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Yahoo bloquea la huella TLS de `net/http` | Sin datos | Spike A (Fase 0); `utls`/`tls-client` vía `WithHTTPClient`; fallback temporal a worker Python |
| gohoo sin screener / campos de `info` distintos | Universo Tendencia y ETF incompletos | Contribuir a gohoo en Fase 2; tests de equivalencia de campos |
| Endpoints de Yahoo cambian | Roturas | Fixtures de respuestas reales + CI nocturno de contrato contra Yahoo |
| Optimizador Go ≠ PyPortfolioOpt | Pesos y rankings distintos | Spike C; tolerancia firmada; documentar en PARIDAD |
| CSV byte a byte (floats, orden de empates) | Paridad ETF rota | Escritor de floats compatible con pandas; ranking estable documentado |
| PDF con otra estética | Informes distintos | Revisión visual contra `core/pdf` y el estándar ejecutivo; textos ≥ 10 pt |
| Dos stacks conviviendo | Complejidad temporal | Fases por módulo, un dueño por prefijo en Caddy, plazo máximo definido |
| Espera de Yahoo domina el tiempo | La migración "no acelera" | Fase 0 lo cuantifica; paralelismo de cómputo y workers (Fase 8) |
| Zona horaria en Windows | Fechas erróneas | `import _ "time/tzdata"` en el binario |
| Dependencia de un único mantenedor (gohoo) | Bloqueo | Es repo propio: versionar, fijar en `go.mod`, vendoring |

## 9. Definición de terminado (por módulo)

1. Tests portados en verde (unitarios, API, historial, paridad) + `go vet`/lint sin avisos.
2. OpenAPI equivalente; `npm run gen:api && npm run check` del frontend sin cambios inesperados; E2E del módulo en verde.
3. Paridad firmada en `docs/PARIDAD.md` y `docs/METODOLOGIAS.md` actualizada si cambió algo.
4. Prefijo enrutado a Go en Caddy con corrida en sombra sin diferencias.
5. Métricas (tiempo de pipeline, p95, memoria) comparadas con la línea base de la Fase 0.

## 10. Resultado de la Fase 0 (2026-10-08)

- **TLS:** `net/http` + gohoo no es bloqueado (0 × 429/403 en ~330 requests). `utls` pasa a plan B.
- **Datos:** cierres idénticos a yfinance; `info` requiere mapeo (`dividendYield` ×100, módulo fijo para `forwardPE`).
- **Optimizador:** QP propio reproduce PyPortfolioOpt a 1.6e-10 en 25 ms → riesgo retirado.
- **Rendimiento:** el espaciado es solo ~15 % del tiempo; el resto es latencia en serie. Espaciar inicios + 4 workers
  da 2.2× de throughput sin 429 (EEUU ~760 s → ~350 s). La API ya responde en 13 ms (mediana): el problema está en los pipelines.
- **Perfil por etapa:** el cómputo es ≈ 3 % del tiempo de los pipelines; 95–99 % es red (EEUU: 89 % en 503 `Ticker.info`;
  Tendencia: 97 % en descubrimiento y precios; ETF: >99 % en el screener).
- **Escala:** 1 000 requests (500 ETFs × info + chart) en 350 s, 4 workers, 0 × 429 / 0 errores.
- **Chile:** `FundamentalsTimeseries` idéntico a yfinance en los 30 tickers del IPSA.
- **Bug de producción hallado y corregido (ETF):** `tempfile.mkdtemp` en Windows dejaba los CSV instalados ilegibles
  (causa del 503 en ETF y de la corrida fallida del 7-oct); ver `docs/BASELINE_GO.md` §1.
- **Ajuste al plan:** la concurrencia acotada del cliente Yahoo (Fase 2) pasa a ser el entregable de mayor valor.

## 11. Estado de la Fase 1 (2026-10-08)

**Hecho** (código en `backend-go/`, instrucciones en `backend-go/README.md`):
- Esqueleto `cmd/fogradar` + `internal/{api,core,modules}`; `huma` sobre `chi`; `go vet`, `golangci-lint` y `go test -race` en verde.
- `core`: settings/`.env` con las reglas de prod, errores con el envelope `{detail, code}`, API key en tiempo constante,
  rate limit por IP (ventana deslizante), `Gate` (= `limite_concurrente`, 503 `busy`), CORS, registro de jobs (lectura,
  con `stale`), `histdb` (`Filas` solo lectura, `Migrar` con `user_version`, `DiffCorridas`), programación del cron.
- Endpoints: `/api/health`, `/api/status`, `/api/<modulo>/status`, `/api/jobs*`, `/api/cl/history/*` (4).
- **Criterio cumplido:** `golden/compare.py` — 35 caminos (incluye 404/422, ticker en minúsculas, métricas inválidas, diff
  con y sin parámetros) **sin diferencias** contra Python sobre los datos reales; `golden/openapi_diff.py` — las 13 rutas con
  la misma forma de OpenAPI (parámetros, códigos, propiedades, requeridas, nulabilidad, `date-time`).
- CI (`backend-go` en `.github/workflows/ci.yml`), `Dockerfile` multi-stage (distroless), `deploy/caddy/Caddyfile.migracion`.

**Pendiente de la Fase 1:**
- [ ] Probar `docker build` (el daemon de Docker no estaba corriendo en esta sesión; el binario estático linux/amd64 sí compila; con todos los módulos pesa ~25 MB).
- [ ] Probar el `Caddyfile.migracion` con Caddy real (no instalado aquí) y el fallback a Python.
- [ ] Historial de ETF, EEUU y Tendencia (cada uno con su módulo: SQL propio de cada `history.py`).
- [ ] `npm run gen:api` contra el OpenAPI de Go y revisar el diff de `schema.d.ts` (se hará al enrutar el primer módulo, Fase 4).
- [ ] Escritura en `data/_jobs` y lock global compartido con Python (`gofrs/flock`): se necesita recién en la Fase 8.

## 12. Próximo paso inmediato

Ver §13 («Pendiente»). Lo que queda ya no es código de módulos sino operación: corrida en sombra, `docker build`, Caddy real,
y la revisión visual de los dos informes PDF.

## 13. Estado de las Fases 2–8 (2026-10-09)

Todo el código vive en `backend-go/`; `go vet`, `golangci-lint` (0 issues) y `go test -race ./internal/...` en verde.

**Hecho**
- **Fase 2** — `internal/yahoo` (espaciador global, backoff 429, `Interactive(ctx)`, estadísticas) y `internal/fx`
  (spot, `ToUSD` con semántica `reindex`+`ffill`, subunidades GBp/ZAc/ILA). Caché de históricos en el mismo CSV que pandas.
  Calibración de `info` (dividendYield ×100, `forwardPE` de `summaryDetail`, PEG de fundamentals-timeseries) en `spikes/`.
- **Fase 3** — `internal/quant`: `Frame` con lectura/escritura de CSV idéntica a pandas (`PyRepr`), suma por pares de numpy,
  cuantiles, `PctRank`, media de Kahan, QP (ADMM) con max-Sharpe (Cornuéjols–Tütüncü), covarianza por pares + corrección
  PSD, `clean_weights`, linkage promedio + `fcluster`. Fixtures numéricos generados con scipy/PyPortfolioOpt.
- **Fase 4 (Chile)** — extractor, scoring, XLSX/CSV, historial (lectura y escritura), hub, pipeline. CSV byte a byte con los
  fixtures de Python. Pipeline completo: 43 s (Python: 115 s).
- **Fase 5 (EEUU)** — universo (Wikipedia), scanner, fundamentos, scoring, portafolio (Markowitz / Black-Litterman / HRP /
  `POST optimize`), evaluación en vivo de un ticker (`Interactive`, caché TTL, rate limit + `Gate`), historial, hub, pipeline.
  Corrida completa ≈ 5 min (Python ≈ 12,7 min), 0 errores con manejo de 429.
- **Fase 6 (ETF)** — informe PDF (`internal/pdf`, compartido con Tendencia: Carlito, tablas con encabezado claro, pie con «Página X de Y»; `GET /api/etf/pdf`
  con caché por firma de los CSV) y screener (rendimientos por período derivados de **un** gráfico diario de 5 años), 4 análisis, rotación,
  historial, hub, pipeline. CSV byte a byte con los fixtures, salvo el orden de empates (ver `docs/PARIDAD_GO.md`).
- **Fase 7 (Tendencia)** — lectura completa (14 endpoints) y escritura: informe PDF (cards, tablas y gráficos de barras dibujados), dataset en USD, Markowitz (análisis completo y solo
  EEUU, top-N y portafolios reducidos), proyección a 6 meses, momentum, notas de régimen, macro (FRED + VIX + FOMC),
  `dashboard_data.json`, `store_run` y `--backfill`, `run_manifest.json`. **Paridad**: `internal/modules/tend/tend_test.go`
  contra `backend/tests/tendencia/fixtures` (rtol 1e-7) y una corrida real offline Go vs Python con los mismos datos de
  entrada: CSV y JSON idénticos (la columna de empates en pesos 0 puede cambiar de orden) y `history.db` equivalente.
- **Fase 8** — `fogradar job <modulo|--todos>` con las mismas opciones y códigos de salida (0/1/2/3), lock global
  compartido con Python (`gofrs/flock`), reintentos, pausas, `--list`, `--crontab`; hub y buscador global.
- **Enrutamiento**: `deploy/caddy/Caddyfile.migracion` manda todo `/api/*` a Go, con *fallback* a Python.
- **Contrato**: `golden/compare.py` (110 caminos, 0 diferencias), `golden/openapi_diff.py` (61 rutas, 0 diferencias) y
  los 89 esquemas compartidos generan el mismo `schema.d.ts`.

**Pendiente**
- [ ] **Revisión visual de los PDF** de ETF y Tendencia contra los de Python (el maquetado de `go-pdf/fpdf` no es idéntico al de
  ReportLab: mismos anchos, fuentes y paleta; el salto de página puede variar; los gráficos de Tendencia se dibujan sin matplotlib).
- [ ] **Screener de universo de Tendencia** (EquityQuery de Yahoo: crumb + POST): `gohoo` no lo expone. Go reutiliza el
  snapshot de hoy o el último (como `--skip-discovery`) y, sin ninguno, la lista de respaldo. Opciones: contribuir el
  screener a `gohoo` o dejar el descubrimiento semanal en Python.
- [ ] Cola en SQLite con workers (red 1 / cómputo N): opcional, no bloquea el corte.
- [ ] **Fase 9**: corrida en sombra 1–2 semanas, `docker build` (el daemon no corría), Caddy real, prueba de carga,
  actualizar `CLAUDE.md`, `docs/METODOLOGIAS.md`, `docs/PARIDAD.md`, `ROADMAP.md` y `docs/BACKLOG.md`
  (propuestas en `docs/PARIDAD_GO.md` y `docs/ADR/0004-backend-go.md`), mover `backend/` a `_backups/`.
- [ ] Fase 0: desbloquear `data/etf` (ACL, como administrador) y repetir el Spike A desde la IP de producción.
