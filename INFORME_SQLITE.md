# Informe: uso de SQLite en los 4 módulos de Fog Radar Quant

*Fecha: 2026-09-29 · Alcance: revisión de código (`backend/src/fog_radar/modules/*`) y de los datos en `data/`.
No se modificó código.*

> **Estado (2026-10-04): implementado** (ADR 0003, `docs/ADR/0003-historial-sqlite-por-modulo.md`).
> §4 y los pasos 1-4 de §5 están hechos: `core/histdb.py`; Tendencia con WAL, `user_version`,
> `UNIQUE(analysis_id, ticker)` y `universe_weights`; `history.db` en Chile, EEUU y ETF con backfill
> y endpoints `/history/*`. Backfill sobre `data/` real: Chile 9 corridas, EEUU 14 (18 045 cierres),
> ETF 23 (22 de `hist/` + la vigente), Tendencia 8. Quedan pendientes (BACKLOG, Épica B1): incidencias
> del screener y `ETF_eliminados.csv` en la base, reescribir la racha/rotación de ETF sobre SQL, la
> tabla `jobs` (§3.5) y la Épica B2.

## 1. Resumen

| Módulo | Cómo guarda hoy cada corrida | ¿Usa SQLite? | ¿Hay historial consultable? | Recomendación | Prioridad |
|---|---|---|---|---|---|
| **Tendencia** | Carpeta `output/<fecha>_NN/` (JSON/CSV) **+** índice `history.db` | ✅ Sí (único módulo) | ✅ 6 endpoints `/history/*` | Mantener; pequeñas mejoras (WAL, versión de esquema, guardar universo completo) | Baja |
| **EEUU** | `runs/<timestamp>/{result,candidates,prices}.json` (14 corridas) | ❌ | ❌ Solo lista de corridas | **Sí, agregar `history.db`** | **Alta** |
| **ETF** | `current/` + `hist/*_<fecha>.csv` (186 archivos, 22 corridas) + `historial_corridas.csv` | ❌ | Parcial (rotación vs. corrida anterior, racha y resumen por corrida, todo leyendo CSV) | **Sí, agregar `history.db`** | **Alta** |
| **Chile** | `ranking_acciones_<ts>.csv` + `indicadores_acciones_<ts>.csv` (+ xlsx) | ❌ | ❌ Solo la última | Sí, agregar `history.db` (el más simple, buen piloto) | Media |
| **Transversal (jobs)** | `data/_jobs/<modulo>.jsonl` | ❌ | `/estado` lee el último estado | Opcional: tabla `jobs` para tendencias de duración, 429 y fallas | Baja |

**Conclusión:** hoy solo **Tendencia** usa SQLite (los otros `.db` en `data/_cache/yfinance/` son la caché interna de yfinance).
Los que más ganan con SQLite son **EEUU** y **ETF**: acumulan muchas corridas con puntajes por ticker, pero ese historial está repartido en
decenas de archivos que nadie consulta en conjunto. **Chile** también gana y es el más barato de hacer, así que sirve como piloto del esquema común.
Es exactamente la **Épica B1 del BACKLOG** (US-BACK-03, US-BACK-04, US-DATA-03, "CL histórico") y es requisito de la **Épica B2 (backtesting)**.

---

## 2. Principio de diseño (tomado de Tendencia, que ya funciona)

Tendencia usa SQLite como **índice consultable de todas las corridas, no como reemplazo** de los archivos:

- Los archivos de cada corrida siguen siendo la fuente de verdad (y lo que sirve la "última corrida"). → **La paridad con los originales no se toca.**
- El pipeline escribe en la base **al final**, en una sola transacción e idempotente (si la corrida ya existe, la borra y la reinserta).
- La API abre la base **solo lectura** (`mode=ro`) y devuelve `[]` si todavía no existe.
- Un script de *backfill* reconstruye la base desde los archivos (`tendencia/backfill.py`).

Se propone replicar este mismo patrón en los otros 3 módulos: **un `history.db` por módulo en `data/<modulo>/`**
(como propone el BACKLOG), con la misma estructura base y un helper común.

