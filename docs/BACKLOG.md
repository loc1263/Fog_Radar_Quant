# Backlog unificado — Fog Radar Quant

> Triage hecho el 2026-09-27 (Fase 0) a partir de los 4 archivos de mejoras pendientes:
> `Stock_Choice_Nacional/WEB/mejoras_pendientes.txt` · `Stock_Choice_EEUU/mejoras_pendientes.md` ·
> `ETF_Choice/WEB/mejoras_web_pendientes.txt` · `Stock_Tendencia/backend/mejoras_pendientes.txt`.
> Solo se consideraron ítems **abiertos** (los resueltos quedan en los históricos de cada proyecto).

**Etiquetas**

- **A — Durante la migración:** se resuelve en la fase indicada, casi siempre porque la nueva arquitectura ya lo exige (seguridad, CI, Docker, componentes comunes).
- **B — Después:** mejoras de producto o de modelo. No entran hasta cerrar la Fase 6 (paridad de los 4 módulos).
- ✅ en la columna Fase = resuelto.
- **C — Descartar / cerrar:** obsoleto con la nueva arquitectura, o se documenta como límite conocido.

**IDs nuevos:** `CL-` (Chile), `US-` (EEUU, conserva el ID original), `ETF-` (conserva el número original), `TD-` (Tendencia, conserva el ID original).

## Resumen

| Etiqueta | Ítems |
|---|---|
| A — Durante la migración | 34 |
| B — Después | 23 |
| C — Descartar / cerrar | 3 |
| Decidido (TD-LIC-1: repo privado) | 1 |
| **Total** | **61** (16 Chile · 22 EEUU · 9 ETF · 13 Tendencia · 1 del sistema de diseño) |

---

## A — Durante la migración

### Seguridad y despliegue (se unifican: aparecían repetidos en 3 proyectos)

| ID | Ítem | Fase | Cómo se resuelve |
|---|---|---|---|
| ETF-23 · US-BACK-05 | Sin autenticación en ningún endpoint | 1 ✅ | Dependencia `require_api_key` (`X-API-Key`) en todas las rutas salvo `/api/health`; el navegador nunca ve la key (BFF). |
| ETF-24 · US-BACK-02 | Sin rate limiting | 1 ✅ (global) / 4 ✅ (EEUU) / 5 ✅ (ETF `/run` 3/min, `/pdf` 10/min) | `rate_limit()` de `core/ratelimit.py` en endpoints costosos (`/run`, `/evaluate`, portafolio) + allowlist de IP del servidor frontend. |
| ETF-26 | CORS amplio; revisar si hay cookies | 1 ✅ | `ALLOWED_ORIGINS` explícito. El login con cookies (si lo hay) vive en SvelteKit; el backend sigue con API key, sin credenciales CORS. |
| ETF-27 | `/docs` y `/openapi.json` públicos | 1 ✅ | Deshabilitados en producción (`docs_url=None` según `settings.ENV`); disponibles en dev para `npm run gen:api`. |
| ETF-28 | Sin HTTPS forzado ni cabeceras de seguridad | 8 | Caddy (HTTPS automático + HSTS) en ambos servidores; CSP y cabeceras en `hooks.server.ts`. |
| US-SEC-01 · CL (infra) | CORS hardcodeado a localhost | 1 ✅ | `settings.py` + `.env`. |
| US-SEC-02 | Path param `ticker` sin validar | 4 ✅ | `Path(..., pattern=r"^[A-Z0-9.\-^=]{1,15}$")` (permite `.SN`, `.TW`, `^`, `=X`). |
| ETF-20 | Host del backend / plan Vercel | 8 | **Reemplazado** por la arquitectura decidida: 2 servidores (front Node + back FastAPI) con disco persistente para `data/` y cron en el backend. |

### Calidad, CI y operación

| ID | Ítem | Fase | Cómo se resuelve |
|---|---|---|---|
| US-TEST-01 · US-DEVOPS-01 | Tests sin CI | 1–2 ✅ | GitHub Actions: jobs `backend` (ruff + pytest) y `frontend` (check + lint + vitest + build). |
| US-TEST-03 | Sin linting obligatorio | 1–2 ✅ | Ruff (back) + ESLint/Prettier + `svelte-check` (front) en CI. |
| US-TEST-02 | Frontend sin tests | 2 ✅ (base) | Vitest para lógica (`DataTable`, scoring de Chile) + Playwright smoke por módulo (Fase 8). |
| CL (tests) | Sin tests de API ni de scoring IPSA | 3 ✅ | pytest de `winsorizar`, `calcular_zscore`, duplicados, FX + tests de paridad contra el original. |
| US-DEVOPS-02 | Sin Docker | 8 | `deploy/backend` y `deploy/frontend` con Dockerfile + compose. |
| US-DEVOPS-03 | Solo Task Scheduler de Windows | 7 | `crontab` en el servidor backend; Task Scheduler solo para dev local. |
| US-DEVOPS-04 | Sin alertas si falla un pipeline | 7 | `core/jobs.py` registra estado; webhook/correo al fallar + página `/estado`. |
| US-DATA-04 | El batch no reintenta la corrida completa | 7 | Reintento con backoff en el runner común de jobs. |
| TD-OPS-1 | Sin lockfile | 0 ✅ | `uv.lock` en el monorepo. |
| CL (reintentos) | Sin reintento por ticker ante errores transitorios | 7 | Cliente yfinance común (`core/yf_client.py`, base: `robust_yf.py` de Tendencia). |

