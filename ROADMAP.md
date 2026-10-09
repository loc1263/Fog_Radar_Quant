# Fog_Radar_Quant — Mapa de ruta de unificación

> Fecha: 2026-09-26 · Autor: Luis (con Claude)
> Objetivo: unir 4 proyectos cuantitativos en **un solo sitio web** con
> **SvelteKit + TypeScript + Tailwind** (frontend) y **Python + FastAPI** (backend).

---

## 1. Punto de partida (inventario)

| # | Proyecto | Qué hace | Frontend actual | Backend actual | Datos / pipeline | Tests |
|---|---|---|---|---|---|---|
| 1 | `ETF_Choice` | Ranking de ETFs (Top 15, momentum, categorías, rotación, Vanguard, Russell 3000, PDF) | Astro 7 + React 19 + Tailwind **v3** + Recharts | FastAPI `main.py` (15 endpoints `/api/*`, incl. `POST /api/run` y `GET /api/pdf`) | `actualizar_datos_semanal.py` (cron lunes) → `data/` + `data_hist/` (CSV) · `filelock` | Vitest (frontend) |
| 2 | `Stock_Choice_EEUU` | Screener momentum large caps EEUU + score de salud + optimización de portafolio | Astro 7 + React 19 + Tailwind **v4** + shadcn | FastAPI `app/main.py` (8 endpoints, 2 `POST` de portafolio, 1 evaluación en vivo) | `batch.py` → `data/runs/<run_id>/*.json` | Pytest (8 módulos) |
| 3 | `Stock_Choice_Nacional` | Ranking multi-factor IPSA (scoring con pesos ajustables en el cliente) | Vite + React 19 + Tailwind v4 + shadcn | FastAPI `api.py` (3 endpoints) | `pyYahooStockAnalisis.py` → `scoring_acciones.py` → CSV/XLSX con timestamp | — |
| 4 | `Stock_Tendencia` | Markowitz + tendencia 6m + momentum Asia/EEUU, historial SQLite, PDF | Vue 3 + Vite + Tailwind v4 + Chart.js | FastAPI `src/stock_tendencia/api/app.py` (9 endpoints **sin prefijo** `/api`) + JSON estático `dashboard_data.json` | `run_pipeline.py` (step00→step07) → `output/<fecha>_NN/` + `history.db` | Pytest + CI GitHub + Ruff |

**Observaciones clave**

- **Los 4 backends ya son FastAPI** → la unificación del backend es de *integración*, no de reescritura.
- **Los 4 frontends se reescriben** (React ×3, Vue ×1 → Svelte). Es el grueso del trabajo.
- 3 de los 4 ya comparten el sistema de diseño **"Dashboard ejecutivo"** (`DESIGN_SYSTEM.md`, tokens `brand-*`, ámbar `#9A6E00` / azul `#1B3A6B`). `Stock_Choice_Nacional` no lo usa todavía. → Es la base visual natural del sitio unificado.
- Todos los pipelines son **batch** (escriben archivos); las APIs son mayormente de **solo lectura**. Excepciones: `ETF POST /api/run`, `EEUU POST /api/portfolio/*` y `EEUU GET /api/tickers/{t}/evaluate` (yfinance en vivo).
- Hay código repetido entre proyectos: wrappers anti-bloqueo de yfinance (`robust_yf.py`, pausas), conversión FX, winsorización/z-score, generación de PDF con ReportLab, CSV export, formateo.
- Solo `Stock_Tendencia` tiene git. Hay ~740 líneas de "mejoras pendientes" repartidas en 4 archivos.
- Hay archivos `.env` (p. ej. clave FRED en `Stock_Tendencia/backend/.env`) → no deben terminar en el repo nuevo.

---

## 2. Arquitectura objetivo

### 2.1 Estructura del monorepo

```
Fog_Radar_Quant/
├── backend/
│   ├── pyproject.toml              # un solo entorno (uv), Python 3.12/3.13
│   ├── .env.example                # FRED_API_KEY, rutas de datos, CORS...
│   ├── src/fog_radar/
│   │   ├── main.py                 # create_app(): monta routers (solo API, no sirve el frontend)
│   │   ├── settings.py             # pydantic-settings (rutas, puertos, claves)
│   │   ├── core/                   # código compartido
│   │   │   ├── yahoo.py            # yfinance común (espaciado global, 429, caché)
│   │   │   ├── fx.py               # tasas de cambio
│   │   │   ├── stats.py            # winsorización, z-score, momentum 12-1, indicadores
│   │   │   ├── pdf/                # estilos ReportLab comunes (Calibri/Carlito, paleta)
│   │   │   ├── runs.py             # "última corrida", historial, locks de archivo
│   │   │   └── jobs.py             # registro/estado de ejecuciones de pipelines
│   │   └── modules/
│   │       ├── etf/          { router.py, schemas.py, analysis/, pipeline.py }
│   │       ├── us_stocks/    { router.py, schemas.py, scanner/, portfolio.py, pipeline.py }
│   │       ├── cl_stocks/    { router.py, schemas.py, extractor.py, scoring.py, pipeline.py }
│   │       └── tendencia/    { router.py, schemas.py, steps/, history_db.py, pipeline.py }
│   ├── jobs/                       # CLI: python -m fog_radar.jobs <modulo> [--full]
│   └── tests/{core,etf,us_stocks,cl_stocks,tendencia}/
├── data/                           # fuera del código, ignorado por git
│   ├── etf/{current,hist}/
│   ├── us_stocks/runs/
│   ├── cl_stocks/
│   └── tendencia/{output,history.db,macro}/
├── frontend/                       # SvelteKit + TS + Tailwind v4 · adapter-node (SSR)
│   ├── src/
│   │   ├── app.css                 # tokens brand-* (claro/oscuro) del Dashboard ejecutivo
│   │   ├── hooks.server.ts         # headers de seguridad, logging, manejo de errores
│   │   ├── lib/
│   │   │   ├── server/api.ts       # cliente del backend (SOLO servidor): URL privada + API key
│   │   │   ├── api/types.ts        # tipos generados desde el OpenAPI de FastAPI
│   │   │   ├── components/ui/      # shadcn-svelte (Card, Table, Badge, Tabs, Button, Input...)
│   │   │   ├── components/shared/  # DataTable ordenable/filtrable, KpiCard, ScoreBar, TickerLink,
│   │   │   │                       #  SignalBadge, Sparkline, ThemeToggle, CsvButton, ErrorState
│   │   │   ├── charts/             # wrappers de gráficos (línea, barras, frontera eficiente)
│   │   │   └── utils/              # format, csv, theme
│   │   └── routes/                 # ver 2.3
│   └── tests/ (vitest + playwright)
├── deploy/
│   ├── backend/   Dockerfile · docker-compose.yml · Caddyfile · crontab (pipelines)
│   └── frontend/  Dockerfile · docker-compose.yml · Caddyfile
├── scripts/  run_all_pipelines.py · fixtures_e2e.py   (raíz: iniciar_dev.py · iniciar.bat · correr_pipelines.bat)
├── docs/     DESIGN_SYSTEM.md · METODOLOGIAS.md · ADR/
├── .github/workflows/ci.yml
└── ROADMAP.md
```

