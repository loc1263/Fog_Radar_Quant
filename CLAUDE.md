# Fog_Radar_Quant — Instrucciones para Claude

Monorepo que unifica 4 proyectos cuantitativos (ETF_Choice, Stock_Choice_EEUU,
Stock_Choice_Nacional, Stock_Tendencia) en un solo sitio. Plan completo y
estado de avance: **ROADMAP.md**. Backlog: **docs/BACKLOG.md**.

## Stack (decidido, no relitigar)
- **Frontend:** SvelteKit 2 + **Svelte 5 (runes)** + TypeScript estricto + Tailwind v4 +
  shadcn-svelte + **LayerChart**. `adapter-node` (SSR).
- **Backend:** Python 3.12 + FastAPI, una sola app con un `APIRouter` por módulo
  (`/api/etf`, `/api/us`, `/api/cl`, `/api/tendencia`). Gestor: **uv**.
- **Producción:** 2 servidores separados (frontend Node / backend FastAPI), Docker + Caddy.

## Reglas de Svelte (obligatorias)
1. Usar **solo sintaxis Svelte 5**: `$props()`, `$state`, `$derived`, `$effect`,
   `onclick={...}`, snippets + `{@render}`. **Prohibido**: `export let`, `$:`,
   `on:click`, `<slot>`, `createEventDispatcher`, stores para estado local.
2. Usar el **Svelte MCP** (`https://mcp.svelte.dev/mcp`) para consultar documentación
   (`list-sections` → `get-documentation`) ante cualquier duda de API.
3. **Todo archivo `.svelte` / `.svelte.ts` debe pasar por `svelte-autofixer`** y
   corregirse hasta que no reporte issues antes de darlo por terminado.
4. `npm run check` (svelte-check) sin errores antes de cada commit del frontend.

## Arquitectura front ↔ back (patrón BFF)
- El navegador **nunca** llama a FastAPI directamente. Toda carga de datos va en
  `+page.server.ts` / `+layout.server.ts`; mutaciones en form actions, `+server.ts`
  o remote functions.
- El cliente del backend vive en `src/lib/server/api.ts` y usa
  `$env/dynamic/private` (`PRIVATE_API_URL`, `PRIVATE_API_KEY`).
- Los tipos TS de la API se **generan** desde el OpenAPI de FastAPI
  (`npm run gen:api`); no escribir tipos de respuesta a mano.
- Todo endpoint FastAPI declara `response_model` Pydantic.

## Backend
- Los pipelines (descarga Yahoo, scoring, Markowitz) son **batch por CLI**
  (`python -m fog_radar.jobs <modulo>`); nunca se ejecutan dentro de un request,
  salvo la evaluación en vivo de un solo ticker (módulo EEUU).
- Datos en `data/<modulo>/` (fuera de git). Rutas y secretos vía `settings.py`
  (pydantic-settings) + `.env`.
- Ruff + pytest. Al portar lógica de un proyecto original, agregar **test de
  paridad** (mismo input → mismo output que el original).

## Diseño
- Sistema **"Dashboard ejecutivo"**: `docs/DESIGN_SYSTEM.md`. Todo color sale de
  tokens `brand-*`; nada de colores sueltos ni clases `dark:` en componentes.
- UI en español.

## Proyectos originales
- Viven en `D:\Claude\<Proyecto>` y **no se modifican** durante la migración
  (respaldo en `_backups/`). Se usan solo como referencia de lectura.
