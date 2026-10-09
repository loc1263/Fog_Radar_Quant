# Análisis: migrar el backend a Go con 2 workers

*Fecha: 2026-10-04 · Alcance: `backend/` actual (FastAPI + pipelines batch). Solo análisis:
no se modificó código.*

## 1. Resumen ejecutivo

| Opción | Esfuerzo | Riesgo | Recomendación |
|---|---|---|---|
| **A. Todo en Go** (API + pipelines + 2 workers) | Muy alto (≈ 14–20 semanas) | Alto: hay que rehacer la paridad de los 4 módulos | ❌ No conviene hoy |
| **B. Híbrido**: API en Go + 2 workers Python (pipelines sin cambios) | Medio (≈ 5–7 semanas) | Medio | ⚠️ Solo si la API tiene un problema real de rendimiento o despliegue |
| **C. Mantener Python** y ordenar los 2 workers (API + worker de jobs) | Bajo (≈ 1–2 semanas) | Bajo | ✅ **Recomendada** |

**Conclusión:** el cuello de botella del sistema no es la API sino **Yahoo Finance**:
los pipelines tardan entre 10 y 30 minutos porque se espacian a propósito para evitar los 429,
y deben correr **en serie** (lock global `lock_pipelines`). Go no acelera eso. La parte
cuantitativa (pandas, numpy, scipy, PyPortfolioOpt) y los PDF (reportlab + matplotlib) no
tienen equivalentes maduros en Go, y cada módulo tiene tests de paridad firmados contra los
proyectos originales que habría que volver a ganar desde cero. Si el objetivo es tener
"2 workers", se logra en Python sin reescribir nada (opción C).

---

## 2. Qué hay que migrar (inventario)

- **Código:** ~17 500 líneas de Python en `src/` y ~6 400 líneas de tests.
- **API:** 56 endpoints en 4 routers (`/api/cl`, `/api/us`, `/api/etf`, `/api/tendencia`) más
  sistema, hub y buscador; contrato OpenAPI del que el frontend **genera sus tipos**
  (`npm run gen:api`).
- **Pipelines batch** (`python -m fog_radar.jobs`): Chile, EEUU, ETF y Tendencia, con lock
  global, reintentos, registro de jobs (JSONL), cron y alertas por webhook.
- **Historial SQLite** (`core/histdb.py` + `history.db` por módulo, ADR 0003).

### Dependencias Python y su equivalente en Go

| Uso | Python | Go | Brecha |
|---|---|---|---|
| API HTTP | FastAPI + Pydantic + uvicorn | `net/http` + chi/echo, `go-playground/validator` | Baja. OpenAPI hay que generarlo aparte (huma, ogen, swaggo) |
| Settings / `.env` | pydantic-settings | `envconfig`/`koanf` | Baja |
| Rate limit / caché | `limits`, `cachetools` | `x/time/rate`, `ristretto` | Baja |
| SQLite | `sqlite3` | `modernc.org/sqlite` (sin CGO) | Baja |
| Locks entre procesos | `filelock` | `gofrs/flock` | Baja |
| DataFrames | **pandas** (35 imports) | gota / dataframe-go (inmaduros) | **Alta**: winsorizado, z-score, MAD, percentiles, rankings, merges y CSV byte a byte |
| Álgebra / estadística | numpy, scipy | gonum | Media-alta: `scipy.optimize` (SLSQP con restricciones) y `linkage` (HRP) hay que portarlos o reimplementarlos |
| Optimización de portafolio | **PyPortfolioOpt** (Markowitz max-Sharpe con L2, frontera eficiente) | No existe | **Alta**: reimplementar el optimizador y reproducir los pesos a 1e-6 |
| Yahoo Finance | **yfinance** + **curl_cffi** (imita el TLS de un navegador) | No hay cliente mantenido; habría que escribir el scraping (crumb/cookie, quoteSummary, chart) y la imitación de TLS (`utls`/`tls-client`) | **Muy alta**: es lo más frágil del sistema y yfinance absorbe los cambios de Yahoo por nosotros |
| FRED | fredapi | HTTP simple | Baja |
| Scraping (Wikipedia S&P 500, screener) | lxml, BeautifulSoup | goquery | Baja-media |
| Excel | openpyxl | excelize | Baja |
| PDF | **reportlab** (estilo común en `core/pdf`) | gofpdf/maroto | Media-alta: rehacer el diseño de 2 informes |
| Gráficos del PDF | **matplotlib** | go-chart/gonum plot | Media: otra estética, hay que rediseñar |

## 3. Qué significa "2 workers" en este sistema

Hay tres lecturas posibles; el análisis las cubre todas.

