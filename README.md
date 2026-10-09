# Fog Radar Quant

Sitio unificado de análisis cuantitativo: ETFs, acciones EEUU, acciones Chile (IPSA)
y tendencia/Markowitz Asia + EEUU. Repositorio privado.

- `backend-go/` — API y pipelines en Go (huma + chi, sqlite sin CGO); ver `backend-go/README.md`
- `frontend/`   — SvelteKit + TypeScript + Tailwind v4 + LayerChart (ver `frontend/README.md`)
- `docs/`       — sistema de diseño, backlog, [metodologías](docs/METODOLOGIAS.md)
- `deploy/`     — Docker/Caddy/cron para los 2 servidores productivos

Plan y estado: [ROADMAP.md](ROADMAP.md) · Convenciones: [CLAUDE.md](CLAUDE.md)

## Todo junto (desarrollo)

Requiere [Go](https://go.dev/) 1.26+ y Node.js 22+.

```bat
iniciar.bat              REM backend :8000 + frontend :5173 (dos ventanas)
```

## Backend (desarrollo)

```bash
cd backend-go
cp .env.example .env                       # (Windows: copy) y completar; en dev API_KEY puede quedar vacía
go run ./cmd/fogradar api                  # sirve en :8000
```

- Salud: `GET /api/health` (público). El resto exige el header `X-API-Key` si `API_KEY` está definida.
- Pipelines: `fogradar job --list` · `fogradar job cl_stocks` · `fogradar job us_stocks` ·
  `fogradar job etf [--solo-analisis] [--refrescar-info]` · `fogradar job tendencia [--skip-fetch] [--macro] [--backfill] [--force-discovery] [--top-n N] [--max-weight X]`
- Todos los pipelines en secuencia, a pedido: `fogradar job --todos` (o doble clic en `correr_pipelines.bat`); en producción los corre el cron (`deploy/backend/README.md`).
- Calidad: `gofmt -l cmd internal && go vet ./... && go test -race ./...`
- Log único y detallado de todo el backend: `data/_jobs/logs/fogradar.log`.

## Sitio

- **Inicio (hub):** estado de cada módulo con la fecha de sus datos, si están al día respecto
  de la corrida programada, su top 3 y la próxima corrida.
- **Buscador global** (menú superior o `/buscar?q=`): en qué módulos aparece un ticker, en qué
  posición y con qué puntaje; cada resultado lleva a la vista del módulo ya filtrada.
- **Estado:** jobs recientes y programación de producción.
