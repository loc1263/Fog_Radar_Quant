# Fog_Radar_Quant — Mapa de ruta

> Objetivo: **un solo sitio web** de análisis cuantitativo con **SvelteKit + TypeScript + Tailwind** (frontend)
> y **Go** (backend: API + pipelines batch).

---

## 1. Qué es

Unifica cuatro proyectos (`ETF_Choice`, `Stock_Choice_EEUU`, `Stock_Choice_Nacional`, `Stock_Tendencia`) en un solo
sitio con un hub, un buscador global de tickers y cuatro módulos: ETFs, acciones EEUU, acciones Chile (IPSA) y
Tendencia/Markowitz Asia + EEUU. Los proyectos originales viven en `D:\Claude\<Proyecto>` y no se modifican.

## 2. Arquitectura

### 2.1 Estructura del monorepo

```
Fog_Radar_Quant/
├── backend-go/                     # API + pipelines (un binario: fogradar api | job | openapi)
│   ├── cmd/fogradar/               # main, CLI de jobs, log único
│   └── internal/{api,core,modules/{cl,us,etf,tend},yahoo,fx,quant,jobs,transversal}
├── data/                           # fuera del código, ignorado por git
│   ├── etf/{current,hist,screener}/   us_stocks/runs/   cl_stocks/   tendencia/{output,universe,...}
│   └── _jobs/                      # registro de corridas (jsonl) y log único (logs/fogradar.log)
├── frontend/                       # SvelteKit + TS + Tailwind v4 · adapter-static (SPA)
├── docs/                           # DESIGN_SYSTEM · METODOLOGIAS · BACKLOG · ADR/
├── iniciar.bat → iniciar.py · correr_pipelines.bat
└── .github/workflows/ci.yml
```

### 2.2 Decisiones técnicas

| Tema | Decisión | Motivo |
|---|---|---|
| Backend | **Go**: huma sobre chi, sqlite sin CGO, `gohoo` para Yahoo, gonum; rutas `/api/<modulo>` | Un binario estático, pipelines rápidos, despliegue simple (ADR 0004). |
| Contratos API | Tipos de respuesta en el backend + tipos TS generados con `openapi-typescript` (`npm run gen:api`) | Una sola fuente de verdad; el front no se desincroniza del back. |
| Frontend | **SvelteKit 2 + Svelte 5 (runes)** con **`adapter-static`** (SPA) | Se publica como sitio estático (ADR 0005, `docs/PLAN_GITHUB_PAGES.md`). |
| Comunicación front ↔ back | `fogradar export` genera JSON estático y el sitio lo lee (`$lib/datos`) | No hay servidor ni API key en el sitio (ADR 0005). |
| Seguridad del backend | Header `X-API-Key` + allowlist de IP del servidor frontend + HTTPS | Protege los endpoints costosos (consumen Yahoo/CPU). |
| Configuración | Front: `BASE_PATH` (subruta de Pages) · Back: `API_KEY`, `ALLOWED_ORIGINS`, `DATA_DIR`… | Mismo código en dev y producción. |
| Dev | `iniciar.bat`: exporta los datos y levanta `vite dev` (:5173); `--api` agrega `fogradar api` (:8000) | El sitio no depende de la API en ejecución. |
| Publicación | GitHub Pages (sitio estático) con un workflow de Actions | Sin servidores propios (ADR 0005). |
| UI | **Tailwind v4** + **shadcn-svelte** + tokens del *Dashboard ejecutivo* (`docs/DESIGN_SYSTEM.md`) | Continuidad visual. |
| Gráficos | **LayerChart 2.5** | Librería nativa de Svelte, estilable con tokens `brand-*`. |
| Tablas | `DataTable` genérico propio (orden, filtro, CSV) | Casi todas las pestañas son tablas. |
| Pipelines | **Batch fuera del proceso web** (`fogradar job`). Producción: cron en el servidor backend; la API solo lee | Evita bloqueos de Yahoo en requests web. |
| Estado de corridas | Registro en `data/_jobs/<modulo>.jsonl` + `/api/jobs` + log único `fogradar.log` | El hub muestra "al día / atrasado". |
| Historial | Retirado (2026-10-21): sin vistas ni `history.db` (ver ADR 0005) | El sitio estático no lo necesita. |
| Calidad | `gofmt`, `go vet`, `golangci-lint`, `go test -race` (back); `svelte-check` + ESLint + Prettier + Vitest + Playwright (front); CI en GitHub | |

### 2.3 Mapa de rutas del sitio

| Ruta SvelteKit | Contenido |
|---|---|
| `/` | **Hub**: 4 tarjetas (una por módulo) con fecha de última corrida, KPI principal y top 3; accesos directos |
| `/etf` → `/etf/resumen` | Resumen de la corrida |
| `/etf/[seccion]` | top15 · universo · excluidos · rotacion · diversificacion · categorias · top15-simple · top15-vanguard · momentum · top15-cap |
| `/eeuu` | Top 15 momentum + datos de la corrida |
| `/eeuu/candidatos` · `/eeuu/evaluacion` · `/eeuu/rendimiento` | Vistas del screener |
| `/eeuu/portafolio` | Portafolios precalculados (modelo único: clusters de correlación + Black-Litterman, `docs/MODELO_NEW.md`) |
| `/chile` | Ranking IPSA |
| `/chile/indicadores` | Tabla de indicadores crudos |
| `/tendencia/[seccion]?analisis=…` | resumen · portafolio · tendencia · sectores · momentum · limitaciones |
| `/estado` | Estado de los módulos y cadencia de actualización (instantánea de la exportación) |

