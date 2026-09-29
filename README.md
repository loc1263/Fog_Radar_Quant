# Fog Radar Quant

Sitio unificado de análisis cuantitativo: ETFs, acciones EEUU, acciones Chile (IPSA)
y tendencia/Markowitz Asia + EEUU. Repositorio privado.

- `backend/`  — FastAPI (Python 3.12, uv)
- `frontend/` — SvelteKit + TypeScript + Tailwind v4 + LayerChart (ver `frontend/README.md`)
- `docs/`     — sistema de diseño, backlog, [metodologías](docs/METODOLOGIAS.md), paridad con los originales
- `deploy/`   — Docker/Caddy para los 2 servidores productivos

Plan y estado: [ROADMAP.md](ROADMAP.md) · Convenciones: [CLAUDE.md](CLAUDE.md)

## Todo junto (desarrollo)

Requiere Python, [uv](https://docs.astral.sh/uv/) y Node.js 22+. La primera vez instala solo lo que falte.

```bash
python iniciar_dev.py              # backend :8000 + frontend :5173
# En Windows también: doble clic en iniciar.bat
```

## Backend (desarrollo)

Requiere [uv](https://docs.astral.sh/uv/) (instala Python 3.12 solo si hace falta).

```bash
cd backend
uv sync                                    # crea backend/.venv con el lock exacto
copy .env.example .env                     # (Windows) y completar; en dev API_KEY puede quedar vacía
uv run uvicorn fog_radar.main:app --reload --port 8000
```

- Documentación interactiva: http://localhost:8000/docs
- Salud: `GET /api/health` (público). El resto exige el header `X-API-Key` si `API_KEY` está definida.
- Pipelines: `uv run python -m fog_radar.jobs --list` · `... jobs cl_stocks` · `... jobs us_stocks` ·
  `... jobs etf [--solo-analisis]` · `... jobs tendencia [--skip-fetch] [--macro] [--backfill] [--top-n N] [--max-weight X]`
- Todos los pipelines en secuencia, a pedido: `python scripts/run_all_pipelines.py` (o doble clic en `correr_pipelines.bat`); en producción los corre el cron (`deploy/backend/README.md`).
- Datos de ejemplo (fixtures de los tests, para probar el sitio sin Yahoo): `uv run python ../scripts/fixtures_e2e.py`
- Calidad: `uv run ruff check . && uv run ruff format --check . && uv run pytest`

## Sitio

- **Inicio (hub):** estado de cada módulo con la fecha de sus datos, si están al día respecto
  de la corrida programada, su top 3 y la próxima corrida.
- **Buscador global** (menú superior o `/buscar?q=`): en qué módulos aparece un ticker, en qué
  posición y con qué puntaje; cada resultado lleva a la vista del módulo ya filtrada.
- **Estado:** jobs recientes y programación de producción.
