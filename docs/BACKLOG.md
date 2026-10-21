# Backlog unificado — Fog Radar Quant

> Triage hecho el 2026-09-27 (Fase 0) a partir de los 4 archivos de mejoras pendientes:
> `Stock_Choice_Nacional/WEB/mejoras_pendientes.txt` · `Stock_Choice_EEUU/mejoras_pendientes.md` ·
> `ETF_Choice/WEB/mejoras_web_pendientes.txt` · `Stock_Tendencia/backend/mejoras_pendientes.txt`.
> Solo se consideraron ítems **abiertos** (los resueltos quedan en los históricos de cada proyecto).

**Etiquetas**

- **A — Durante la migración:** se resuelve en la fase indicada, casi siempre porque la nueva arquitectura ya lo exige (seguridad, CI, Docker, componentes comunes).
- **B — Después:** mejoras de producto o de modelo. No entran hasta cerrar la Fase 6.
- ✅ en la columna Fase = resuelto.
- **C — Descartar / cerrar:** obsoleto con la nueva arquitectura, o se documenta como límite conocido.

**IDs nuevos:** `CL-` (Chile), `US-` (EEUU, conserva el ID original), `ETF-` (conserva el número original), `TD-` (Tendencia, conserva el ID original).

## Resumen

| Etiqueta | Ítems |
|---|---|
| A — Durante la migración | 34 (+2 de producción de la revisión de la Fase 7) |
| B — Después | 23 (+7 de metodología de la revisión de la Fase 7) |
| C — Descartar / cerrar | 3 |
| Decidido (TD-LIC-1: repo privado) | 1 |
| **Total** | **61** (16 Chile · 22 EEUU · 9 ETF · 13 Tendencia · 1 del sistema de diseño) |

---

## A — Durante la migración

### Seguridad y despliegue (se unifican: aparecían repetidos en 3 proyectos)

| ID | Ítem | Fase | Cómo se resuelve |
|---|---|---|---|
| ETF-23 · US-BACK-05 | Sin autenticación en ningún endpoint | 1 ✅ | Dependencia `require_api_key` (`X-API-Key`) en todas las rutas salvo `/api/health`; el navegador nunca ve la key. |
| ETF-24 · US-BACK-02 | Sin rate limiting | 1 ✅ (global) / 4 ✅ (EEUU) / 5 ✅ (ETF `/run` 3/min, `/pdf` 10/min) | rate limit de `internal/core/ratelimit.go` en endpoints costosos (`/run`, `/evaluate`, portafolio) + allowlist de IP del servidor frontend. |
| ETF-26 | CORS amplio; revisar si hay cookies | 1 ✅ | `ALLOWED_ORIGINS` explícito. El login con cookies (si lo hay) vive en SvelteKit; el backend sigue con API key, sin credenciales CORS. |
| ETF-27 | `/docs` y `/openapi.json` públicos | 1 ✅ | Deshabilitados en producción (`docs_url=None` según `settings.ENV`); disponibles en dev para `npm run gen:api`. |
| ETF-28 | Sin HTTPS forzado ni cabeceras de seguridad | 8 | Caddy (HTTPS automático + HSTS) en ambos servidores; CSP en `vite.config.ts` (el BFF y `hooks.server.ts` se retiraron: ADR 0005). |
| US-SEC-01 · CL (infra) | CORS hardcodeado a localhost | 1 ✅ | `settings.go` + `.env`. |
| US-SEC-02 | Path param `ticker` sin validar | 4 ✅ | `Path(..., pattern=r"^[A-Z0-9.\-^=]{1,15}$")` (permite `.SN`, `.TW`, `^`, `=X`). |
| ETF-20 | Host del backend / plan Vercel | 8 | **Reemplazado** por la arquitectura decidida: 2 servidores (front Node + back Go) con disco persistente para `data/` y cron en el backend. |

### Calidad, CI y operación

