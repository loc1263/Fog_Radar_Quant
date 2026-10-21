# backend-go — backend de Fog Radar Quant

API HTTP y pipelines batch del sitio, en Go.

## Estado

Sirve las **61 rutas** de `/api` (sistema, hub, `cl`, `us`, `etf`, `tendencia`; incluye
`POST /api/us/portfolio/optimize` y `GET /api/etf/pdf`).

Pipelines batch (`fogradar job <modulo>`; nunca dentro de un request): `cl_stocks`, `us_stocks`, `etf`, `tendencia`.
Tendencia: dataset en USD, Markowitz, proyección a 6 meses, momentum, macro (`--macro`, requiere `FRED_API_KEY`),
informe PDF, `dashboard_data.json` e historial. El universo se redescubre con el screener de Yahoo si su snapshot
tiene más de 3 días. ETF: caché de info y sin gráfico para los que el ranking descarta (ver `internal/modules/etf/infocache.go`).

## Uso

```bash
cd backend-go
go run ./cmd/fogradar api          # sirve en :8000 (ADDR)
go run ./cmd/fogradar openapi      # imprime el OpenAPI generado
go test -race ./...                # tests
golangci-lint run ./cmd/... ./internal/...
```

Configuración: variables de entorno o `backend-go/.env` (ver `.env.example`): `ENV`,
`DATA_DIR`, `TIMEZONE`, `LOG_LEVEL`, `APP_NAME`, `ADDR` (por defecto `:8000`), `ENV_FILE`, `YAHOO_*`,
`PIPELINE_*`. Una variable de entorno gana al `.env`. `DATA_DIR` relativo se resuelve desde el directorio actual
(por defecto `../data`). Reglas de prod: `API_KEY` ≥ 32 caracteres y aleatoria, sin `localhost` en `ALLOWED_ORIGINS`.

## Estructura

```
cmd/fogradar/            main (api | openapi | export | job: --todos, --list, --offline, --no-cache…)
internal/api/            servidor huma+chi (solo lectura, uso local): errores {detail, code}
internal/core/           settings, errores, jobs, schedule (cadencia de actualización), fechas
internal/modules/        registro de módulos; un paquete por módulo (cl, us, etf, tend)
internal/{yahoo,fx,quant,jobs,transversal}  cliente Yahoo común, FX, biblioteca numérica, runner de pipelines y hub
```

## Pipelines

```bash
go run ./cmd/fogradar job --list
go run ./cmd/fogradar job tendencia --skip-fetch --skip-discovery   # offline, sobre raw/ ya descargado
go run ./cmd/fogradar job --todos --offline
go run ./cmd/fogradar job --todos --no-cache     # ignora todas las cachés y pide todo de nuevo a Yahoo
```
Códigos de salida: 0 ok, 1 falla, 2 uso, 3 ocupado (otro pipeline tiene el lock global). Usa `DATA_DIR=/ruta` para correr sobre
una copia de los datos (recomendado al probar). La consola muestra una línea por hito (con la hora), un encabezado `>>> módulo` por
cada módulo y el avance de los recorridos largos como una barra que se actualiza cada 10 % (en vez de una línea por ticker). Al
terminar, imprime un resumen con una tabla (tiempo HH:MM:SS, requests a Yahoo, respuestas 429 y espaciado de cada módulo), el
resultado de cada uno y los avisos. El detalle completo queda siempre en `data/_jobs/logs/fogradar.log`; con `LOG_LEVEL=DEBUG` la
consola también lo muestra todo.