### 2.2 Decisiones técnicas

| Tema | Decisión | Motivo |
|---|---|---|
| Backend | **Una sola app FastAPI**, un `APIRouter` por módulo, prefijo `/api/<modulo>` | Un proceso, un puerto, un OpenAPI; cada módulo sigue aislado. |
| Gestor Python | **uv** + `pyproject.toml` único; Python **3.12 o 3.13** | `Stock_Tendencia` exige `<3.14`; ETF ya ajustó pisos de numpy para 3.13. |
| Contratos API | **Modelos Pydantic** en `schemas.py` + tipos TS generados con `openapi-typescript` | Una sola fuente de verdad; el front no se desincroniza del back. |
| Frontend | **SvelteKit 2 + Svelte 5 (runes)** con **`adapter-node`** (SSR) (✅ decidido 2026-09-27) | Se publicará en **2 servidores productivos separados**: uno para el frontend (Node) y otro solo para el backend (FastAPI). SSR entrega las páginas ya armadas con datos. |
| Comunicación front ↔ back | Patrón **BFF** (*backend for frontend*): el navegador solo habla con SvelteKit; SvelteKit llama a FastAPI desde el servidor (`+page.server.ts`, form actions y `+server.ts`) | El backend no queda expuesto al público: solo acepta requests del servidor del frontend. Sin CORS en el navegador y la API key nunca llega al cliente. |
| Seguridad del backend | Header `X-API-Key` obligatorio + firewall/allowlist de la IP del servidor frontend + HTTPS | Protege `POST /api/etf/run`, optimización de portafolio y evaluación en vivo (consumen Yahoo/CPU). |
| Configuración | Front: `PRIVATE_API_URL`, `PRIVATE_API_KEY`, `ORIGIN` (`$env/dynamic/private`) · Back: `API_KEY`, `ALLOWED_ORIGINS`, rutas de datos | Mismo código en dev y producción; solo cambian variables de entorno. |
| Dev | 2 procesos: `vite dev` (:5173) + `uvicorn` (:8000); `PRIVATE_API_URL=http://localhost:8000` | Replica la topología de producción desde el primer día. |
| Deploy | Docker por servidor + **Caddy** (HTTPS automático). Ej.: `quant.midominio.cl` (front) y `api.midominio.cl` (back) | Despliegues independientes: se puede actualizar el front sin tocar el back y viceversa. |
| UI | **Tailwind v4** + **shadcn-svelte** (bits-ui) + tokens del *Dashboard ejecutivo* | Continuidad con shadcn (EEUU, Nacional) y con el diseño ya consensuado. |
| Gráficos | **LayerChart 2.5** (✅ decidido 2026-09-27; integrado vía el `chart` de shadcn-svelte) | Librería nativa de Svelte (sobre D3/Layer Cake), componible, estilable con clases Tailwind y tokens `brand-*`. Reemplaza Recharts (ETF) y Chart.js (Tendencia). Cubre línea, barras, área, scatter (frontera eficiente) y sparklines. |
| Tablas | Componente `DataTable` genérico propio (orden, filtro, CSV) | Casi todas las pestañas son tablas; hoy hay 4 implementaciones distintas. |
| Pipelines | Siguen siendo **batch fuera del proceso web** (CLI). En dev: Programador de tareas de Windows. En producción: **cron en el servidor backend** (donde viven los datos); la API solo lee | Mantiene el diseño actual (evita bloqueos de Yahoo en requests web). |
| Estado de corridas | `core/jobs.py` + endpoint `/api/jobs` (última corrida, duración, estado, log) | Permite al hub mostrar "datos al día / desactualizados" de los 4 módulos. |
| Tendencia "última corrida" | Pasa de JSON estático en `public/` a **endpoint** `/api/tendencia/latest` | Todo el front lee de la API; desaparece el acople back→carpeta del front. |
| Datos | Carpeta `data/` raíz por módulo, rutas en `settings.py` | Separa código de datos; un solo lugar para respaldar. |
| Asistencia IA para Svelte | **Svelte MCP oficial** (`https://mcp.svelte.dev/mcp`, ✅ conectado 2026-09-27): `list-sections`, `get-documentation`, `svelte-autofixer`, `playground-link` | Documentación actualizada de Svelte 5/SvelteKit y análisis estático del código generado. **Regla en `CLAUDE.md`: todo `.svelte` / `.svelte.ts` pasa por `svelte-autofixer` hasta quedar sin issues antes de darlo por terminado.** |
| Calidad | Ruff + pytest (back), `svelte-check` + ESLint + Prettier + Vitest + Playwright (front), CI en GitHub | Extiende lo que ya hacen Tendencia y ETF. |

### 2.3 Mapa de rutas del sitio

| Ruta SvelteKit | Contenido | Origen |
|---|---|---|
| `/` | **Hub**: 4 tarjetas (una por módulo) con fecha de última corrida, KPI principal y top 3; accesos directos | Nuevo |
| `/etf` → `/etf/resumen` | Resumen de la corrida | ETF |
| `/etf/[seccion]` | top15 · universo · excluidos · rotacion · diversificacion · categorias · top15-simple · top15-vanguard · momentum · russell3000 · historial · descargar | ETF (13 pestañas) |
| `/eeuu` | Top 15 momentum + datos de la corrida | Stock_Choice_EEUU ✅ |
| `/eeuu/candidatos` · `/eeuu/evaluacion` · `/eeuu/rendimiento` | Vistas del screener | Stock_Choice_EEUU ✅ |
| `/eeuu/portafolio` | modelo único: clusters de correlación + Black-Litterman (`docs/MODELO_NEW.md`) | Stock_Choice_EEUU ✅ |
| `/eeuu/ticker` · `/eeuu/ticker/[ticker]` | Buscador + evaluación en vivo de un ticker | Stock_Choice_EEUU ✅ |
| `/chile` | Ranking IPSA + panel de pesos ajustables | Stock_Choice_Nacional |
| `/chile/indicadores` | Tabla de indicadores crudos | Stock_Choice_Nacional |
| `/tendencia/[seccion]?analisis=…` | resumen · portafolio · tendencia · sectores · momentum | Stock_Tendencia |
| `/tendencia/historial` · `/tendencia/limitaciones` | Historial SQLite y limitaciones | Stock_Tendencia |
| `/estado` | Estado de pipelines (`/api/jobs`) y salud del backend | Nuevo |