| ID | Ítem | Fase | Cómo se resuelve |
|---|---|---|---|
| US-TEST-01 · US-DEVOPS-01 | Tests sin CI | 1–2 ✅ | GitHub Actions: jobs `backend-go` (gofmt + vet + golangci-lint + tests) y `frontend` (check + lint + vitest + build). |
| US-TEST-03 | Sin linting obligatorio | 1–2 ✅ | golangci-lint (back) + ESLint/Prettier + `svelte-check` (front) en CI. |
| US-TEST-02 | Frontend sin tests | 2 ✅ (base) | Vitest para lógica (`DataTable`, scoring de Chile) + Playwright smoke por módulo (Fase 8). |
| CL (tests) | Sin tests de API ni de scoring IPSA | 3 ✅ | Tests de winsorización, z-score, duplicados y FX. |
| US-DEVOPS-02 | Sin Docker | 8 | `deploy/backend` y `deploy/frontend` con Dockerfile + compose. |
| US-DEVOPS-03 | Solo Task Scheduler de Windows | 7 ✅ | `crontab` en el servidor backend; Task Scheduler solo para dev local. |
| US-DEVOPS-04 | Sin alertas si falla un pipeline | 7 ✅ | `internal/core/jobs.go` registra estado; webhook/correo al fallar + página `/estado`. |
| US-DATA-04 | El batch no reintenta la corrida completa | 7 ✅ | Reintento con backoff en el runner común de jobs. |
| TD-OPS-1 | Sin lockfile | 0 ✅ | `go.sum` en el monorepo. |
| CL (reintentos) | Sin reintento por ticker ante errores transitorios | 7 ✅ | `internal/yahoo`: cliente común (espaciado global, reintento con backoff ante 429, reintentos de red) + `History` con caché. |

### Datos y robustez

| ID | Ítem | Fase | Cómo se resuelve |
|---|---|---|---|
| CL (CSV corrupto) | Manejo de CSV con columnas faltantes | 3 ✅ | Validación al cargar + tipos de respuesta tipados; 503 `invalid_data` con el nombre de las columnas faltantes. |
| US-BACK-01 | Sin caché en `/evaluate` | 4 ✅ | `TTLCache` de 5 min por ticker. |
| TD-MODEL-3 | Sin validación de factibilidad de la configuración | 6 ✅ | `config.validar_configuracion()` antes de cada corrida (`top_n × MAX_WEIGHT ≥ 1`, `top_n ≤ universo`, portafolios reducidos < `top_n`); el pipeline falla con el detalle. |
| TD-DATA-1 | FX limitado a 6 monedas + sin aviso visible | 6 ✅ (aviso) / 7 ✅ (FX común) | Aviso visible en el resumen y en Limitaciones ✅. `internal/fx` (`USDPair`, `ToUSD`): cualquier moneda ISO 4217 y subunidades GBp/ZAc/ILA ✅. |
| TD-OPS-2 | `excluded_tickers.txt` sin encabezado | 6 ✅ | `GET /api/tendencia/excluidos` (ticker, nombre, motivo) + tabla con CSV en `/tendencia/limitaciones`. |
| TD-CFG-1 | Parámetros del modelo solo editables en código | 7 ✅ | `fogradar job tendencia --top-n N --top-n-us N --max-weight X` (validados, solo para esa corrida, quedan en `run_manifest.json`). |
| TD-MACRO-1 | Datos macro (FRED) solo se actualizan con `--macro` manual | 8 (producción) | Agregar `--macro` a Tendencia en el programa `diario` (lun–vie 18:00) para que la macro esté siempre al día. Requiere `FRED_API_KEY` en el servidor; +segundos de descarga. **Pendiente por decisión del usuario**: se hace junto con la puesta en producción. |

### Frontend (lo resuelven los componentes comunes)

| ID | Ítem | Fase | Cómo se resuelve |
|---|---|---|---|
| CL (CSV) | Exportar vista filtrada a CSV | 2 ✅ (en `DataTable`) | `DataTable` común con botón CSV. |
| ETF-15/16 (retirados por decisión previa) | Búsqueda y CSV faltantes en Excluidos/Categorías | 5 ✅ | Sin trabajo extra: `DataTable` los trae por defecto. |
| DESIGN (ETF) | Orden por click solo en "Top 15 Vanguard" | 2 ✅ | Todas las tablas ordenables (regla del DESIGN_SYSTEM). |
| CL (dark mode) | Falta toggle de modo oscuro | 2 ✅ | Tema común del sitio. |
| CL (ranking.json) | Snapshot huérfano `src/data/ranking.json` | 3 ✅ | No se porta. |

---

## B — Después (post Fase 6)