---

## 3. Análisis por módulo

### 3.1 Tendencia (ya usa SQLite)

**Estado:** `data/tendencia/history.db` (136 KB): 8 corridas, 16 análisis, 240 holdings, 88 filas de sectores y 160 de momentum.
Esquema en `modules/tendencia/storage/db.py` (`runs → analyses → holdings / sectors / momentum`, con `ON DELETE CASCADE`).
Se escribe en `history_writer.store_run()` y se lee en `service.py` (por ejemplo, `ticker_history`, `sector_history` y `momentum_leaders`).

**Mejoras sugeridas (pequeñas):**
1. **`PRAGMA journal_mode=WAL`** al escribir. Hoy usa el modo por defecto (rollback journal): mientras el pipeline escribe,
   un GET de la API puede recibir `database is locked` y responder 503. Con WAL los lectores no se bloquean.
2. **Versión de esquema** (`PRAGMA user_version`) y migraciones simples. Hoy es `CREATE TABLE IF NOT EXISTS`, así que agregar una
   columna nueva a una base existente no se aplica sola.
3. **Guardar los pesos del universo completo**, no solo los 15 seleccionados y el top de momentum. Hoy no se puede ver cómo "se acerca"
   un ticker al top ni por qué salió (el optimizador calcula `weights_full` para ~74 tickers y se descarta).
4. `UNIQUE(analysis_id, ticker)` en `holdings`, para evitar duplicados silenciosos.
5. Extraer `get_connection`/`init_db` a un helper común en `core/` (ver §4) para reutilizarlo en los demás módulos.

### 3.2 EEUU — **prioridad alta**

**Estado:** cada corrida es una carpeta `data/us_stocks/runs/<ISO>/` con `result.json` (top 15 con score y pilares), `candidates.json`
(~59 candidatos de 503 con todas sus métricas, ~100 KB), `prices.json` (matriz de cierres, ~450 KB) y `run.log`. Hay 14 corridas desde el 18-09.
La API solo expone la **última** (`/rankings/latest*`) y la lista de nombres (`/rankings/runs`). **No hay forma de ver la evolución de un ticker.**

**Qué ganaría con SQLite:**
- **Histórico de score por ticker** (US-DATA-03): score, momentum 12-1 y pilares (valuation, profitability, financial_health,
  shareholder_return) de cada candidato en cada corrida.
- **Diff entre corridas** (US-BACK-04): qué entró o salió del top 15 y cuánto cambió el score. Es un `JOIN` entre dos `run_id`, sin leer JSON.
- **Backtesting** (B2 / US-DATA-02): el `prices.json` de cada corrida trae los cierres. Guardando un precio por (ticker, fecha) se puede
  medir el retorno a 30/60/90 días de lo que el modelo recomendó, **sin llamadas extra a Yahoo**.
- La corrida es **diaria** (lunes a viernes), así que es el módulo que más rápido acumula historia (~15 000 filas de candidatos al año).

**Tablas sugeridas:**
```sql
runs(id, run_id TEXT UNIQUE, started_at, finished_at, status, min_market_cap, universe_total, candidates_total, top_n)
candidates(run_pk → runs.id, ticker, name, sector, market_cap, price, momentum_12_1,
           return_1d, return_5d, return_3m, return_6m, return_1y, return_ytd,
           valuation, profitability, financial_health, shareholder_return, score,
           rank INTEGER, in_top INTEGER,  UNIQUE(run_pk, ticker))
prices(ticker, date, close, PRIMARY KEY(ticker, date))   -- deduplicado entre corridas
```
`prices.json` sigue en disco (lo usa el portafolio); la tabla `prices` solo guarda cierres únicos. Todas las corridas comparten fechas,
así que ocupa muy poco.

**Endpoints nuevos posibles:** `/api/us/history/ticker/{t}`, `/api/us/history/diff?a=&b=`, `/api/us/history/top` (permanencia en el top).

### 3.3 ETF — **prioridad alta**

