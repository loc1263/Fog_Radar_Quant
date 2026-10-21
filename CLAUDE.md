# Fog_Radar_Quant — Instrucciones para Claude

Monorepo que unifica 4 proyectos cuantitativos (ETF_Choice, Stock_Choice_EEUU,
Stock_Choice_Nacional, Stock_Tendencia) en un solo sitio. Plan completo y
estado de avance: **ROADMAP.md**. Backlog: **docs/BACKLOG.md**. Cómo calcula cada
módulo: **docs/METODOLOGIAS.md** (actualizarla si cambia la lógica).

## Stack (decidido, no relitigar)
- **Frontend:** SvelteKit 2 + **Svelte 5 (runes)** + TypeScript estricto + Tailwind v4 +
  shadcn-svelte + **LayerChart**. `adapter-static` (SPA estática, sin servidor).
- **Backend:** Go 1.26 (`backend-go/`): un solo binario `fogradar` con la API (huma sobre chi,
  rutas `/api/etf`, `/api/us`, `/api/cl`, `/api/tendencia`) y los pipelines batch (`fogradar job`).
- **Publicación:** sitio estático en GitHub Pages; los datos los genera el backend (`fogradar export`).

## Reglas de Svelte (obligatorias)
1. Usar **solo sintaxis Svelte 5**: `$props()`, `$state`, `$derived`, `$effect`,
   `onclick={...}`, snippets + `{@render}`. **Prohibido**: `export let`, `$:`,
   `on:click`, `<slot>`, `createEventDispatcher`, stores para estado local.
2. Usar el **Svelte MCP** (`https://mcp.svelte.dev/mcp`) para consultar documentación
   (`list-sections` → `get-documentation`) ante cualquier duda de API.
3. **Todo archivo `.svelte` / `.svelte.ts` debe pasar por `svelte-autofixer`** y
   corregirse hasta que no reporte issues antes de darlo por terminado. En local:
   `npm run autofix` (mismo autofixer del MCP, sin red; omite `components/ui`).
4. Antes de cada commit del frontend: `npm run autofix && npm run check && npm run lint && npm test`.
5. Enlaces internos con `resolve()` de `$app/paths`; externos con `href` literal. Para hrefs que
   vienen de una unión de rutas (menús), `resolve(ruta(href))` de `$lib/nav-href` (con > 25 rutas
   TypeScript no resuelve la sobrecarga).
6. Componentes base de shadcn-svelte en `src/lib/components/ui` (copiados de la versión
   1.0.12, adaptados a los tokens); los propios del sitio en `components/shared` y
   gráficos en `src/lib/charts` (LayerChart).

## Arquitectura front ↔ back (sitio estático, ADR 0005)
- No hay servidor de frontend: el backend exporta su API a JSON (`fogradar export` →
  `frontend/static/data/`, fuera de git) y el sitio lo lee. El navegador no llama a ninguna API.
- Toda carga de datos va en `+page.ts` / `+layout.ts` con `datos(fetch, ruta)` de `$lib/datos.ts`
  (tipado por ruta). **Prohibido** `+page.server.ts`, `+server.ts`, form actions y remote functions.
- Lo que el sitio necesite de un endpoint nuevo se agrega en el backend (sin parámetros obligatorios,
  para que el export lo incluya) y se regenera la instantánea.
- Los tipos TS de la API se **generan** desde el OpenAPI del backend
  (`npm run gen:api`); no escribir tipos de respuesta a mano.
- Todo endpoint del backend declara su tipo de respuesta (el OpenAPI sale de ahí).

## Backend
- Los pipelines (descarga Yahoo, scoring, Markowitz) son **batch por CLI**
  (`fogradar job <modulo>`); nunca se ejecutan dentro de un request.
- Datos en `data/<modulo>/` (fuera de git). Rutas y secretos vía `internal/core/settings.go`
  (variables de entorno + `backend-go/.env`).
- Calidad: `gofmt`, `go vet`, `golangci-lint`, `go test -race`. Toda lógica nueva lleva tests.
- PDFs: estilo común en `internal/pdf` (paleta, Carlito incluida, tablas, encabezado/pie).
  Textos ≥ 10 pt, encabezados de tabla claros (nunca fondos oscuros), números a la derecha.
- **Yahoo Finance:** todo pasa por el cliente común de `internal/yahoo` (espaciado
  global, reintento ante 429). Históricos con caché: `Client.History`.
- **FX:** `internal/fx` (spot `Rates`, series `ToUSD`, cualquier moneda ISO 4217 y
  subunidades GBp/ZAc/ILA). No agregar tablas de monedas por módulo.
- La API (`fogradar api`) es de solo lectura y de uso local (`/docs`, `gen:api`, `export`): sin
  autenticación, rate limit ni CORS. Toda la salida del sitio pasa por `fogradar export`.
- Las secuencias de pipelines toman el lock global (`core.LockPipelines`): nunca hay dos a la vez.
- Cada corrida deja su detalle en `data/_jobs/logs/fogradar.log` y un resumen (tiempo HH:MM:SS y avisos).

## Convenciones de módulos (ADR 0002)
- Backend: `internal/modules/<modulo>/` con scoring/scanner, extractor, storage, schemas, service, router y pipeline.
- En disco, el formato del proyecto original; en la API, claves snake_case. Parámetros del
  modelo (pesos, umbrales) viajan en la respuesta: el front no los duplica.
- Fechas con zona (`Settings.Loc`); en el front, `fmtDate` (zona fija).
- Cada módulo expone `hub.go` (resumen del hub) para el hub (`internal/transversal`, `GET /api/hub`).
- Front: `src/lib/modules/<modulo>/` para lógica y componentes propios del módulo.
- Las diferencias de lógica respecto del proyecto original se documentan en `docs/METODOLOGIAS.md`.

## Diseño
- Sistema **"Dashboard ejecutivo"**: `docs/DESIGN_SYSTEM.md`. Todo color sale de
  tokens `brand-*`; nada de colores sueltos ni clases `dark:` en componentes.
- UI en español.

## Proyectos originales
- Viven en `D:\Claude\<Proyecto>` y **no se modifican**. Se usan solo como referencia de lectura.