### Épica B1 · Historial unificado entre corridas — ✅ 2026-10-04
Una base `data/<modulo>/history.db` por módulo, índice derivado de los archivos de cada
corrida (ADR 0003, `INFORME_SQLITE.md`). Helper común `internal/core/histdb.go` (WAL, versión de
esquema, `diff_corridas`); escritura al final del pipeline sin hacerlo fallar; backfill con
`fogradar job <modulo> --backfill`; endpoints `/api/<mod>/history/*`.

| ID | Ítem | Estado |
|---|---|---|
| US-BACK-03 | Persistencia en SQLite en vez de JSON por corrida | ✅ `internal/modules/us/history.go` (los JSON se mantienen como fuente de verdad) |
| US-BACK-04 | Diff entre corridas (entraron/salieron del top, delta de score). ETF ya tiene "Rotación": se generaliza. | ✅ `/history/diff` en Chile, EEUU y ETF; vistas `/eeuu/historial` y `/etf/evolucion` |
| US-DATA-03 | Histórico de score de un ticker a lo largo de las corridas | ✅ `/history/ticker/{t}` en los 3 módulos |
| CL (histórico) | Endpoint de histórico de rankings IPSA + gráfico de evolución | ✅ `/api/cl/history/*` + `/chile/historial` |
| TD (universo) | Guardar los pesos de Markowitz del universo completo (no solo los seleccionados) | ✅ `universe_weights` (history.db v2) + gráfico en `/tendencia/historial` |
| B1-ETF-INC | Incidencias del screener y `ETF_eliminados.csv` en la base (detectar tickers que fallan N corridas seguidas) | Pendiente |
| B1-ETF-SQL | Reescribir racha/rotación de `rotacion.go` sobre SQL | Pendiente (no urgente) |
| B1-JOBS | Tabla `jobs` (duración, requests y 429 de Yahoo por corrida) con gráfico en `/estado` | Pendiente (opcional) |

### Épica B2 · Backtesting / validación del modelo
Pedido en los 4 proyectos. B1 ya está hecho: falta acumular corridas. EEUU guarda los
cierres deduplicados en `us_stocks/history.db` (`prices`) y ETF el `Precio` de cada corrida.

| ID | Ítem |
|---|---|
| US-DATA-02 | Correlación (Spearman) score vs. retorno a 30/60/90 días |
| TD-MODEL-2 | Backtest walk-forward de Markowitz + tendencia |
| CL (backtesting) | ¿El score compuesto IPSA predice retorno futuro? |
| ETF-17 (retirado por decisión previa) | Portar el backtest de la metodología. Retomar solo si se aprueba la épica. |

### Modelo y metodología

| ID | Ítem |
|---|---|
| TD-MODEL-1 | Shrinkage de covarianza Ledoit-Wolf |
| US-DATA-05 | Perfiles de market cap (mega/mid/small) |
| CL (perfiles) + CL (selector UI) | Perfiles de scoring predefinidos (Valor/Momentum/Dividendo/Balanceado) + selector en la UI |
| CL (sector-neutral) | z-score sector-neutral (solo si crece el universo) |
| CL (payout) | Payout > 100 % como flag general de riesgo |

### Datos y universo

| ID | Ítem |
|---|---|
| US-DATA-01 | Universo EEUU frágil (scraping de Wikipedia): segundo scraper de respaldo |
| TD-DISC-2 | Deduplicación por ISIN en vez de por nombre |
| TD-DATA-2 | Taxonomía de sectores común (GICS) para los 4 módulos. Habilita la comparación entre módulos (el buscador global ya existe desde la Fase 7). |

### Otros

| ID | Ítem |
|---|---|
| CL (gráficos) | Gráficos de pilares por acción / comparación entre acciones |
| CL (disparar corrida) | Disparar la corrida desde la web (`POST /api/refrescar` en el original). Los pipelines no se ejecutan dentro del request: se generaliza como `POST /api/jobs/{modulo}/run` protegido, que encola el job (hoy solo ETF tiene algo así). |
| US-DEVOPS-05 | Logs centralizados / logrotate en el servidor |
| TD-TEST-1 · TD-TEST-2 | Smoke test con datos reales en CI (workflow manual) + tests de estructura del PDF |

---

## Revisión de la Fase 7 (2026-09-29)