**Estado:** es el módulo con más volumen (32 MB). Cada corrida semanal archiva ~9 CSV en `data/etf/hist/` (22 corridas, 186 archivos), además de
`etf_perf_data_*` (~1 226 ETFs × 20 métricas) y los CSV crudos del screener.
Hay historial "a mano" en tres lugares:
- `historial_corridas.csv`: una fila resumen por corrida, que se reescribe entera con pandas.
- `rotacion.py`: busca el archivo previo con `glob` y parseo de nombres, y calcula la **racha** de cada ticker en el top 15
  leyendo todos los `etf_top15_recomendados_*.csv`.
- `incidencias_*.csv` y el nuevo `ETF_eliminados.csv`: problemas del screener, separados por corrida.

**Qué ganaría con SQLite:**
- **Histórico por ETF**: score compuesto, posición, Sharpe/Treynor/Beta y rendimientos de cada ETF en cada corrida
  (hoy hay que abrir 22 CSV).
- **Rotación y racha por SQL**: la lógica actual con `glob` y ordenamiento por (fecha, secuencia) se vuelve una consulta con `ORDER BY run_date`.
  Además se generaliza a "rotación entre dos corridas cualquiera" y no solo contra la anterior.
- **Tendencia por categoría**: cómo evoluciona el peso o score de cada categoría Morningstar en el tiempo (hoy solo existe la foto actual).
- **Seguimiento de la calidad del universo**: la tabla `incidencias` por corrida permite detectar tickers que fallan N corridas seguidas y
  proponer su eliminación automáticamente. Es justo el criterio que se usó a mano hoy con los 24 ETF.
- **Backtesting**: la columna `Precio` de cada corrida ya permite calcular el retorno posterior de los ETF recomendados.

**Tablas sugeridas:**
```sql
runs(id, fecha_tag TEXT UNIQUE, run_at, universo_bruto, universo_depurado, n_outliers,
     sharpe_promedio, beta_promedio, top1_ticker, top1_score)        -- reemplaza historial_corridas.csv
ranking(run_pk, ticker, rank, score, s_sharpe, s_treynor, s_1y, s_6m, s_consistencia, s_tendencia, ...,
        precio, sharpe, treynor, beta, perf_5d, perf_1m, perf_6m, perf_1y, category, proveedor,
        en_top15 INTEGER, en_top15_simple INTEGER, en_momentum_top15 INTEGER, UNIQUE(run_pk, ticker))
excluidos(run_pk, ticker, ventana, z)
categorias(run_pk, category, n, score_prom, ...)
screener_incidencias(run_pk, ticker, tipo, detalle, ts)
universo_eliminados(ticker, company, name, category, fecha_eliminacion, ultima_valida, motivo)  -- migra ETF_eliminados.csv
```
Volumen estimado: ~1 200 filas por corrida × 52 semanas ≈ 60 000 filas al año, muy poco para SQLite.

### 3.4 Chile (IPSA) — prioridad media

**Estado:** 15 CSV con timestamp (`ranking_acciones_*` e `indicadores_acciones_*`, más xlsx) para 30 acciones.
La API solo lee el **último** archivo (`core.runs.latest_file`). El BACKLOG ya pide "Endpoint de histórico de rankings IPSA + gráfico de evolución".

**Qué ganaría con SQLite:** evolución del score compuesto y de los 6 pilares (valor, calidad, crecimiento, salud financiera, dividendo, momentum)
por acción, y cambios de posición en el ranking día a día. El volumen es mínimo (30 filas por corrida).

**Por qué hacerlo primero (piloto):** es el esquema más simple (una tabla de ranking) y sirve para validar el helper común,
el backfill desde CSV y los endpoints `/history/*` antes de pasar a EEUU y ETF.

```sql
runs(id, ts TEXT UNIQUE, run_at, n_tickers)
ranking(run_pk, ticker, rank, nombre, sector, industria, score_compuesto,
        score_valor, score_calidad, score_crecimiento, score_salud, score_dividendo, score_momentum,
        metricas_disponibles, UNIQUE(run_pk, ticker))
indicadores(run_pk, ticker, metrica, valor)   -- formato largo: 65 columnas → sin migraciones al agregar métricas
```

