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
│   │   │   ├── yf_client.py        # yfinance robusto (reintentos, rate-limit, cache)
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
├── scripts/  iniciar_dev.py · iniciar.bat · run_all_pipelines.py
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
| Gráficos | **LayerChart** (✅ decidido 2026-09-27) | Librería nativa de Svelte (sobre D3/Layer Cake), componible, estilable con clases Tailwind y tokens `brand-*`. Reemplaza Recharts (ETF) y Chart.js (Tendencia). Cubre línea, barras, área, scatter (frontera eficiente) y sparklines. |
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
| `/eeuu` → `/eeuu/top15` | Top 15 momentum + score de salud | Stock_Choice_EEUU |
| `/eeuu/candidatos` · `/eeuu/rendimiento` · `/eeuu/evaluar` · `/eeuu/portafolio` · `/eeuu/mejor-combinacion` | Vistas del screener | Stock_Choice_EEUU |
| `/eeuu/ticker/[ticker]` | Detalle / evaluación en vivo de un ticker | Stock_Choice_EEUU |
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
| EEUU | `/api/rankings/latest[/candidates\|/prices]`, `/api/rankings/runs`, `/api/tickers/{t}/evaluate`, `POST /api/portfolio/optimize`, `POST /api/portfolio/best_combination` | `/api/us/<mismo sufijo>` |
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
- [x] Triage → `docs/BACKLOG.md`: 61 ítems abiertos → **34 A** (durante la migración) · **23 B** (después) · **3 C** (descartar) · **1 decisión pendiente** (licencia, TD-LIC-1).
- [x] `docs/DESIGN_SYSTEM.md` único (base: Stock_Tendencia; adaptado a SvelteKit + LayerChart; módulo de tema Svelte 5 validado con `svelte-autofixer`).
- [x] `backend/pyproject.toml` + `uv.lock` (Python 3.12, 88 paquetes). Resuelve e instala en venv limpio. Versiones clave: fastapi 0.141 · pandas 2.3.3 · numpy 2.5 · scipy 1.18 · yfinance 1.7 · PyPortfolioOpt 1.6 · reportlab 4.5.
- [x] **Compatibilidad verificada** corriendo los tests originales contra el entorno unificado: EEUU 137/137 ✅ · Tendencia 75/75 ✅ · ETF 10/10 ✅ · Chile (sin tests) imports ✅. Las 4 apps FastAPI originales importan sin error.
- Nota: aparecen `DeprecationWarning` de NumPy por `Timedelta` con unidad genérica (EEUU `scanner.py` y pandas `resample`). No rompe hoy; corregir al portar (Fases 4 y 6).

### Fase 1 — Esqueleto del backend (≈1 sesión)
- [ ] `create_app()` con logging, manejo de errores uniforme (`{detail, code}`), `/api/health` (público, sin datos) y resto de rutas protegidas por dependencia `require_api_key`.
- [ ] CORS restringido a `ALLOWED_ORIGINS` (en BFF casi no se usa, pero queda cerrado por defecto).
- [ ] `settings.py` (pydantic-settings) con rutas `data/<modulo>` y claves desde `.env`.
- [ ] `core/runs.py` (buscar última corrida, lock de archivo) y `core/jobs.py` (registro de ejecuciones).
- [ ] Routers vacíos de los 4 módulos montados; OpenAPI visible en `/docs`.
- [ ] Ruff + pytest + CI (job backend).
- **Listo cuando:** `uvicorn fog_radar.main:app` levanta y `/docs` muestra los 4 grupos.