Layout global: header con logo *Fog Radar Quant*, menú principal de 4 módulos, sub-navegación por módulo (tabs como links reales — cada pestaña es una URL compartible), toggle claro/oscuro, footer con disclaimer "no es asesoramiento financiero".

### 2.4 Endpoints

| Módulo | Prefijo |
|---|---|
| Sistema | `/api/health`, `/api/status`, `/api/jobs`, `/api/jobs/programas`, `/api/hub` |
| ETF | `/api/etf/*` (resumen, top15, ranking-completo, excluidos, rotacion, diversificacion, categorias, momentum, russell3000) |
| EEUU | `/api/us/*` (rankings, portfolio/limits, portfolio/precalculado) |
| Chile | `/api/cl/*` (ranking, indicadores) |
| Tendencia | `/api/tendencia/*` (latest, runs/{run_dir}/dashboard, excluidos) |

---

## 3. Estado

| Módulo | Backend | Frontend | Pipeline |
|---|---|---|---|
| Chile (IPSA) | ✅ | ✅ | ✅ `fogradar job cl_stocks` (~40 s) |
| EEUU | ✅ | ✅ | ✅ `fogradar job us_stocks` (~4 min) |
| ETF | ✅ | ✅ | ✅ `fogradar job etf` (~7–13 min; caché de info) |
| Tendencia | ✅ | ✅ | ✅ `fogradar job tendencia` (universo redescubierto si el snapshot tiene > 3 días) |
| Hub y estado | ✅ | ✅ | — |

Cadencia esperada (`internal/core/schedule.go`): ETFs los **lunes 06:00**; Chile → EEUU → Tendencia de **lunes a viernes 18:00**, en
secuencia. En desarrollo: `correr_pipelines.bat` o `fogradar job --todos`.

## 4. Pendiente

**Desarrollo**
- [ ] Opcionales: comparador EEUU vs. Tendencia, informe PDF consolidado semanal de los 4 módulos.
- [ ] Usuario: revisar el sitio con datos reales y archivar los proyectos antiguos.
- [x] Mover el screener de Yahoo a `gohoo` (v0.3.0: `Screen`, `GetJSON`/`PostJSON`); `internal/yahoo/screener.go` queda como adaptador fino.
- [x] `gohoo` v0.4.0: market cap de EEUU con `Quotes` en lote; espaciado, contadores y reintento ante 429 instalados dentro de gohoo
  (`WithLimiter`, `WithRequestHook`, `RetryPolicy.RateLimit`, `WithRetryPolicyContext`); se eliminó `Traffic.Do`.
- [x] `gohoo` v0.5.0: `FlattenInfo`, `TimeseriesPoints` y `AutoAdjust`/`AtMidnight` viven en gohoo;
  `internal/yahoo/{info,fundamentals}.go` y `AutoAdjust` quedan como envoltorios sobre la `Source` (para los falsos de los tests).
- [x] `gohoo` v0.6.0: indicadores sobre `[]Bar` con períodos configurables y métricas de riesgo (`Returns`, `Volatility`, `MaxDrawdown`).
- [ ] Después (con backtest): TD-MODEL-4…9 y US-DATA-08 (`docs/BACKLOG.md`).

**Producción** — ⏸ en pausa hasta tener el desarrollo depurado
- [ ] **Sitio estático**: publicar en GitHub Pages con un workflow de Actions que corra los jobs, `fogradar export` y `npm run build` (`docs/PLAN_GITHUB_PAGES.md`).
- [ ] Dominios/DNS, respaldo diario de `data/`.
- [ ] Monitoreo básico: `/api/health` + uptime check; mensajes de error genéricos en prod (SEC-PROD-2).
- [ ] **TD-MACRO-1**: agregar `--macro` a Tendencia en la corrida diaria (requiere `FRED_API_KEY` en el servidor).
- [ ] Probar las descargas de Yahoo desde el servidor productivo (las IPs de datacenter se bloquean más seguido).

---

## 5. Riesgos y mitigaciones

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Bloqueos de Yahoo al correr varios pipelines seguidos | Alto | Cliente común con espaciado global y backoff ante 429; corridas en secuencia con pausa; caché de info de ETFs. |
| Mezcla de sintaxis Svelte 4/5 en código generado por IA | Medio → Bajo | Svelte MCP + `svelte-autofixer` obligatorio; `svelte-check` en CI. |
| Backend expuesto en internet | Alto | API key + allowlist de IP + HTTPS; endpoints costosos con rate limit y límite de concurrencia. |
| Latencia extra por el salto front → back (SSR) | Bajo | Ambos servidores en la misma región; caché corta en el backend. |
| LayerChart en evolución | Bajo-Medio | Versión fija y wrappers en `lib/charts/`. |
| Secretos en `.env` | Medio | `.env.example` + `.gitignore`. |
| Alcance creciente por el backlog | Medio | Triage en `docs/BACKLOG.md`. |

---

## 6. Convenciones

- Idioma de la UI: español. Código: nombres en inglés para módulos/paquetes, se respetan nombres de dominio existentes (p. ej. `rotacion`, `excluidos`).
- Commits por módulo (`feat(cl): ...`, `feat(us): ...`, `chore(core): ...`).
- Ningún pipeline se ejecuta dentro de un request web (salvo la evaluación de un solo ticker en EEUU).
- Todo color proviene de un token `brand-*`; nada de colores sueltos en componentes.