1. **2 réplicas de la API** (dos procesos que atienden requests). En Go no hace falta: un
   solo binario ya usa todos los núcleos con goroutines. En Python equivale a
   `uvicorn --workers 2`. **Cuidado:** hoy hay estado en memoria por proceso (caché de
   `/evaluate` con TTL de 5 min, contadores de rate limit, `limite_concurrente`). Con 2
   réplicas, esos límites valen "por proceso", así que se duplican. Hay que moverlos a algo
   compartido (SQLite/Redis) o aceptar el doble.
2. **API + 2 workers de jobs** (procesos que ejecutan los pipelines). Es la lectura más útil:
   la API nunca ejecuta pipelines (regla del proyecto), y los workers toman trabajos de una
   cola (cron, `POST /api/etf/run`, re-análisis).
3. **2 pipelines en paralelo.** **No es posible con Yahoo:** dos descargas simultáneas
   disparan los 429 (por eso existe `lock_pipelines` y `core/yahoo.py` espacia los requests
   globalmente). Lo que sí se puede paralelizar es **cómputo sin red** (re-análisis ETF,
   `--solo-scoring`, backfills, PDF) mientras otro worker descarga.

### Diseño propuesto de los 2 workers (sirve para Go o Python)

```
              ┌──────────────┐  encola   ┌──────────────────────────┐
 cron / API ─►│ cola de jobs │──────────►│ worker 1 "red"           │  Yahoo/FRED/screener
              │ (SQLite)     │           │ 1 job a la vez, en serie │  (respeta el espaciado)
              └──────────────┘           └──────────────────────────┘
                     │                   ┌──────────────────────────┐
                     └──────────────────►│ worker 2 "cómputo"       │  re-análisis ETF, scoring,
                                         │ sin red, sin Yahoo       │  backfills, PDF, Markowitz
                                         └──────────────────────────┘
```

- **Cola en SQLite** (`data/_jobs/jobs.db`): tabla `jobs(id, modulo, tipo, opciones, estado,
  trigger, creado, iniciado, terminado, mensaje, requests_yahoo, n_429)`. Reemplaza el JSONL
  y cubre el pendiente **B1-JOBS** de `PENDIENTES_SQLITE.md` (gráficos en `/estado`).
- Cada worker toma trabajos con `UPDATE … WHERE estado='pendiente' … RETURNING` dentro de una
  transacción (WAL + `busy_timeout`), así no hace falta Redis.
- **Worker 1 (red):** una sola instancia, porque Yahoo exige serialización; mantiene la pausa
  entre módulos y los reintentos actuales (`pipeline_pausa_s`, `pipeline_reintentos`).
- **Worker 2 (cómputo):** trabajos que no tocan la red. Necesita locks **por módulo** en vez
  del lock global, para no re-analizar ETF mientras el worker 1 instala una corrida nueva de
  ETF (ya existe `storage.bloqueo_datos` para eso).
- La API solo **encola** y consulta el estado (`POST /api/jobs`, `GET /api/jobs/{id}`): ningún
  request espera a un pipeline.

## 4. Opción A — Todo en Go

**Ventajas**
- Binario único, imagen Docker de ~20 MB (hoy, Python + pandas/scipy/matplotlib supera los 600 MB).
- Arranque instantáneo, poca memoria, concurrencia nativa.
- Tipado fuerte de punta a punta.

**Costos y riesgos**
- **Paridad:** los 4 módulos tienen paridad firmada (`docs/PARIDAD.md`) contra los originales,
  incluidos CSV byte a byte (ETF) y pesos de Markowitz a 1e-6. En Go cambian el redondeo, el
  formato de los floats al escribir CSV, el orden en los empates (`sort_values` de pandas no
  es estable) y los optimizadores. Hay que volver a ganar cada test y documentar cada
  diferencia como "corrección cuantitativa".
- **Yahoo sin yfinance:** habría que mantener nuestro propio cliente frente a cambios de
  Yahoo (crumb, cookies, endpoints, imitación de TLS). Hoy ese mantenimiento lo hace la
  comunidad de yfinance.
- **Optimizadores:** Markowitz max-Sharpe con L2 y tope por activo (PyPortfolioOpt + scipy
  SLSQP), Black-Litterman y HRP (scipy `linkage`) se reimplementan sobre gonum. Es la parte
  con más riesgo numérico.
- **PDF:** los dos informes (ETF y Tendencia) y sus gráficos se rehacen con otra librería y
  otra estética; hay que cumplir de nuevo el estilo de `core/pdf`.
- Se pierde el código de referencia directo: los originales están en Python y la paridad se
  verificaba leyéndolos lado a lado.

**Estimación (1 desarrollador)**