### Fase 2 — Esqueleto del frontend (≈1–2 sesiones)
- [ ] `npx sv create frontend` (SvelteKit, TS estricto, Tailwind v4, ESLint, Prettier, Vitest, Playwright) con **`adapter-node`**.
- [ ] `$lib/server/api.ts`: cliente del backend que solo corre en el servidor (lee `PRIVATE_API_URL` y `PRIVATE_API_KEY`, timeout, reintentos, errores tipados). Convención: toda carga de datos va en `+page.server.ts`; las acciones (POST) en *form actions* o `+server.ts`.
- [ ] Evaluar **Remote functions** de SvelteKit (`query`/`form`/`command` en `.remote.ts`) como alternativa tipada a form actions + `+server.ts` para el BFF.
- [ ] Streaming de promesas en `load` para endpoints lentos (la página aparece de inmediato con skeletons y los datos llegan después).
- [ ] `hooks.server.ts`: headers de seguridad (CSP, HSTS), logging de requests.
- [ ] `app.css` con los tokens `brand-*` claro/oscuro y script anti-flash del tema.
- [ ] shadcn-svelte: Card, Table, Badge, Tabs, Button, Input, Alert, Skeleton.
- [ ] Componentes compartidos: `DataTable` (orden + filtro + CSV), `KpiCard`, `ScoreBar`, `TickerLink`, `SignalBadge`, `Sparkline`, `ErrorState`, `EmptyState`.
- [ ] Componentes de gráfico base con LayerChart (`LineChart`, `BarChart`, `ScatterChart`, `Sparkline`) con tema claro/oscuro desde los tokens.
- [ ] Cliente API tipado: script `npm run gen:api` → `openapi-typescript http://localhost:8000/openapi.json`.
- [ ] Layout global, menú de módulos, página `/` con placeholders.
- [ ] `scripts/iniciar_dev.py` que levanta back + front juntos (como los `start_dev.py` actuales).
- **Listo cuando:** navegación entre los 4 módulos vacía funciona, tema claro/oscuro OK, tipos generados.

### Fase 3 — Módulo Chile / IPSA (piloto, ≈1–2 sesiones)
- Backend: portar `pyYahooStockAnalisis.py` → `extractor.py`, `scoring_acciones.py` → `scoring.py`, `api.py` → `router.py` con schemas Pydantic; pipeline `python -m fog_radar.jobs cl`.
- Frontend: `/chile` con `RankingTable` → `DataTable`, `PanelPesos` con recalculado del score en el cliente (`lib/scoring.ts` se porta tal cual a TS puro), `/chile/indicadores`.
- Aplicar el Dashboard ejecutivo (este módulo hoy no lo usa).
- Tests: agregar pytest para scoring (hoy no tiene) — winsorización, sostenibilidad del dividendo, conversión FX.
- **Listo cuando:** los mismos datos muestran el mismo ranking que el proyecto original (comparación fila a fila).
- 👉 **Revisión de arquitectura**: ajustar convenciones antes de seguir con los otros módulos.

### Fase 4 — Módulo EEUU (≈2–3 sesiones)
- Backend: `scanner.py` (7 pasos), `fundamentals`, `indicators`, `scoring`, `portfolio`, `ticker_eval`, `universe` → `modules/us_stocks/`; mover los 8 archivos de tests.
- Frontend: top15, candidatos (búsqueda), rendimiento, evaluar ticker (en vivo), portafolio (optimización + gráfico de pesos + frontera eficiente), mejor combinación, detalle de ticker; los hooks React (`use-portfolio`, `use-run`…) pasan a `$state` de Svelte; la optimización de portafolio y la evaluación de ticker pasan por *form actions* del servidor SvelteKit (el navegador nunca llama directo a FastAPI).
- **Listo cuando:** tests pytest pasan y las 7 vistas replican el original.

### Fase 5 — Módulo ETF (≈3 sesiones)
- Backend: `analysis/` (ranking, momentum, categorías, rotación, archivado), `Screener_Data_ETF_Script`, `actualizar_datos_semanal.py` → `pipeline.py`; `POST /api/etf/run` pasa a registrar el job en `core/jobs.py`; `/api/etf/pdf` usa el módulo PDF común.
- Frontend: 13 secciones como rutas `/etf/[seccion]`; `TendenciaChart`, `Rango52SemanasBar` → LayerChart/componentes propios; portar los tests Vitest.
- **Listo cuando:** las 13 secciones y la descarga del PDF funcionan igual que en el original.

### Fase 6 — Módulo Tendencia (≈3 sesiones)
- Backend: `step00…step07`, `robust_yf.py` (→ base de `core/yf_client.py`), `fetch_macro_data.py`, `backfill_history.py`, `history.db`; nuevo `/api/tendencia/latest` que reemplaza el JSON en `public/`; los PDFs de `output/` se sirven vía `/api/tendencia/runs/{dir}/files/...` en lugar del servidor estático en :8000.
- Frontend: resumen, portafolio (selector 5/10), tendencia 6m, sectores, momentum (mensual/semanal), limitaciones, historial; selector `?analisis=` (Asia+EEUU / solo EEUU) como parámetro de URL.
- Migrar pytest + Ruff existentes.
- **Listo cuando:** vista "última corrida" e "historial" idénticas al original, sin JSON estático.