Layout global: header con logo *Fog Radar Quant*, menú principal de 4 módulos, sub-navegación por módulo (tabs como links reales — cada pestaña es una URL compartible), toggle claro/oscuro, footer con disclaimer "no es asesoramiento financiero".

### 2.4 Mapeo de endpoints (antes → después)

| Módulo | Antes | Después |
|---|---|---|
| ETF | `/api/status`, `/api/top15`, `/api/top15-simple`, `/api/top15-vanguard`, `/api/ranking-completo`, `/api/excluidos`, `/api/diversificacion`, `/api/rotacion`, `/api/momentum/top15`, `/api/categorias/ranking`, `/api/russell3000`, `/api/historial`, `/api/pdf`, `POST /api/run` | `/api/etf/<mismo sufijo>` |
| EEUU | `/api/rankings/latest[/candidates\|/prices]`, `/api/rankings/runs`, `/api/tickers/{t}/evaluate`, `POST /api/portfolio/optimize` | `/api/us/<mismo sufijo>` |
| Chile | `/api/ranking`, `/api/indicadores` | `/api/cl/ranking`, `/api/cl/indicadores` |
| Tendencia | `/analyses`, `/runs`, `/runs/{dir}`, `/history/*` + `dashboard_data.json` estático | `/api/tendencia/<mismo sufijo>` + `/api/tendencia/latest` |
| Global | 4 × `/api/health` | `/api/health` (agrega salud de los 4 módulos) + `/api/jobs` |

---

## 3. Plan de trabajo por fases

Estrategia: **"cortes verticales"**. Primero el esqueleto común, luego se migra **un módulo completo (back + front) a la vez**, empezando por el más simple para validar la arquitectura. Los proyectos originales **no se tocan** hasta la fase final: siguen funcionando como referencia y respaldo.

Orden de migración de módulos: **Chile → EEUU → ETF → Tendencia**
(de menor a mayor complejidad: 3 → 8 → 15 endpoints; Tendencia al final por su pipeline de 8 pasos, SQLite y el cambio de JSON estático a API).

### Fase 0 — Preparación ✅ completada 2026-09-27
- [x] `git init` en `Fog_Radar_Quant` (rama `main`), `.gitignore` (data/, .env, node_modules, caches, output/, _backups/).
- [x] `CLAUDE.md` del monorepo con las convenciones (Svelte 5 runes, uso obligatorio del Svelte MCP + `svelte-autofixer`, patrón BFF, tokens `brand-*`) + `README.md`.
- [x] Svelte MCP conectado en Cowork ✅ (probado: `list-sections` y `svelte-autofixer`). En Claude Code: `claude mcp add -t http -s project svelte https://mcp.svelte.dev/mcp`.
- [x] Respaldo de los 4 proyectos en `_backups/<Proyecto>_2026-09-27.zip` (sin node_modules/venv/caches; integridad verificada con `unzip -t`). Los originales no se modifican.
- [x] Triage → `docs/BACKLOG.md`: 61 ítems abiertos → **34 A** (durante la migración) · **23 B** (después) · **3 C** (descartar) · **1 decisión** (TD-LIC-1: repo privado, sin LICENSE ✅).
- [x] `docs/DESIGN_SYSTEM.md` único (base: Stock_Tendencia; adaptado a SvelteKit + LayerChart; módulo de tema Svelte 5 validado con `svelte-autofixer`).
- [x] `backend/pyproject.toml` + `uv.lock` (Python 3.12, 88 paquetes). Resuelve e instala en venv limpio. Versiones clave: fastapi 0.141 · pandas 2.3.3 · numpy 2.5 · scipy 1.18 · yfinance 1.7 · PyPortfolioOpt 1.6 · reportlab 4.5.
- [x] **Compatibilidad verificada** corriendo los tests originales contra el entorno unificado: EEUU 137/137 ✅ · Tendencia 75/75 ✅ · ETF 10/10 ✅ · Chile (sin tests) imports ✅. Las 4 apps FastAPI originales importan sin error.
- Nota: aparecen `DeprecationWarning` de NumPy por `Timedelta` con unidad genérica (EEUU `scanner.py` y pandas `resample`). No rompe hoy; corregir al portar (Fases 4 y 6).

### Fase 1 — Esqueleto del backend ✅ completada 2026-09-27
- [x] `create_app()` (`backend/src/fog_radar/main.py`) con logging, errores uniformes `{detail, code}` (`core/errors.py`, oculta detalles internos en prod), `/api/health` público y el resto protegido por `require_api_key` (`X-API-Key`, comparación en tiempo constante).
- [x] CORS restringido a `ALLOWED_ORIGINS` (solo GET/POST, sin credenciales).
- [x] `settings.py` (pydantic-settings): `ENV` dev/test/prod, `API_KEY`, `ALLOWED_ORIGINS` (lista o "a,b"), `DATA_DIR`, `RATE_LIMIT_DEFAULT`. En prod exige API key ≥ 24 caracteres y prohíbe orígenes localhost; `/docs` y `/openapi.json` se apagan.
- [x] Rate limiting como dependencia (`core/ratelimit.py`, librería `limits`): límite global por usuario final (IP reenviada por el BFF en `X-Forwarded-For`) + `rate_limit("N/minute", scope=…)` para endpoints costosos. *slowapi se descartó: su middleware no reconoce las rutas con FastAPI 0.14x y no limitaba nada.*
- [x] `core/runs.py` (última corrida por nombre o mtime, carpetas de corridas, `data_lock` con filelock) y `core/jobs.py` (`JobRegistry` en `data/_jobs/<modulo>.jsonl`, `track()` como context manager, detección de corridas colgadas > 6 h).
- [x] CLI `python -m fog_radar.jobs [--list | <modulo> --trigger cron]` (los pipelines se registran al portar cada módulo).
- [x] Routers de los 4 módulos montados (`/api/cl`, `/api/us`, `/api/etf`, `/api/tendencia`), cada uno con `GET /status`; sistema: `/api/health`, `/api/status`, `/api/jobs`, `/api/jobs/{modulo}`.
- [x] Ruff (lint + formato) + **33 tests pytest** ✅ + GitHub Actions (`.github/workflows/ci.yml`, job backend con `uv sync --frozen`).
- [x] Smoke test real con uvicorn: health 200, sin key 401, con key 200, `/docs` 200, rate limit 429 al superar el límite.
- ⏭️ Movido a Fase 8: probar descargas de Yahoo desde el servidor productivo (requiere tener el servidor).

