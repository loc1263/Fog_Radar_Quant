# Pendientes tras la incorporación de SQLite (Épica B1)

*Fecha: 2026-10-04 · Contexto: `INFORME_SQLITE.md`, `docs/ADR/0003-historial-sqlite-por-modulo.md`,
`docs/BACKLOG.md` (Épica B1).*

## 1. Antes de cerrar este cambio

- [ ] **Commit**: los cambios están sin commitear (backend, frontend, docs y tests nuevos).
- [ ] **`.git/index.lock` huérfano** (vacío, del 29-09). Bloquea `git stash` y probablemente
      otras operaciones de git. Si no hay otro git corriendo, borrarlo.
- [ ] **Correr los E2E nuevos** (no se ejecutaron): `chile.e2e.ts` (historial),
      `eeuu.e2e.ts` (historial) y `etf.e2e.ts` (evolución), con datos reales y `fogradar job <modulo> --backfill`.
- [ ] **Revisión visual** de `/chile/historial`, `/eeuu/historial`, `/etf/evolucion` y del gráfico
      nuevo en `/tendencia/historial` (se verificó que cargan con datos reales, pero no se
      revisaron en el navegador).
- [ ] Respaldo de la base de Tendencia previo a la migración v2: está en el scratchpad de la
      sesión (temporal). Si se quiere conservar, copiarlo a `_backups/`.

## 2. Pendientes de la Épica B1 (anotados en el BACKLOG)

| ID | Ítem | Prioridad |
|---|---|---|
| B1-ETF-INC | Guardar en `etf/history.db` las incidencias del screener (`incidencias_*.csv`) y `ETF_eliminados.csv`, para detectar tickers que fallan N corridas seguidas y proponer su eliminación automáticamente. | Media |
| B1-ETF-SQL | Reescribir la racha y la rotación de la rotación del ETF (`rotacion.go`) sobre SQL, con un test que demuestre el mismo resultado que la versión con CSV. Ojo: la racha del original agrupa por `Fecha` (las corridas `_2`/`_3` del mismo día cuentan una vez) y `/history/top` las cuenta por separado. | Baja |
| B1-JOBS | Tabla `jobs` (duración, requests a Yahoo, respuestas 429 y reintentos por corrida) en lugar o además de `data/_jobs/<modulo>.jsonl`, con gráfico en `/estado`. | Baja (opcional) |
| — | `en_top15_simple` no se guarda en ETF: el Top 15 simple no se archiva en `hist/`. Hoy solo se guarda `score_simple`; el puesto se puede calcular por SQL si hace falta. | Baja |
| — | Chile: algunas corridas antiguas no tienen `indicadores_acciones_<ts>.csv` con el mismo timestamp que el ranking, así que en esas corridas la tabla `indicadores` queda vacía. | Informativo |

## 3. Épica B2 · Backtesting (desbloqueada por B1)

- [ ] Correlación (Spearman) score vs. retorno a 30/60/90 días (US-DATA-02), usando
      `us_stocks/history.db → prices` y la columna `precio` del ranking de ETF.
- [ ] Decidir con backtest los cambios de metodología pendientes (TD-MODEL-4…9, US-DATA-08).
- [ ] Esperar a acumular corridas: hoy hay 14 corridas de EEUU (desde el 18-09) y 23 de ETF.

## 4. Fallas de tests preexistentes (no causadas por SQLite)

- [ ] **Frontend `format.spec.ts > fechas sin zona`**: diferencia de ICU o locale de Node en
      el formato de hora (`10:11 p. m.`, probablemente un espacio especial). No se tocó `format.ts`.

## 5. Producción (Fase 8, en pausa)

- [ ] Incluir los 4 `data/<modulo>/history.db` en el respaldo diario de `data/`.
- [ ] No poner `data/` en carpetas sincronizadas (OneDrive/Dropbox): corrompen los archivos
      `-wal`/`-shm` de SQLite.
- [ ] En el servidor, después de copiar `data/`, correr `--backfill` de cada módulo si las bases
      no se copiaron (se reconstruyen desde los archivos).
- [ ] SEC-PROD-2: en producción, los errores de SQLite (503 `invalid_data`) deben mostrar un
      mensaje genérico; el detalle va solo al log.
