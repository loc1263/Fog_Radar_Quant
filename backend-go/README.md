# backend-go — backend Go de Fog Radar Quant

Migración por fases del backend Python (`../backend`) a Go. Plan: `../roadmap_go.md`. Línea base: `../docs/BASELINE_GO.md`.
Mientras dure la migración Python sigue siendo el oráculo y sirve todo lo que Go aún no migró (Caddy enruta por
prefijo: `../deploy/caddy/Caddyfile.migracion`).

## Estado (Fases 1–8)

Go sirve las **61 rutas** de `/api` con el **mismo contrato** que FastAPI (sistema, hub, buscador, `cl`, `us`, `etf`,
`tendencia`; incluye `POST /api/us/portfolio/optimize` y `GET /api/etf/pdf`).

Pipelines batch (`fogradar job <modulo>`; nunca dentro de un request): `cl_stocks`, `us_stocks`, `etf`, `tendencia`.
Tendencia: dataset en USD, Markowitz, proyección a 6 meses, momentum, macro (`--macro`, requiere `FRED_API_KEY`),
informe PDF, `dashboard_data.json` e historial. Sin screener de universo (ver `../docs/PARIDAD_GO.md`).

## Uso

```bash
cd backend-go
go run ./cmd/fogradar api          # sirve en :8000 (ADDR)
go run ./cmd/fogradar openapi      # imprime el OpenAPI generado
go test -race ./...                # tests
golangci-lint run ./cmd/... ./internal/...
```

Configuración: las **mismas variables** que `backend/.env` (`ENV`, `API_KEY`, `ALLOWED_ORIGINS`, `DATA_DIR`,
`TIMEZONE`, `RATE_LIMIT_DEFAULT`, `LOG_LEVEL`, `APP_NAME`) más `ADDR` (por defecto `:8000`) y `ENV_FILE`. Lee el entorno y,
como respaldo, `./.env` o `../backend/.env`. `DATA_DIR` relativo se resuelve desde el directorio actual (por defecto `../data`).
Se mantienen las reglas de prod: `API_KEY` ≥ 32 caracteres y aleatoria, sin `localhost` en `ALLOWED_ORIGINS`.

Docker (contexto `backend-go/`): `docker build -t fogradar-api backend-go` → imagen distroless con un binario estático (~25 MB).

## Estructura

```
cmd/fogradar/            main (api | openapi | job: --todos, --list, --crontab, --offline, --backfill…)
internal/api/            servidor huma+chi: errores {detail, code}, CORS, rate limit, API key
internal/core/           settings, errores, ratelimit (+Gate = limite_concurrente), jobs, histdb, schedule, fechas
internal/modules/        registro de módulos; un paquete por módulo (cl, us, etf, tend)
internal/{yahoo,fx,quant,jobs,transversal}  cliente Yahoo común, FX, biblioteca numérica, runner de pipelines, hub y buscador
golden/                  oráculo de paridad (Fase 0) y comparadores de la Fase 1
spikes/                  experimentos de la Fase 0 (módulos Go independientes)
```

## Verificar paridad con Python

Con el servidor Go levantado (`go run ./cmd/fogradar api`), desde `../backend`:

```bash
uv run python ../backend-go/golden/compare.py       # mismas respuestas JSON (110 caminos)
uv run python ../backend-go/golden/openapi_diff.py  # misma forma del OpenAPI (61 rutas)
```

## Diferencias conocidas con FastAPI (intencionales)

- Floats: Python imprime `5.0` y Go `5`; el valor JSON es el mismo (los comparadores comparan por valor).
- Textos de error 422: el `code` (`validation_error`) y el status coinciden; el `detail` lo redacta huma
  (`path.ticker: expected string to match pattern …`) en vez de pydantic. El front solo usa `code`.
- El 422 documentado en el OpenAPI usa el esquema `ErrorResponse` en vez de `HTTPValidationError`.
- Rate limit y `Gate` son en memoria por proceso, igual que en Python.

## Pipelines

```bash
go run ./cmd/fogradar job --list
go run ./cmd/fogradar job tendencia --skip-fetch --skip-discovery   # offline, sobre raw/ ya descargado
go run ./cmd/fogradar job --todos --offline
go run ./cmd/fogradar job --crontab
```
Códigos de salida: 0 ok, 1 falla, 2 uso, 3 ocupado (otro pipeline tiene el lock global). Usa `DATA_DIR=/ruta` para correr sobre
una copia de los datos (recomendado al probar).
