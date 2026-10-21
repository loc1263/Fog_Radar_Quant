# Fog Radar Quant

Sitio unificado de análisis cuantitativo: ETFs, acciones EEUU, acciones Chile (IPSA)
y tendencia/Markowitz Asia + EEUU. Repositorio privado.

- `backend-go/` — API y pipelines en Go (huma + chi, sqlite sin CGO); ver `backend-go/README.md`
- `frontend/`   — SvelteKit + TypeScript + Tailwind v4 + LayerChart (ver `frontend/README.md`)
- `docs/`       — sistema de diseño, backlog, [metodologías](docs/METODOLOGIAS.md)
- `deploy/`     — Docker/Caddy/cron para los 2 servidores productivos

Plan y estado: [ROADMAP.md](ROADMAP.md) · Convenciones: [CLAUDE.md](CLAUDE.md)

## Requisitos

[Go](https://go.dev/) 1.26+ y [Node.js](https://nodejs.org/) 22+ (más Python 3 solo si usas `iniciar.bat`).

## Todo junto (desarrollo)

```bat
iniciar.bat              REM backend :8000 + frontend :5173 (lanza iniciar.py; una consola, con chequeos)
iniciar.bat --rebuild --no-browser   REM opciones: --rebuild, --no-browser, --backend-port, --frontend-port, --timeout
```

Compila el backend si falta el binario o hay fuentes `.go` más nuevas, y levanta ambos servidores.

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
fogradar job --list                       # módulos y programas disponibles
fogradar job cl_stocks                    # un módulo
fogradar job etf --solo-analisis          # opciones propias de cada módulo
fogradar job --todos                      # todos en secuencia (Chile, EEUU, Tendencia, ETF)
fogradar job --programa diario            # el grupo diario (Chile, EEUU y Tendencia); también: semanal (ETFs)
fogradar job --todos --offline            # reutiliza lo ya descargado, sin pedir a Yahoo
fogradar job --todos --no-cache           # ignora cachés y vuelve a pedir todo
```

- En Windows, doble clic en `correr_pipelines.bat` equivale a `job --todos`; compila el binario si falta y acepta los mismos argumentos (`correr_pipelines.bat etf`).
- Opciones por módulo: `etf [--solo-analisis] [--refrescar-info]` · `tendencia [--skip-fetch] [--skip-discovery] [--macro] [--backfill] [--force-discovery] [--top-n N] [--max-weight X]`.
- `--offline` y `--no-cache` son excluyentes. Otras opciones: `--pausa`, `--reintentos`, `--esperar`, `--crontab` (imprime el crontab de producción).
- Solo corre un pipeline a la vez (lock global). Códigos de salida: 0 ok, 1 falla, 2 uso, 3 ocupado.
- Al terminar imprime una tabla resumen (tiempo HH:MM:SS, requests a Yahoo, 429) y los avisos. Detalle completo en `data/_jobs/logs/fogradar.log`.
- Para probar sobre una copia de los datos: `DATA_DIR=/ruta fogradar job ...`. En producción los corre el cron (`deploy/backend/README.md`).

## Frontend

```bash
cd frontend
cp .env.example .env     # (Windows: copy .env.example .env)
npm install
npm run dev              # desarrollo en http://localhost:5173
```

En `.env`, `PRIVATE_API_URL` apunta al backend (`http://localhost:8000`) y `PRIVATE_API_KEY` debe coincidir con `API_KEY` del backend. El backend debe estar corriendo.

Otros comandos:

```bash
npm run build && npm start     # build de producción (adapter-node) y servidor Node
npm run autofix && npm run check && npm run lint && npm test   # validaciones previas a un commit
npm run gen:api                # regenera los tipos TS desde el OpenAPI del backend (backend arriba)
```

## Sitio estático (en migración: ETF y Chile listos)

Objetivo: publicar el sitio como SPA en GitHub Pages ([plan](docs/PLAN_GITHUB_PAGES.md)). Las vistas de **ETF y Chile** ya leen JSON
estático en vez de llamar al backend; EEUU y Tendencia siguen con el backend (BFF) hasta migrarlas.

```bash
# 1. Exportar la instantánea de la API a frontend/static/data (lo hace iniciar.bat solo)
cd backend-go && ./bin/fogradar.exe export --modulo etf,cl --out ../frontend/static/data
# 2. Construir la SPA (BASE_PATH = subruta de Pages; vacío si va en la raíz)
cd ../frontend && STATIC_BUILD=1 BASE_PATH=/Fog_Radar_Quant npm run build     # salida en frontend/build
```

- `fogradar export` llama a la API en proceso y guarda cada GET en `static/data/<ruta>.json` (carpeta fuera de git), más un `manifest.json`.
  Deja también el PDF de ETF, la trayectoria por ticker y el diff entre corridas (ETF: cada una contra la anterior; Chile: todos los pares).
- En Git Bash de Windows, `BASE_PATH=/x` se convierte en una ruta de disco: antepón `MSYS_NO_PATHCONV=1`.
- `404.html` hace de fallback SPA (lo que GitHub Pages sirve para rutas desconocidas).

## Sitio

- **Inicio (hub):** estado de cada módulo con la fecha de sus datos, si están al día respecto
  de la corrida programada, su top 3 y la próxima corrida.
- **Buscador global** (menú superior o `/buscar?q=`): en qué módulos aparece un ticker, en qué
  posición y con qué puntaje; cada resultado lleva a la vista del módulo ya filtrada.
- **Estado:** jobs recientes y programación de producción.