### Fase 7 — Consolidación y valor agregado (≈2–3 sesiones)
- [ ] Extraer código repetido a `core/`: cliente yfinance único (rate-limit, reintentos, caché en disco), FX, stats, estilos PDF (Calibri/Carlito + paleta ejecutiva).
- [ ] `scripts/run_all_pipelines.py` + calendario de ejecución (ETF lunes; resto según frecuencia actual): Programador de tareas en local y `crontab` en el servidor backend.
- [ ] Página `/estado` y el **Hub** con datos reales (fecha, frescura, top 3 por módulo).
- [ ] Funciones transversales (opcionales): **buscador global de ticker** (en qué módulos aparece y con qué score), comparador EEUU vs. Tendencia, informe PDF consolidado semanal de los 4 módulos.
- **Listo cuando:** un solo comando actualiza todo y el hub refleja el estado real.

### Fase 8 — Calidad, despliegue en 2 servidores y cierre (≈2–3 sesiones)
- [ ] E2E Playwright: una prueba de humo por módulo (carga, tabla con filas, gráfico renderizado).
- [ ] **Servidor backend**: Docker (uvicorn con varios workers) + volumen persistente para `data/` + cron de pipelines + Caddy con HTTPS; firewall que solo acepte el puerto 443 desde la IP del servidor frontend (y SSH).
- [ ] **Servidor frontend**: Docker (`node build`) + Caddy con HTTPS en el dominio público; variables `ORIGIN`, `PRIVATE_API_URL`, `PRIVATE_API_KEY`.
- [ ] Dominios/DNS (ej. `quant.midominio.cl` y `api.midominio.cl`), rotación de la API key, respaldo diario de `data/` (incluye `history.db`).
- [ ] Autenticación de usuarios en el frontend si el sitio no será público (p. ej. login simple con sesión en cookie, manejado en `hooks.server.ts`).
- [ ] CI/CD: GitHub Actions construye y publica imágenes separadas (`fog-radar-api`, `fog-radar-web`) y despliega cada servidor de forma independiente.
- [ ] Monitoreo básico: `/api/health` + uptime check; logs de pipelines accesibles desde `/estado`.
- [ ] Local: `iniciar_dev.py` / `iniciar.bat` siguen levantando los 2 procesos para desarrollo.
- [ ] Documentación: `README.md`, `docs/METODOLOGIAS.md` (reúne las metodologías de los 4 CLAUDE.md/README), `CLAUDE.md` del monorepo.
- [ ] Verificación final de paridad con los 4 originales → archivar los proyectos antiguos.
- **Listo cuando:** los 4 proyectos originales ya no son necesarios para el uso diario.

---

## 4. Cronograma estimado

| Fase | Contenido | Sesiones aprox. | Acumulado |
|---|---|---|---|
| 0 | Preparación | 1 | 1 |
| 1 | Esqueleto backend | 1 | 2 |
| 2 | Esqueleto frontend | 1–2 | 4 |
| 3 | Chile (piloto) | 1–2 | 6 |
| 4 | EEUU | 2–3 | 9 |
| 5 | ETF | 3 | 12 |
| 6 | Tendencia | 3 | 15 |
| 7 | Consolidación | 2–3 | 18 |
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
| Yahoo Finance bloquea más seguido las IPs de datacenter | Alto | Probar los pipelines desde el servidor backend en la Fase 1; si hay bloqueos: `curl_cffi` (ya usado en Tendencia), pausas más largas, o correr los pipelines en local y sincronizar `data/` al servidor (rsync). |
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
2. Pendiente de definir más adelante (no bloquea la Fase 0): proveedor de hosting, dominio, y si el sitio tendrá login o será público.
3. ~~Fase 0~~ ✅. **Siguiente: Fase 1 — esqueleto del backend.** Pendiente de decidir: TD-LIC-1 (licencia / visibilidad del repo).
