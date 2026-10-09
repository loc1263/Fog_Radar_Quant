# ADR 0003 — Historial SQLite por módulo (`history.db`)

- **Fecha:** 2026-10-04
- **Estado:** aceptada (implementa la Épica B1 del BACKLOG; análisis previo en `INFORME_SQLITE.md`)

## Contexto

Solo Tendencia guardaba sus corridas en SQLite. EEUU (`runs/<id>/*.json`), ETF
(`hist/*_<fecha>.csv`) y Chile (`ranking_acciones_<ts>.csv`) acumulaban historia en
decenas de archivos que nadie consultaba en conjunto: no había evolución por ticker,
ni diff entre dos corridas cualesquiera, ni base para el backtesting (Épica B2).

## Decisión

Cada módulo tiene `data/<modulo>/history.db`, un **índice derivado** de los archivos
de sus corridas (patrón que ya usaba Tendencia):

1. **Los archivos siguen siendo la fuente de verdad** y lo que sirve la "última
   corrida". La paridad con los originales no depende de la base.
2. **Escritura al final del pipeline**, en una transacción e idempotente (la corrida
   se borra y se reinserta). Si falla, `histdb.guardar_sin_fallar` registra un WARNING
   y la corrida **no** falla.
3. **Backfill** por módulo (`python -m fog_radar.jobs <modulo> --backfill`) que
   reconstruye la base desde los archivos. Borrar `history.db` nunca pierde datos.
4. **Lectura en solo lectura** (`mode=ro`); `[]` si la base todavía no existe.
5. Helper común `core/histdb.py`: WAL (los GET no se bloquean mientras el pipeline
   escribe), `busy_timeout`, versión de esquema en `PRAGMA user_version` con
   migraciones en lista (nunca se editan las publicadas; un cambio es una migración
   nueva al final) y `diff_corridas` para comparar dos rankings.
6. **API con la misma forma** en los módulos: `GET /api/<mod>/history/runs`,
   `/history/ticker/{ticker}` y `/history/diff?desde=&hasta=` (respuesta común
   `HistDiff`); EEUU y ETF suman `/history/top` (permanencia y racha) y ETF
   `/history/categorias`. Los schemas propios llevan prefijo de módulo
   (`ClHist*`, `UsHist*`, `EtfHist*`) porque el OpenAPI es uno solo.

### Clave de corrida por módulo

| Módulo | Clave | Tablas |
|---|---|---|
| Chile | `ts` del nombre del CSV (`YYYYMMDD_HHMMSS`) | `runs`, `ranking`, `indicadores` (formato largo) |
| EEUU | `run_id` de la carpeta | `runs`, `candidates` (rank por momentum, `in_top`, score y pilares), `prices` (cierres únicos por ticker/fecha) |
| ETF | `run_key` = `yyyymmdd[_n]`, el sufijo con que `archivado` archiva (o archivará) la corrida | `runs`, `ranking`, `momentum`, `excluidos`, `categorias` |
| Tendencia | `run_dir` (`YYYYMMDD_NN`) | las del original + `universe_weights` (v2) |

En ETF la corrida vigente toma `n` = corridas archivadas de esa fecha + 1, así su
clave no cambia al archivarse; re-analizarla (`--solo-analisis`, `POST /run`) la
reemplaza. Solo se guardan corridas terminadas (EEUU: `status: done`).

## Consecuencias

- Los tests de cada módulo verifican que el backfill reproduce los mismos valores que
  los archivos, que es idempotente y que una falla de la base no rompe el pipeline. El
  diff de ETF se valida contra la rotación del original (`rotacion.json`).
- `historial_corridas.csv`, `rotacion.py` y su racha siguen intactos (paridad). La
  versión SQL (`/history/diff`, `/history/top`) generaliza a dos corridas cualesquiera.
- `history.db` entra en el respaldo diario de `data/`. No usar carpetas sincronizadas
  (OneDrive/Dropbox) para `data/`: corrompen los archivos `-wal`/`-shm`.
- Pendiente: incidencias del screener ETF y `ETF_eliminados.csv` en la base, tabla
  `jobs` para `/estado` y la Épica B2 (backtesting sobre `us_stocks.prices` y los
  precios de ETF).