### 3.5 Transversal: registro de jobs (opcional)

`data/_jobs/<modulo>.jsonl` guarda inicio y fin de cada job. Hoy solo se usa el último estado.
Pasarlo a una tabla `jobs(id, module, status, trigger, started_at, finished_at, duration_s, requests_yahoo, n_429, reintentos, message)`
permitiría graficar en `/estado` la **duración y los 429 de Yahoo por corrida en el tiempo**: una señal temprana de bloqueo o de degradación.
Hoy los contadores solo aparecen en el resumen del log (por ejemplo, "1509 requests a Yahoo, 0 respuestas 429").
Es de baja prioridad: el JSONL funciona bien y es de solo anexar.

---

## 4. Propuesta de implementación

**Helper común `core/histdb.py`** (extraído de `tendencia/storage/db.py`):
- `conectar_escritura(path)`: activa `foreign_keys=ON`, `journal_mode=WAL` y `busy_timeout`, y hace commit o rollback.
- `conectar_lectura(path)`: abre en `mode=ro` y devuelve `None` si la base no existe (patrón actual de `tendencia/service.py`).
- `migrar(conn, migraciones: list[str])`: aplica las migraciones pendientes según `PRAGMA user_version`.

**Por módulo:** `modules/<modulo>/history.py` con `SCHEMA`, `guardar_corrida(...)` (llamada desde `pipeline.py` después de instalar
los archivos, bajo `lock_pipelines`) y `backfill(settings)` para reconstruir desde los archivos existentes. Se expone en la CLI:
`python -m fog_radar.jobs <modulo> --backfill-history`.

**API:** endpoints con la misma forma en los 4 módulos. El hub y el buscador global pueden mostrar "tendencia del ticker" para cualquier módulo.
- `GET /api/<mod>/history/runs`
- `GET /api/<mod>/history/ticker/{ticker}`
- `GET /api/<mod>/history/diff?desde=&hasta=`

**Tests:** en cada módulo, un test de backfill que tome los fixtures (`tests/<mod>/fixtures/`) y verifique que la base reproduce
los mismos valores que los archivos. Los tests de paridad existentes no cambian, porque los archivos siguen igual.

**Respaldo:** los `history.db` quedan en `data/<modulo>/` y entran en el respaldo diario de `data/` que ya está en el ROADMAP.

## 5. Orden sugerido

1. `core/histdb.py` + WAL y `user_version` en Tendencia (sin cambiar su esquema).
2. **Chile** (piloto): esquema, writer, backfill de los 15 CSV, `/history/*` y un gráfico de evolución en `/chile`.
3. **EEUU**: `runs`, `candidates` y `prices`; backfill de las 14 corridas; histórico por ticker y diff.
4. **ETF**: `runs` (reemplaza `historial_corridas.csv`), `ranking`, `excluidos`, `categorias` e `incidencias`; backfill de las 22 corridas de `hist/`.
   Luego se reescriben la racha y la rotación sobre SQL, comprobando que dan lo mismo que la versión con CSV.
5. (Opcional) tabla `jobs` y gráficos en `/estado`.
6. Con 2–4 hechos, queda desbloqueada la **Épica B2 (backtesting)**: correlación score vs. retorno a 30/60/90 días.

## 6. Riesgos y cuidados

- **No cambiar la lógica cuantitativa:** la base es un índice derivado. Si algo falla al escribirla, se registra un WARNING y el pipeline
  **no** falla (los archivos ya quedaron instalados).
- **Windows y el disco D:**: SQLite con WAL funciona bien en disco local. Evitar poner `data/` en carpetas sincronizadas (OneDrive/Dropbox),
  porque corrompen los archivos `-wal`/`-shm`.
- **Un solo escritor:** ya está garantizado por `lock_pipelines`. Las escrituras desde la API (re-análisis ETF) deben tomar el mismo lock, como hoy.
- **Tamaño:** aun guardando todo, las 4 bases deberían quedar en unos pocos MB al año. No se justifica Postgres.