### Fase 2 — Esqueleto del frontend ✅ completada 2026-09-27
- [x] SvelteKit 2.63 + Svelte 5.56 (runes forzado) + TypeScript + Tailwind 4.3 + **adapter-node**, con ESLint, Prettier, Vitest (Node + navegador) y Playwright (`sv create` 0.17). La configuración de SvelteKit vive en `vite.config.ts` (formato nuevo, sin `svelte.config.js`).
- [x] **Cliente BFF** `src/lib/server/api.ts`: `backend(event)` tipado por ruta con `openapi-fetch` + `call()` / `tryCall()` (timeout 15 s, errores `{message, code}`, 401 del backend → 502 "el sitio no pudo autenticarse"). Envía `X-API-Key` y `X-Forwarded-For` con la IP del usuario final.
- [x] Tipos generados desde el OpenAPI de FastAPI (`npm run gen:api` → `src/lib/api/schema.d.ts`).
- [x] `hooks.server.ts` con cabeceras de seguridad + **CSP con nonce** (el script anti-flash del tema lleva nonce). HSTS queda para Caddy (Fase 8).
- [x] Remote functions evaluadas → **no por ahora** (siguen experimentales). Decisión en `docs/ADR/0001-bff-load-functions-vs-remote-functions.md`; se revisa en la Fase 4.
- [x] Tokens `brand-*`/`viz-*` claro/oscuro en `src/app.css` + puente a las variables semánticas de shadcn; tema persistente sin destello.
- [x] shadcn-svelte (v1.0.12, adaptado al diseño): button, card, table, badge, input, alert, skeleton, tabs, separator, tooltip, label, select, chart. *El registro de shadcn-svelte.com no es accesible desde este entorno; los componentes se tomaron del repositorio en GitHub.*
- [x] Componentes compartidos: `DataTable` (orden por clic en todas las columnas, búsqueda sin tildes, CSV con `;` + BOM para Excel), `KpiCard`, `KpiGrid`, `Section`, `SectionHeader`, `ScoreBar`, `WeightBar`, `CompositionBar`, `SignalBadge`, `TickerLink`, `Notice`, `EmptyState`, `ErrorState`, `ThemeToggle`, `ModuleCard`, `ModuleStatusPanel`.
- [x] Gráficos LayerChart 2.5: `LineChart`, `BarChart`, `ScatterChart`, `Sparkline`, `ChartEmpty` (paleta categórica del sistema, ejes en `viz-axis`, sin redibujar al cambiar de tema).
- [x] Rutas: `/` (hub con datos reales de `/api/status`), `/chile`, `/eeuu`, `/etf`, `/tendencia` (estado de migración), `/estado` (backend + jobs), `/componentes` (vitrina, **solo en dev**), página de error.
- [x] `iniciar_dev.py` + `iniciar.bat` en la raíz (back + front con salida combinada; instala dependencias faltantes la primera vez; Windows y Linux).
- [x] **`npm run autofix`**: el autofixer oficial de Svelte (paquete `@sveltejs/mcp`) sobre todo `src/`, sin red. 31/31 archivos sin observaciones.
- [x] CI: job `frontend` (autofix, check, lint, tests, e2e con backend levantado). `.mcp.json` con el Svelte MCP para Claude Code.
- [x] Verificación: svelte-check 0 errores · ESLint/Prettier OK · **16 tests Vitest** (13 lógica + 3 de componente en Chromium) · **4 E2E Playwright** · build de producción · capturas claro/oscuro sin errores de consola (`docs/screenshots/`).
- Nota Windows: `node_modules` no se versiona; en cada PC correr `npm install` en `frontend/` (Node ≥ 22).

### Fase 3 — Módulo Chile / IPSA (piloto) ✅ completada 2026-09-27
- [x] Backend `modules/cl_stocks/`: `extractor.py` (port de `pyYahooStockAnalisis.py`, funciones testeables con dobles de `yf.Ticker`), `scoring.py` (port fiel de `scoring_acciones.py`), `storage.py` (último CSV, validación de columnas, escritura atómica `.tmp` → rename), `columnas.py` (catálogo de 61 indicadores: clave, etiqueta, grupo, unidad), `schemas.py` + `service.py` + `router.py`.
- [x] Endpoints `GET /api/cl/ranking` (incluye `modelo`: pilares, pesos por defecto y métricas) y `GET /api/cl/indicadores` (columnas con unidad para formatear en el front). Sin datos → 503 `no_data`; CSV corrupto o con columnas faltantes → 503 `invalid_data`.
- [x] `core/fx.py` (`FxRates`, caché por par, fuente inyectable) — adelantado de la Fase 7; lo reutilizará Tendencia.
- [x] Pipeline `python -m fog_radar.jobs cl_stocks [--solo-scoring]`. El CLI ahora acepta opciones por módulo y guarda el resumen del pipeline en el registro de jobs. `--solo-scoring` conserva el timestamp de los indicadores de origen.
- [x] Datos del original copiados a `data/cl_stocks/` (11 CSV). Mismo formato de archivo que el original.
- [x] Frontend: `/chile` (KPIs, panel de pesos con recálculo en el navegador, top 10, ranking con filtro de sector, búsqueda, CSV y detalle desplegable por acción, metodología) y `/chile/indicadores` (9 grupos de columnas + "Todos"). Sub-navegación Ranking · Indicadores. Dashboard ejecutivo aplicado.
- [x] `DataTable` ganó filas desplegables (`detail` + `rowKey`), reutilizable en EEUU/ETF.
- [x] Tests: **44 pytest nuevos** (77 en total): scoring (winsorización, z-score, duplicados, dividendo sostenible, múltiplos ≤ 0, empates), FX, extractor (conversión de moneda, yields, retornos, pausas), API, pipeline y CLI. **13 Vitest nuevos + 1 de componente** (30 en total) y **3 E2E** de Chile (7 en total).
- [x] **Paridad verificada** (`docs/PARIDAD.md`): con los mismos indicadores, el `ranking_acciones_*.csv` generado es **byte a byte idéntico** al del original (2 corridas: 24-09 y 25-09). El recálculo en TS reproduce el orden y el puesto del backend.
- ⚠️ Pendiente del usuario: correr una vez el pipeline completo contra Yahoo en Windows (`uv run python -m fog_radar.jobs cl_stocks`, ~10 min). El entorno de esta sesión no tenía acceso a Yahoo.

