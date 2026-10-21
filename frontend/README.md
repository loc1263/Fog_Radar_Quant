# Fog Radar Quant — frontend

SvelteKit 2 + Svelte 5 (runes) + TypeScript + Tailwind v4 + shadcn-svelte + LayerChart,
como SPA estática (`adapter-static`). Convenciones en [`../CLAUDE.md`](../CLAUDE.md) y diseño en
[`../docs/DESIGN_SYSTEM.md`](../docs/DESIGN_SYSTEM.md).

## Desarrollo

```bash
npm install
npm run dev                   # http://localhost:5173
```

El sitio lee JSON de `static/data/` (fuera de git), que genera el backend:
`cd ../backend-go && go run ./cmd/fogradar export --out ../frontend/static/data`.
O levantar todo desde la raíz: doble clic en `iniciar.bat` (exporta y arranca).

- Vitrina de componentes (solo dev): http://localhost:5173/componentes

## Comandos

| Comando            | Qué hace                                                                  |
| ------------------ | ------------------------------------------------------------------------- |
| `npm run check`    | svelte-check (tipos)                                                      |
| `npm run lint`     | Prettier + ESLint                                                         |
| `npm run autofix`  | Autofixer oficial de Svelte (el del Svelte MCP) sobre todos los `.svelte` |
| `npm test`         | Tests unitarios (Vitest: lógica en Node + componentes en Chromium)        |
| `npm run test:e2e` | E2E de humo con Playwright (compila y sirve el sitio estático)            |
| `npm run gen:api`  | Regenera `src/lib/api/schema.d.ts` desde el OpenAPI del backend           |
| `npm run build`    | Build estático en `build/` (`BASE_PATH=/subruta` para GitHub Pages)       |

## Arquitectura (sitio estático, ADR 0005)

No hay servidor: cada página carga sus datos en `+page.ts` / `+layout.ts` con `datos(fetch, ruta)`
de `src/lib/datos.ts`, que lee `static/data/<ruta>.json` con el mismo tipo que la API del backend.

```
src/
  app.css                 tokens brand-* / viz-* + puente a shadcn
  lib/datos.ts            lectura tipada de los JSON exportados
  lib/api/                tipos generados (schema.d.ts) + alias
  lib/components/ui/      shadcn-svelte (copiados, adaptados al diseño)
  lib/components/shared/  DataTable, KpiCard, badges, estados…
  lib/charts/             LineChart, BarChart, ScatterChart, Sparkline (LayerChart)
  routes/                 /, /chile, /eeuu, /etf, /tendencia, /estado
```