Revisión del código con criterios estándar de TI (OWASP API Top 10, WCAG 2.1 AA) y de finanzas
cuantitativas. **Corregido** en la Fase 7 (detalle en el commit):
Black-Litterman con rf, mediana GBM de Tendencia, beta de ETF por fecha, `POST /api/etf/run`
bajo el lock global, API key no ASCII, rate limit antes de la autenticación, límite de
concurrencia en endpoints costosos, lista blanca de archivos descargables, clave de prod
robusta, fechas con zona, rotación del registro de jobs, inyección CSV, accesibilidad del
sub-menú, paginación del historial, PDF de Tendencia (encabezados claros, ≥ 10 pt).

### Pendiente — metodología (B; cambian rankings: decidir con backtest, épica B2)

| ID | Ítem | Propuesta |
|---|---|---|
| TD-MODEL-4 | ETF: Sharpe/Treynor con rf fija de 2 % (`internal/modules/etf/screener.go`); la T-bill a 3 meses está cerca de 4 %. Favorece a los fondos de baja volatilidad. | Leer rf del ^IRX (como Tendencia) con respaldo configurable. |
| TD-MODEL-5 | EEUU: el MACD del score técnico está en dólares (histograma sin normalizar): una acción de USD 900 satura el puntaje y una de USD 15 queda en 50. | Puntuar `hist / precio · 100` con cortes re-escalados. |
| TD-MODEL-6 | Chile: el pilar momentum promedia 5D…1A, todos hasta hoy (incluye el mes más reciente, con efecto reversión de corto plazo) y pesa 2,0. | 12-1 y 6-1 (saltar el último mes) o quitar 5D/1M del pilar. |
| TD-MODEL-7 | Chile: P/E, EV/EBITDA y D/E negativos pasan a NaN (`solo_positivos`): una empresa con pérdidas no se penaliza en Valor. | Usar rendimientos (E/P, EBITDA/EV) o asignar el peor z-score a los ≤ 0. |
| TD-MODEL-8 | ETF momentum: sin filtro MAD de atípicos y con min-max (un ETF apalancado comprime al resto); `Q_raw = 0,5·Sharpe + 0,5·Treynor` mezcla escalas; la "consistencia" compara con el archivo **más antiguo** (la brecha crece con el archivo). | Mismo filtro MAD del ranking o percentiles; normalizar por rango antes de promediar; comparar contra un rezago fijo (~26 semanas). |
| TD-MODEL-9 | Tendencia: la covarianza muestral por pares con historias distintas depende del arreglo de semidefinida positiva; huecos > 2 semanas se pierden del CAGR. | Junto con TD-MODEL-1 (Ledoit-Wolf). |
| US-DATA-08 | EEUU: la "mejor combinación" se elige por retorno esperado histórico dentro de la muestra (sesgo de selección); en BL rankea con medias históricas y no con el posterior. | Rankear por Sharpe fuera de muestra (con B2) y, en BL, por el posterior. |

### Pendiente — producción (Fase 8, no se hace hasta depurar el desarrollo)

| ID | Ítem | Propuesta |
|---|---|---|
| SEC-PROD-1 | Detrás de Caddy sin `ADDRESS_HEADER`/`XFF_DEPTH`, todos los usuarios comparten un balde de rate limit. | Obsoleto: ya no hay servidor de frontend (ADR 0005). |
| SEC-PROD-2 | Mensajes internos en `/api/jobs` y errores de Yahoo/SQLite visibles en el sitio público. | En prod: mensaje genérico + `code`, detalle solo en el log (el hub ya lo hace). |

---

## C — Descartar / cerrar

| ID | Motivo |
|---|---|
| CL (paginación/virtualización) | Universo de ~30 acciones; `DataTable` puede agregarla después si hiciera falta. |
| US-DATA-06 | Views de Black-Litterman heurísticas: documentado como límite conocido en `docs/METODOLOGIAS.md` ✅. |
| TD-DISC-3 | Umbrales del screener "no validados": ya hay 6 corridas reales en `output/`. Revisar los logs una vez y cerrar. |

## Decisiones tomadas

| ID | Decisión | Fecha |
|---|---|---|
| TD-LIC-1 | Repositorio **privado** en GitHub, **sin archivo LICENSE** (todos los derechos reservados). No hay archivo de licencia en el repo. | 2026-09-27 |