#### 👉 Revisión de arquitectura (cierre de Fase 3) — convenciones para las Fases 4-6
Detalle y motivos en `docs/ADR/0002-convenciones-de-modulos.md`. Resumen:
1. **Estructura de módulo backend:** `scoring.py` o subpaquete (lógica pura, nombres del original) · `storage.py` (archivos) · `schemas.py` (Pydantic) · `service.py` (archivo → schema) · `router.py` (solo HTTP) · `pipeline.py` (batch). Fixtures del original en `tests/<modulo>/fixtures/` + `test_parity.py`.
2. **Formato de datos en disco = el del original** (se copian los históricos sin convertir). **Claves de la API = snake_case estables**; el catálogo de columnas (clave, etiqueta, grupo, unidad) vive en el backend.
3. **Parámetros del modelo los entrega la API** (p. ej. pesos por defecto): el front no los duplica.
4. **Zona horaria explícita:** `Settings.timezone` (America/Santiago) para timestamps en nombres de archivo; la API devuelve fechas con offset; `fmtDate` formatea siempre en esa zona (SSR = navegador).
5. **Pipelines:** `Pipeline(run, options)` en `fog_radar.jobs`; `run()` devuelve un resumen que queda en `/api/jobs`. Escritura atómica de archivos de datos.
6. **Front por módulo:** `src/lib/modules/<modulo>/` (lógica TS + componentes propios); lo reutilizable sube a `components/shared`. Tipos solo desde `schema.d.ts` (alias en `api/types.ts` con prefijo de módulo: `Cl…`, `Us…`).
7. **Errores de datos:** `no_data` (aún no hay corrida) e `invalid_data` (archivo corrupto) → 503; las páginas muestran `ErrorState` sin tumbar el sitio (`tryCall`).
8. **E2E con datos:** CI copia los fixtures del módulo a `data/<modulo>/` antes de levantar el backend.

### Fase 4 — Módulo EEUU ✅ completada 2026-09-27
- [x] Backend `modules/us_stocks/`: `universe`, `indicators`, `fundamentals`, `scoring`, `portfolio`, `ticker_eval`, `scanner` portados **sin cambios de lógica** (solo imports, logger y `pd.Timedelta(n, unit="D")`, que elimina el `DeprecationWarning` de NumPy anotado en la Fase 0). Nuevos: `storage.py`, `schemas.py` (Pydantic completo, bloques vacíos → nulos), `router.py`, `pipeline.py`.
- [x] Endpoints `/api/us/`: `rankings/latest[/candidates|/prices?days=&tickers=]`, `rankings/runs`, `tickers/{t}/evaluate`, `portfolio/optimize` (modelo único), `portfolio/limits`.
- [x] Seguridad/robustez: ticker validado en path y body (US-SEC-02), caché TTL de 5 min en `/evaluate` (US-BACK-01), rate limit por endpoint (`/evaluate` 20/min, optimizar 30/min, mejor combinación 6/min), errores `no_data` 503 · `portfolio_data` 400 · `ticker_not_found` 404 · `upstream_error` 502.
- [x] Pipeline `python -m fog_radar.jobs us_stocks` (7 pasos; si la corrida termina con `status: error`, el job queda en error). Las 12 corridas del original copiadas a `data/us_stocks/runs/`.
- [x] Frontend: `/eeuu` (Top 15 + datos de la corrida + buscador), `/eeuu/candidatos`, `/eeuu/evaluacion`, `/eeuu/rendimiento` (filtro "crecientes" en la URL), `/eeuu/portafolio` (markowitz · black-litterman · hrp: universo, optimizar, frontera eficiente, tabla π/Q/posterior de BL, mejor combinación) y `/eeuu/ticker/[ticker]` (evaluación en vivo). Tendencia 3 m con `Sparkline` (LayerChart), CSV con los 41 campos.
- [x] Mutaciones con **form actions** + `use:enhance` (un `<form>` con dos `formaction`: funciona sin JavaScript y conserva selección y resultados). Remote functions revisadas: siguen experimentales → se mantiene ADR 0001.
- [x] Tests: los **110 tests originales** migrados (`tests/us_stocks/`) + 8 de paridad + 34 de API/pipeline (**227 pytest** en total, sin warnings). Front: 8 Vitest nuevos (**38**), **6 E2E** de EEUU (**13** en total).
- [x] **Paridad verificada** con la corrida real del 25-09 (`docs/PARIDAD.md`): score idéntico en los 60 candidatos, momentum y retornos recalculados desde `prices.json` iguales, pesos de BL iguales a los del `portfolio.py` original.
- ⚠️ Pendiente del usuario: correr una corrida completa contra Yahoo en Windows (`uv run python -m fog_radar.jobs us_stocks`, ~12 min) y probar `/eeuu/ticker/AAPL` (aquí no hubo acceso a Yahoo).

