# ADR 0004 — Backend en Go (estrangulador)

**Estado:** propuesto (la migración de código está hecha; falta el corte, ver `roadmap_go.md` §13).

## Contexto
El backend Python/FastAPI funciona, pero los pipelines tardan (EEUU ≈ 12,7 min, Chile 115 s), la memoria por proceso es alta y
el despliegue exige Python + uv. El cuello de botella principal es Yahoo (espera de red), no la CPU.

## Decisión
Reescribir el backend en Go (`backend-go/`: huma sobre chi, sqlite sin CGO, `gohoo` para Yahoo, gonum) **conservando el
contrato**: mismas rutas, mismo OpenAPI, mismo envelope de error, mismos archivos en `data/` y el mismo esquema de `history.db`.
Python sigue siendo el oráculo de paridad hasta el corte. Se migra por prefijo con Caddy (`deploy/caddy/Caddyfile.migracion`),
con *fallback* a Python; el frontend (BFF) no cambia.

## Consecuencias
- Python y Go pueden coexistir sobre los mismos datos: comparten los locks (`.pipelines.lock`, `current/.datos.lock`,
  `<archivo>.jsonl.lock`) y las migraciones de `history.db` (Go nunca edita una publicada).
- Binario estático (~25 MB, imagen distroless), pipelines más rápidos (Chile 43 s, EEUU ≈ 5 min).
- Diferencias numéricas y de formato documentadas en `docs/PARIDAD_GO.md`.
- Queda en Python (hasta portarlo): el screener de universo de Tendencia.
- Reglas del `CLAUDE.md` (Backend) que cambian al cortar: gestor `uv`/Ruff/pytest → `go`/`golangci-lint`/`go test`; CLI
  `python -m fog_radar.jobs` → `fogradar job`. Las reglas de contrato, datos, locks, rate limit y paridad se mantienen.

## Alternativas
Ver `docs/ANALISIS_MIGRACION_GO.md` (análisis previo a la decisión).