| Bloque | Semanas |
|---|---|
| Núcleo: settings, errores, auth, rate limit, jobs, SQLite, cola + 2 workers, OpenAPI | 2 |
| Cliente Yahoo + FX + caché de históricos | 2–3 |
| Chile (extractor + scoring + historial) | 1–1,5 |
| EEUU (scanner, scoring, portafolios Markowitz/BL/HRP, evaluación en vivo) | 3–4 |
| ETF (screener, 4 análisis, rotación, archivado, PDF) | 3–4 |
| Tendencia (8 pasos, Markowitz, forecast, momentum, PDF con gráficos) | 3–4 |
| Paridad, E2E, despliegue | 1,5–2 |
| **Total** | **≈ 14–20** |

## 5. Opción B — Híbrido: API en Go + workers Python

- La **API de lectura** (los 56 endpoints) pasa a Go: lee CSV/JSON/SQLite y responde. Casi
  todo es lectura de archivos y armado de JSON.
- Los **2 workers siguen en Python** con los pipelines actuales sin cambios: la paridad se
  conserva.
- Lo que cuesta:
  - Los endpoints con **cómputo** (`/api/us/portfolio`, mejor combinación, evaluación en vivo
    de un ticker con yfinance, PDF de ETF) siguen necesitando Python: o se encolan en el
    worker 2 con espera síncrona (latencia y complejidad), o quedan en un microservicio
    Python, y entonces hay **dos backends**.
  - El contrato OpenAPI y los schemas Pydantic se duplican en Go: hay que regenerar los tipos
    TS desde la API Go y mantener ambos lados sincronizados.
- **Ganancia real:** poca. La API ya responde en milisegundos sobre archivos locales; el
  tráfico esperado es bajo (sitio interno o con pocos usuarios).
- **Esfuerzo:** ≈ 5–7 semanas.

## 6. Opción C — Mantener Python y ordenar los 2 workers (recomendada)

1. **API**: `uvicorn --workers 2` (o 1 si se quiere conservar exactos los límites en memoria).
   Mover a SQLite los contadores de rate limit y de concurrencia si se usan 2 réplicas.
2. **Cola de jobs en SQLite** + **2 procesos worker** (`python -m fog_radar.jobs --worker red`
   y `--worker computo`) con el diseño de la §3. Reutiliza `jobs/runner.py`, el registro de
   jobs y los locks existentes.
3. `POST /api/etf/run` y los re-análisis pasan a encolar en vez de ejecutar dentro del request
   (hoy `POST /run` corre el análisis en el request, bajo el lock).
4. El cron solo encola los programas (`diario`, `semanal`) en vez de ejecutar directamente.
5. Si después se quiere bajar el tamaño de la imagen o la memoria: imagen `slim` y separar
   las dependencias de la API (sin matplotlib/scipy) de las de los workers.

**Esfuerzo:** ≈ 1–2 semanas, sin tocar la lógica cuantitativa ni la paridad.

## 7. Cuándo sí tendría sentido Go

- Si la API pasa a ser **pública con tráfico alto**, o se necesitan conexiones en tiempo real
  (websockets/SSE con cotizaciones).
- Si se reemplaza Yahoo por un **proveedor con API oficial** (Polygon, Tiingo, EODHD): ahí
  desaparece la dependencia de yfinance y curl_cffi.
- Si se decide cerrar la paridad con los originales (archivarlos) y se acepta que los
  resultados cambien levemente, documentándolo como nueva versión de la metodología.

En ese caso, conviene migrar por partes: primero la API de lectura (opción B), después un
módulo piloto (Chile, el más simple) con su test de paridad, y recién al final los
optimizadores y los PDF.

## 8. Riesgos transversales

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Diferencias numéricas Go vs. pandas/scipy | Rankings y pesos distintos | Tests de paridad con tolerancia explícita por métrica; documentar en `PARIDAD.md` |
| Cliente Yahoo propio se rompe | Pipelines sin datos | Mantener yfinance en un worker Python (opción B/C) o pasar a un proveedor con API |
| Límites en memoria con 2 réplicas | Rate limit/caché duplicados | Estado compartido en SQLite |
| Dos workers escribiendo el mismo módulo | Datos inconsistentes | Lock por módulo + un único escritor de `history.db` por módulo |
| Contrato OpenAPI distinto | Front roto | Regenerar `schema.d.ts` y correr `npm run check` y los E2E en CI |
| Doble stack (Go + Python) | Más mantenimiento | Solo si hay una ganancia medible; si no, opción C |

## 9. Recomendación y próximos pasos

1. **Implementar la opción C** (cola SQLite + worker "red" + worker "cómputo"), que además
   cierra el pendiente B1-JOBS.
2. Medir: latencia p95 de la API, memoria por proceso y duración de cada pipeline (con la
   tabla `jobs`).
3. Revisar la decisión de Go solo si esas métricas muestran un problema que Python no
   resuelve, o si cambia el proveedor de datos.