### Fase 5 — Módulo ETF ✅ completada 2026-09-28
- [x] Backend `modules/etf/`: `analysis/` (ranking, momentum, categorías, Top 15 Cap., rotación, archivado) portado **sin cambios de lógica** (solo imports relativos, logger y sin los `main()` de CLI); `screener/` (el `Screener_Data_ETF_Script`) como paquete **dentro del mismo proceso** (el subproceso existía por el choque de nombres `config.py`); rutas por parámetro. Nuevos: `storage.py` (staging + lock `filelock`, 503 `busy`), `schemas.py` (claves snake_case, prefijo `Etf`), `service.py`, `router.py`, `pipeline.py`, `informe_pdf.py`.
- [x] Datos en `data/etf/{current,hist,screener}` (= `backend/data`, `data_hist` y `src/` del screener). Históricos copiados sin convertir.
- [x] 14 endpoints `/api/etf/*` (el `GET /api/status` del original es `/api/etf/resumen`, con los parámetros del modelo). `POST /api/etf/run` re-analiza el CSV vigente en *staging*, queda en `/api/jobs` (`trigger: api`), 409 si ya hay una corrida, rate limit 3/min.
- [x] Pipeline `python -m fog_radar.jobs etf [--solo-analisis]`: screener → análisis en staging → archivo con secuencia `_2` → instalación → respaldo de salidas crudas > 30 días.
- [x] **`core/pdf/`**: paleta, fuentes (Calibri → Carlito incluida → Helvetica), tablas y encabezado/pie comunes; `GET /api/etf/pdf` lo usa. Descarga por BFF (`/etf/descargar/informe.pdf`).
- [x] Frontend: 13 secciones con URL propia (`/etf`, `/etf/top15`, `/universo`, `/excluidos`, `/rotacion`, `/diversificacion`, `/categorias`, `/top15-simple`, `/top15-vanguard`, `/momentum`, `/top15-cap`, `/historial`, `/descargar`). `EtfTable` (celdas comunes), `Rango52` (ex `Rango52SemanasBar`), gráfico de tendencia del Top 5 con eje categórico en `LineChart` (ex `TendenciaChart`), `DataTable` con paginación (`pageSize`), `BarChart` con `labelWidth`.
- [x] Tests: **59 pytest** (paridad, 8 tests originales migrados, API, pipeline, screener) · **12 Vitest** (port de los tests del original) · **5 E2E**.
- [x] **Paridad verificada** (`docs/PARIDAD.md`): los 10 CSV de salida **byte a byte idénticos** al original; rotación igual al `rotacion.py` original.
- ⚠️ Pendiente del usuario: correr el screener completo contra Yahoo en Windows (`uv run python -m fog_radar.jobs etf`, ~30 min).
- Nota: con 27+ rutas TypeScript ya no resuelve `resolve()` sobre una unión de rutas; los menús usan `resolve(ruta(href))` (`lib/nav-href.ts`).

### Fase 6 — Módulo Tendencia ✅ completada 2026-09-28
- [x] Backend `modules/tendencia/`: el paquete `stock_tendencia` completo (universe, ingestion, modeling, net/robust_yf, reporting, storage) portado **sin cambios de lógica**; `settings.py` → `config.py` con raíz en `data/tendencia` (`paths.py`, sobrevive al `importlib.reload` del pipeline). `fetch_macro_data.py` → `macro.py` (FRED_API_KEY desde `Settings`); `backfill_history.py` → `backfill.py`. `run_files.py` = el "storage" de ADR 0002 (el nombre `storage/` ya lo usa el paquete SQLite).
- [x] Datos en `data/tendencia/` (raw, processed, macro, universe, output, logs, `history.db`); las 6 corridas y la base del original copiadas.
- [x] API `/api/tendencia/*`: `latest` y `runs/{run_dir}/dashboard` (**reemplaza el JSON estático en `public/`**), `runs/{run_dir}/files[/{nombre}]` (**reemplaza el servidor estático en :8000**), `excluidos`, y los endpoints históricos del original (`analyses`, `runs`, `runs/{run_dir}`, `history/*`) con el mismo SQL, en solo lectura.
- [x] Pipeline `python -m fog_radar.jobs tendencia [--skip-fetch] [--force-refresh] [--force-discovery] [--static-universe] [--macro] [--backfill]`; valida la configuración antes de correr (TD-MODEL-3) y falla si la corrida no deja `dashboard_data.json`.
- [x] Frontend: resumen, portafolio (5 / 10 / 15 en `?n=`), tendencia 6 m (`/tendencia/proyeccion`), sectores, momentum mensual y semanal, limitaciones (con tickers excluidos), descargar (PDF y CSV de la corrida por BFF) e historial (filtros en la URL, gráficos por corrida, trayectoria de un ticker). `?analisis=us` como parámetro de URL que la sub-navegación conserva (`keepParams`).
- [x] Tests: los **73 tests originales** migrados + 24 de paridad + 11 de integración (**395 pytest** en total) · 8 Vitest (**58**) · 5 E2E (**23**). Ruff aplicado al código portado.
- [x] **Paridad verificada** (`docs/PARIDAD.md`): las 23 salidas de la corrida 20260926_01 y el `dashboard_data.json` iguales al original (rtol 1e-7).
- [x] `scripts/fixtures_e2e.py`: instala en `data/` los fixtures de los 4 módulos (+ `history.db` por backfill) para CI o una demo local.
- ⚠️ Pendiente del usuario: correr el pipeline completo contra Yahoo/FRED en Windows (`uv run python -m fog_radar.jobs tendencia --macro`, ~10 min).

