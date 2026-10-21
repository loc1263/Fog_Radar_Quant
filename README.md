# Fog Radar Quant

Sitio unificado de análisis cuantitativo: ETFs, acciones EEUU, acciones Chile (IPSA)
y tendencia/Markowitz Asia + EEUU.

- `backend-go/` — API y pipelines en Go (huma + chi, sqlite sin CGO); ver `backend-go/README.md`
- `frontend/`   — SvelteKit + TypeScript + Tailwind v4 + LayerChart (ver `frontend/README.md`)
- `docs/`       — sistema de diseño, backlog, [metodologías](docs/METODOLOGIAS.md)

Plan y estado: [ROADMAP.md](ROADMAP.md) · Convenciones: [CLAUDE.md](CLAUDE.md)

## Requisitos

[Go](https://go.dev/) 1.26+ y [Node.js](https://nodejs.org/) 22+ (más Python 3 solo si usas `iniciar.bat`).

## Todo junto (desarrollo)

```bat
iniciar.bat              REM exporta los datos y levanta el sitio :5173 (lanza iniciar.py; con --api también la API :8000)
iniciar.bat --rebuild --no-browser   REM opciones: --api, --rebuild, --no-browser, --no-kill, --backend-port, --frontend-port, --timeout
```

Compila el backend si falta el binario o hay fuentes `.go` más nuevas, exporta los datos y levanta el sitio (y la API con `--api`). Cierra lo que ocupe los puertos antes de arrancar.

## Estructura de `data/`

Igual para los cuatro módulos (ver [ADR 0006](docs/ADR/0006-estructura-estandar-de-data.md)):

```
data/
  <modulo>/          cl_stocks | us_stocks | etf | tendencia
    runs/<YYYYMMDD_HHMMSS>/   una carpeta por corrida (salidas y descargas crudas); run.json la marca terminada
    cache/                    cachés y series que se reutilizan entre corridas
  _sistema/{jobs,logs,locks}/ registro de corridas, log único y candados
```

## Backend

### Configuración

```bash
cd backend-go
cp .env.example .env     # (Windows: copy .env.example .env) y completar; en dev API_KEY puede quedar vacía
```

### Compilar

```bash
cd backend-go
go build -o bin/fogradar.exe ./cmd/fogradar      # Windows
go build -o bin/fogradar ./cmd/fogradar          # Linux/macOS
```

Genera un único binario `fogradar` (API + pipelines). No requiere CGO.

### Correr

```bash
# con el binario compilado
./bin/fogradar.exe api          # (Linux: ./bin/fogradar api) sirve en :8000

# o directo desde las fuentes, sin compilar
go run ./cmd/fogradar api
```

- Salud: `GET /api/health` (público). El resto exige el header `X-API-Key` si `API_KEY` está definida.
- Calidad: `gofmt -l cmd internal && go vet ./... && go test -race ./...`

### Pipelines (batch)

Descargan datos de Yahoo Finance, calculan scoring/Markowitz y escriben en `data/<modulo>/`. Se corren por CLI, nunca dentro de la API.
Módulos: `cl_stocks`, `us_stocks`, `etf`, `tendencia`.

```bash
# desde backend-go/ (con binario: ./bin/fogradar.exe ...; sin compilar: go run ./cmd/fogradar ...)
fogradar job --list                       # módulos disponibles
fogradar job cl_stocks                    # un módulo
fogradar job etf --solo-analisis          # opciones propias de cada módulo
fogradar job --todos                      # todos en secuencia (Chile, EEUU, Tendencia, ETF)
fogradar job --todos --solo cl_stocks,etf  # solo algunos módulos de la secuencia
fogradar job --todos --offline            # reutiliza lo ya descargado, sin pedir a Yahoo
fogradar job --todos --no-cache           # ignora cachés y vuelve a pedir todo
```

- En Windows, doble clic en `correr_pipelines.bat` equivale a `job --todos`; compila el binario si falta y acepta los mismos argumentos (`correr_pipelines.bat etf`).
- Opciones por módulo: `etf [--solo-analisis] [--refrescar-info]` · `tendencia [--skip-fetch] [--skip-discovery] [--force-discovery] [--top-n N] [--max-weight X]`.
- `--offline` y `--no-cache` son excluyentes. Otras opciones: `--solo mod1,mod2` (con `--todos`) y `--pausa SEGUNDOS` (espera entre módulos si el anterior falló o recibió 429).
- Al terminar cada módulo, los jobs borran las corridas de más de 7 días (`RETENCION_DIAS`; siempre queda la más reciente). A pedido: `fogradar prune [--dias N] [--dry-run]`.
- Solo corre un pipeline a la vez (lock global). Códigos de salida: 0 ok, 1 falla, 2 uso, 3 ocupado.
- La consola usa colores cuando la salida es una terminal (`NO_COLOR=1` los desactiva, `FORCE_COLOR=1` los fuerza).
- Al terminar imprime una tabla resumen (tiempo HH:MM:SS, requests a Yahoo, 429) y los avisos. Detalle completo en `data/_sistema/logs/fogradar.log`.
- Para probar sobre una copia de los datos: `DATA_DIR=/ruta fogradar job ...`.

## Frontend

```bash
cd frontend
npm install
npm run dev              # desarrollo en http://localhost:5173
```

El sitio lee JSON estático de `frontend/static/data/` (no llama a ninguna API): genéralo antes con
`fogradar export` (lo hace `iniciar.bat` solo; ver más abajo). Comandos habituales:

```bash
npm run autofix && npm run check && npm run lint && npm test   # validaciones previas a un commit
npm run gen:api          # regenera los tipos TS desde el OpenAPI del backend (con la API arriba: fogradar api)
```

## Sitio estático (los 4 módulos migrados)

Objetivo: publicar el sitio como SPA en GitHub Pages ([plan](docs/PLAN_GITHUB_PAGES.md)). Todo el sitio (hub, ETF, Chile, EEUU, Tendencia y Estado) lee JSON
estático en vez de llamar al backend; falta publicarlo con GitHub Actions.

```bash
# 1. Exportar la instantánea de la API a frontend/static/data (lo hace iniciar.bat solo)
cd backend-go && ./bin/fogradar.exe export --out ../frontend/static/data
# 2. Construir la SPA (BASE_PATH = subruta de Pages; vacío si va en la raíz)
cd ../frontend && BASE_PATH=/Fog_Radar_Quant npm run build     # salida en frontend/build
```

- `fogradar export` llama a la API en proceso y guarda cada GET en `static/data/<ruta>.json` (carpeta fuera de git), más un `manifest.json`.
  Deja también los precios de las sparklines de EEUU. `--modulo etf,cl` limita qué módulos se exportan. Las rutas `/history/*` no se exportan: el sitio estático no tiene vistas de historial.
- En Git Bash de Windows, `BASE_PATH=/x` se convierte en una ruta de disco: antepón `MSYS_NO_PATHCONV=1`.
- El hub muestra la frescura calculada al exportar, no al abrir la página.
- `404.html` hace de fallback SPA (lo que GitHub Pages sirve para rutas desconocidas).

## Sitio

- **Inicio (hub):** estado de cada módulo con la fecha de sus datos, si están al día respecto
  de la corrida programada, su top 3 y la próxima corrida.
- **Estado:** jobs recientes y programación de producción.
