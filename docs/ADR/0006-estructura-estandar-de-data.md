# ADR 0006 — Estructura estándar de `data/`

- **Fecha:** 2026-10-21
- **Estado:** aceptada.

## Contexto

Cada módulo guardaba sus archivos a su manera: Chile en archivos sueltos con timestamp, ETF en `current/` +
`hist/` + `screener/` (con archivado, staging y un candado), EEUU en `runs/<id UTC>/`, Tendencia en `output/`,
`raw/`, `processed/`, `macro/`, `universe/`. Cada convención traía su propia lógica de "última corrida",
retención y lectura, y era difícil saber qué se podía borrar.

## Decisión

Los cuatro módulos usan la misma estructura (helpers en `internal/core/rundir.go`):

```
data/
  <modulo>/                  cl_stocks | us_stocks | etf | tendencia
    runs/
      20261021_180416/       una carpeta por corrida (hora local, YYYYMMDD_HHMMSS; sufijo _n si coincide)
        run.json             marca la corrida como terminada (se escribe al final)
        ...salidas (csv, json) y descargas crudas (raw/, processed/)
    cache/                   estado que se reutiliza entre corridas (info_cache, sector_cache, prices/, fx/, macro/, universe/, ancla de ETF)
  _sistema/
    jobs/    registro de corridas (<modulo>.jsonl)
    logs/    fogradar.log
    locks/   pipelines.lock
```

- **"Última corrida"** = la carpeta más reciente con `run.json` (`core.LatestRun`); "la anterior" = `core.PreviousRun`.
  Una corrida a medias o fallida no tiene `run.json`, así que ningún lector la ve: ya no hacen falta staging ni
  candados de lectura.
- **Retención** (`internal/retencion`): borra carpetas de corrida enteras con más de `RETENCION_DIAS` (7), salvo
  la más reciente, y las incompletas con más de un día. `cache/` no se toca.
- Cada pipeline crea su carpeta con `core.NewRun`, escribe ahí todo y termina con `core.FinishRun`; si falla,
  borra su carpeta.
- ETF: la rotación compara con la corrida anterior; la consistencia del momentum usa un ancla fija en
  `cache/etf_perf_baseline.csv` (el `etf_perf_data` de la primera corrida). `--solo-analisis` crea una corrida
  nueva y reemplaza a la que re-analiza.

## Consecuencias

- Una sola regla para leer, publicar y podar datos; el export y el hub no conocen rutas por módulo.
- Al migrar se regeneró `data/` (se perdieron las corridas previas y el ancla de ETF se reinició).
- Para publicar `data/` en la rama `datos` basta con `cache/` y las carpetas de `runs/` de los últimos 7 días.