### Fase 7 — Consolidación y valor agregado ✅ completada 2026-09-29
- [x] **Cliente común de Yahoo** (`core/yahoo.py`): yfinance (1.x) guarda una sesión *singleton*; `yahoo.instalar()` la reemplaza por una sesión curl_cffi que aplica a **todo el proceso** (los 4 módulos e hilos) un espaciado mínimo global entre requests (0,35 s + jitter), reintento con backoff exponencial ante HTTP 429 (3) y reintentos de red (2), y cuenta requests/429/esperas (quedan en el mensaje del job). Se instala en cada módulo del runner y al arrancar la API; dentro de un request web (`yahoo.interactivo()`) no hay backoff largo. La caché de zonas horarias de yfinance vive en `data/_cache/yfinance`. Parámetros `YAHOO_*` en `.env`. Las pausas propias de cada módulo se mantienen (paridad).
- [x] **Caché en disco de históricos** (ex `tendencia/net/robust_yf.py`) generalizada: `yahoo.descargar_historial` / `descargar_varios` (escritura atómica); `robust_yf` queda como adaptador con los parámetros de Tendencia.
- [x] **FX común** (`core/fx.py`, cierra TD-DATA-1): `par_usd`, `serie_a_usd` (forward-fill sin mirar al futuro) y cualquier moneda ISO 4217 + subunidades de Yahoo (GBp, ZAc, ILA). Tendencia (`fetch_fx`, `build_dataset`, `discovery`) ya no depende de una tabla de 6 monedas (`config.fx_ticker_for`).
- [x] **PDF de Tendencia sobre `core/pdf`**: paleta, fuentes (Calibri → Carlito incluida → Helvetica, con cursiva) y matplotlib a juego desde `core/pdf`; `reporting/theme.py` queda como alias. Además, por las preferencias de formato: encabezados de tabla claros (antes azul oscuro con texto blanco), números alineados a la derecha, textos ≥ 10 pt en gráficos y tablas (alto del gráfico según la cantidad de barras). También el PDF de ETF (categoría 9 → 10 pt).
- [x] "Stats" comunes: **se descarta a propósito**. Los cálculos de Sharpe/anualización de cada módulo difieren en frecuencia (diaria/semanal), convención de rf y librería (pypfopt vs. propio) y están cubiertos por los tests de paridad; unificarlos cambiaría resultados sin beneficio.
- [x] **TD-CFG-1 — opciones con valor en el CLI de jobs**: `PipelineOption(tipo=int|float, minimo, maximo)`; Tendencia acepta `--top-n N`, `--top-n-us N`, `--max-weight X` (validadas por argparse y por `validar_configuracion()`), valen solo para esa corrida (`paths.set_overrides`, sobreviven al `reload` de `config`) y quedan en `run_manifest.json` (`parametros` y `parametros_sobrescritos`).
- [x] **Corridas batch en secuencia** (✅ 2026-09-28): `fog_radar/jobs/runner.py` (lock global entre procesos, pausa entre pipelines, reintento con espera, la secuencia sigue si un módulo falla, aviso por `ALERT_WEBHOOK_URL` — US-DATA-04 y US-DEVOPS-04) y `fog_radar/jobs/schedule.py` (única fuente de la programación). CLI: `--todos [--solo …] [--offline]`, `--programa semanal|diario`, `--esperar`, `--crontab`, `--list`.
  - **Producción (Linux):** ETFs los **lunes 06:00**; Chile → EEUU → Tendencia de **lunes a viernes 18:00**, en secuencia. `deploy/backend/crontab` (generado y verificado por test) + `deploy/backend/README.md`.
  - **Desarrollo (Windows):** `python scripts/run_all_pipelines.py` o doble clic en `correr_pipelines.bat`: corre los 4 a pedido (Chile → EEUU → Tendencia → ETF), con confirmación, `--solo`, `--offline` (sin Yahoo) y log en `data/_jobs/logs/`.
  - Tendencia ganó `--skip-discovery` (reutiliza el último universo) para el modo offline.
  - `/estado` muestra las corridas programadas y la próxima ejecución (`GET /api/jobs/programas`).
- [x] **Hub con datos reales** (`GET /api/hub`): por módulo, fecha de los datos, antigüedad, **frescura** ("al día" si los datos son posteriores a la última corrida programada, con 3 h de margen para que termine; si no, "atrasado"), top 3 con su métrica, tamaño del universo, programa y próxima corrida, último job. Un módulo con datos ilegibles no tumba el hub (`frescura: error`). Cada módulo aporta `modules/<m>/hub.py`.
- [x] **Buscador global de ticker** (`GET /api/buscar?q=`, rate limit 120/min): índice de la última corrida de cada módulo (Chile: ranking IPSA; EEUU: candidatos con marca Top 15; ETF: universo depurado con marca Top 15; Tendencia: portafolios y líderes de momentum), coincidencia exacta de ticker (con o sin sufijo de bolsa), prefijo o nombre (sin tildes), caché de 60 s. En el sitio: caja en el menú, sección en el hub y página `/buscar`; cada resultado abre la vista del módulo **ya filtrada** (`?q=` en Chile, EEUU candidatos y ETF universo; `?ticker=` en el historial de Tendencia).
- [x] **Revisión de calidad** (TI + cuantitativa, 2026-09-29): 3 correcciones de cálculo (Black-Litterman con rf, mediana GBM de Tendencia, beta de ETF por fecha — `docs/PARIDAD.md`), 11 de seguridad/robustez y 9 del frontend (ver `docs/BACKLOG.md`, "Revisión de la Fase 7"). Lo que cambia la metodología quedó en el backlog (TD-MODEL-4…9, US-DATA-08) para decidirlo con backtest.
- [ ] Opcionales (no bloquean): comparador EEUU vs. Tendencia, informe PDF consolidado semanal de los 4 módulos.
- **Listo cuando:** un solo comando actualiza todo y el hub refleja el estado real. ✅

### Fase 8 — Calidad, despliegue en 2 servidores y cierre
**Parte de desarrollo** (✅ 2026-09-29, salvo lo que depende del usuario):
- [x] E2E Playwright: prueba de humo por módulo + hub + buscador (**26 E2E**).
- [x] Local: `iniciar_dev.py` / `iniciar.bat` levantan los 2 procesos (verificado).
- [x] **Historial SQLite en los 4 módulos** (Épica B1, ADR 0003, 2026-10-04): `core/histdb.py` (WAL, `user_version`, diff común); `history.db` en Chile, EEUU y ETF (escritura al final del pipeline + `--backfill`), Tendencia migrada a v2 (pesos del universo completo). Endpoints `/api/<mod>/history/*` y vistas `/chile/historial`, `/eeuu/historial`, `/etf/evolucion` y la posición en el universo en `/tendencia/historial`.
- [x] Documentación: `README.md`, `docs/METODOLOGIAS.md` (metodología de los 4 módulos, escrita desde el código actual) y `CLAUDE.md` (convenciones nuevas: cliente Yahoo, FX, límites de concurrencia, `hub.py`).
- [ ] ⚠️ Usuario: correr los 4 pipelines completos contra Yahoo en Windows (`python scripts/run_all_pipelines.py`) y revisar el sitio con datos reales.
- [ ] ⚠️ Usuario: verificación final de paridad con los 4 originales → archivar los proyectos antiguos.