### Datos y robustez

| ID | Ítem | Fase | Cómo se resuelve |
|---|---|---|---|
| CL (CSV corrupto) | Manejo de CSV con columnas faltantes | 3 ✅ | Validación al cargar + `response_model` Pydantic; 503 `invalid_data` con el nombre de las columnas faltantes. |
| US-BACK-01 | Sin caché en `/evaluate` | 4 ✅ | `TTLCache` de 5 min por ticker. |
| TD-MODEL-3 | Sin validación de factibilidad de la configuración | 6 ✅ | `config.validar_configuracion()` antes de cada corrida (`top_n × MAX_WEIGHT ≥ 1`, `top_n ≤ universo`, portafolios reducidos < `top_n`); el pipeline falla con el detalle. |
| TD-DATA-1 | FX limitado a 6 monedas + sin aviso visible | 6 ✅ (aviso) / 7 (FX común) | Aviso visible en el resumen y lista en Limitaciones cuando se excluyen tickers por moneda ✅. Pasar `build_dataset` a `core/fx.py` queda para la Fase 7 (consolidación). |
| TD-OPS-2 | `excluded_tickers.txt` sin encabezado | 6 ✅ | `GET /api/tendencia/excluidos` (ticker, nombre, motivo) + tabla con CSV en `/tendencia/limitaciones`. |
| TD-CFG-1 | Parámetros del modelo solo editables en código | 7 | Requiere opciones con valor en el CLI de jobs (hoy solo banderas): pasa a la Fase 7 junto con `run_all_pipelines.py`. Los parámetros ya viajan por la API (`parametros`). |

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

### Épica B1 · Historial unificado entre corridas
Hoy cada módulo guarda su historia distinto: Tendencia en SQLite (`history.db`), ETF en `data_hist/` (CSV), EEUU en `runs/<id>/*.json` y Chile en CSV con timestamp. Propuesta: una base SQLite por módulo con el mismo esquema (`runs`, `rankings(run_id, ticker, score, …)`) y endpoints comunes.

| ID | Ítem |
|---|---|
| US-BACK-03 | Persistencia en SQLite en vez de JSON por corrida |
| US-BACK-04 | Diff entre corridas (entraron/salieron del top, delta de score). ETF ya tiene "Rotación": se generaliza. |
| US-DATA-03 | Histórico de score de un ticker a lo largo de las corridas |
| CL (histórico) | Endpoint de histórico de rankings IPSA + gráfico de evolución |

### Épica B2 · Backtesting / validación del modelo
Pedido en los 4 proyectos. Depende de B1 (hacen falta corridas acumuladas).

| ID | Ítem |
|---|---|
| US-DATA-02 | Correlación (Spearman) score vs. retorno a 30/60/90 días |
| TD-MODEL-2 | Backtest walk-forward de Markowitz + tendencia |
| CL (backtesting) | ¿El score compuesto IPSA predice retorno futuro? |
| ETF-17 (retirado por decisión previa) | Portar `backtest_metodologia.py`. Retomar solo si se aprueba la épica. |

### Modelo y metodología

| ID | Ítem |
|---|---|
| TD-MODEL-1 | Shrinkage de covarianza Ledoit-Wolf |
| US-DATA-07 | Límite de concentración máxima por ticker en `best_combination` |
| US-DATA-05 | Perfiles de market cap (mega/mid/small) |
| CL (perfiles) + CL (selector UI) | Perfiles de scoring predefinidos (Valor/Momentum/Dividendo/Balanceado) + selector en la UI |
| CL (sector-neutral) | z-score sector-neutral (solo si crece el universo) |
| CL (payout) | Payout > 100 % como flag general de riesgo |

### Datos y universo

| ID | Ítem |
|---|---|
| US-DATA-01 | Universo EEUU frágil (scraping de Wikipedia): segundo scraper de respaldo |
| TD-DISC-2 | Deduplicación por ISIN en vez de por nombre |
| TD-DATA-2 | Taxonomía de sectores común (GICS) para los 4 módulos. También habilita el buscador global y la comparación entre módulos. |

### Otros

| ID | Ítem |
|---|---|
| CL (gráficos) | Gráficos de pilares por acción / comparación entre acciones |
| CL (disparar corrida) | Disparar la corrida desde la web (`POST /api/refrescar` en el original). Los pipelines no se ejecutan dentro del request: se generaliza como `POST /api/jobs/{modulo}/run` protegido, que encola el job (hoy solo ETF tiene algo así). |
| US-DEVOPS-05 | Logs centralizados / logrotate en el servidor |
| TD-TEST-1 · TD-TEST-2 | Smoke test con datos reales en CI (workflow manual) + tests de estructura del PDF |

---

## C — Descartar / cerrar

| ID | Motivo |
|---|---|
| CL (paginación/virtualización) | Universo de ~30 acciones; `DataTable` puede agregarla después si hiciera falta. |
| US-DATA-06 | Views de Black-Litterman heurísticas: se documenta como límite conocido en `docs/METODOLOGIAS.md`. |
| TD-DISC-3 | Umbrales del screener "no validados": ya hay 6 corridas reales en `output/`. Revisar los logs una vez y cerrar. |

## Decisiones tomadas

| ID | Decisión | Fecha |
|---|---|---|
| TD-LIC-1 | Repositorio **privado** en GitHub, **sin archivo LICENSE** (todos los derechos reservados). `pyproject.toml` declara `license = "Proprietary"`. | 2026-09-27 |