**Parte de producción** — ⏸ **en pausa por decisión del usuario** (no se hace hasta tener el desarrollo depurado):
- [ ] **Servidor backend**: Docker (uvicorn con varios workers) + volumen persistente para `data/` + cron de pipelines + Caddy con HTTPS; firewall que solo acepte el puerto 443 desde la IP del servidor frontend (y SSH).
- [ ] **Servidor frontend**: Docker (`node build`) + Caddy con HTTPS en el dominio público; variables `ORIGIN`, `PRIVATE_API_URL`, `PRIVATE_API_KEY`, `ADDRESS_HEADER`/`XFF_DEPTH` (SEC-PROD-1).
- [ ] Dominios/DNS (ej. `quant.midominio.cl` y `api.midominio.cl`), rotación de la API key, respaldo diario de `data/` (incluye `history.db`).
- [ ] Autenticación de usuarios en el frontend si el sitio no será público (p. ej. login simple con sesión en cookie, manejado en `hooks.server.ts`).
- [ ] CI/CD: GitHub Actions construye y publica imágenes separadas (`fog-radar-api`, `fog-radar-web`) y despliega cada servidor de forma independiente.
- [ ] Monitoreo básico: `/api/health` + uptime check; logs de pipelines accesibles desde `/estado`; mensajes de error genéricos en prod (SEC-PROD-2).
- [ ] **TD-MACRO-1**: agregar `--macro` a Tendencia en la corrida diaria (programa `diario` en `jobs/schedule.py` → regenerar `deploy/backend/crontab`). Requiere `FRED_API_KEY` en el servidor; suma unos segundos de descarga FRED.
- **Listo cuando:** los 4 proyectos originales ya no son necesarios para el uso diario.

---

## 4. Cronograma estimado

| Fase | Contenido | Sesiones aprox. | Acumulado |
|---|---|---|---|
| 0 | Preparación | 1 | 1 |
| 1 | Esqueleto backend | 1 | 2 |
| 2 | Esqueleto frontend | 1–2 | 4 |
| 3 | Chile (piloto) ✅ | 1–2 | 6 |
| 4 | EEUU ✅ | 2–3 | 9 |
| 5 | ETF ✅ | 3 | 12 |
| 6 | Tendencia ✅ | 3 | 15 |
| 7 | Consolidación ✅ | 2–3 | 18 |
| 8 | Calidad, despliegue en 2 servidores y cierre | 2–3 | 21 |

*"Sesión" = un bloque de trabajo enfocado de unas horas. Hitos visibles: fin de Fase 3 (primer módulo completo en el sitio nuevo) y fin de Fase 6 (paridad total).*

---

## 5. Riesgos y mitigaciones

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Conflictos de versiones Python (numpy/pandas/yfinance/PyPortfolioOpt) | Alto | Resolver en Fase 0 con uv; fijar techos como ya hace ETF. |
| Bloqueos de Yahoo al correr 4 pipelines seguidos | Alto | Cliente yfinance común con rate-limit global y caché; escalonar horarios de los jobs. |
| Diferencias numéricas al portar (scoring, Markowitz) | Alto | Tests de paridad: mismo input → mismo output que el original, antes de dar un módulo por terminado. |
| Mezcla de sintaxis Svelte 4/5 en código generado por IA | Medio → Bajo | Svelte MCP + `svelte-autofixer` obligatorio; `sv check` en CI. |
| Reescritura de ~70 componentes React/Vue a Svelte | Medio | Componentes compartidos primero (Fase 2); la mayoría de pestañas son "tabla + KPIs + gráfico". |
| Yahoo Finance bloquea más seguido las IPs de datacenter | Alto | Probar los pipelines desde el servidor backend apenas exista (Fase 8, o antes si se contrata el servidor); si hay bloqueos: `curl_cffi` (ya usado en Tendencia), pausas más largas, o correr los pipelines en local y sincronizar `data/` al servidor (rsync). |
| Backend expuesto en internet | Alto | API key + allowlist de IP + HTTPS; endpoints costosos (`/run`, portafolio, evaluación) con rate-limit. |
| Latencia extra por el salto front → back (SSR) | Bajo | Ambos servidores en la misma región/proveedor; caché corta en el backend para "última corrida". |
| Costo y mantención de 2 servidores | Bajo-Medio | VPS pequeños (el front casi no consume); imágenes Docker idénticas en dev y producción. |
| LayerChart en evolución (API cambió entre versiones mayores) | Bajo-Medio | Fijar la versión en `package.json` y encapsular en `lib/charts/` para que un cambio solo toque esos wrappers. |
| Endpoints que devuelven dicts sin esquema | Medio | Agregar modelos Pydantic al migrar cada router; sin esquema no hay tipos TS. |
| Acople Tendencia → `frontend/public/data` | Medio | Reemplazo por endpoint en Fase 6. |
| Secretos en `.env` | Medio | `.env.example` + `.gitignore` desde la Fase 0. |
| Alcance creciente por el backlog de mejoras | Medio | Triage en Fase 0; lo "después" no entra hasta cerrar la Fase 6. |

---

## 6. Convenciones

- Idioma de la UI: español. Código: nombres en inglés para módulos/paquetes, se respetan nombres de dominio existentes (p. ej. `rotacion`, `excluidos`).
- Commits por fase y por módulo (`feat(cl): ...`, `feat(us): ...`, `chore(core): ...`).
- Cada módulo terminado = checklist de paridad firmado en `docs/PARIDAD.md`.
- Ningún pipeline se ejecuta dentro de un request web (salvo la evaluación de un solo ticker en EEUU).
- Todo color proviene de un token `brand-*`; nada de colores sueltos en componentes.

---

## 7. Próximo paso inmediato

1. ~~Orden de módulos~~ ✅ confirmado (Chile → EEUU → ETF → Tendencia) · ~~Librería de gráficos~~ ✅ LayerChart · ~~Modo de despliegue~~ ✅ SvelteKit con `adapter-node` (SSR) + FastAPI en **2 servidores separados**, patrón BFF.
2. Pendiente de definir más adelante (no bloquea): proveedor de hosting, dominio, y si el sitio tendrá login o será público. Repositorio: **privado** ✅.
3. ~~Fase 0~~ ✅ · ~~Fase 1~~ ✅ · ~~Fase 2~~ ✅ · ~~Fase 3~~ ✅ Chile · ~~Fase 4~~ ✅ EEUU · ~~Fase 5~~ ✅ ETF · ~~Fase 6~~ ✅ Tendencia (**paridad total de los 4 módulos**) · ~~Fase 7~~ ✅ consolidación · Fase 8: parte de desarrollo ✅.
4. **Siguiente:** el usuario corre los 4 pipelines contra Yahoo en Windows (`python scripts/run_all_pipelines.py`) y depura con datos reales. La **puesta en producción queda en pausa** hasta que el desarrollo esté depurado (Fase 8, parte de producción, incluye TD-MACRO-1).
5. Después (B, con backtest): TD-MODEL-4…9 y US-DATA-08 (`docs/BACKLOG.md`).
