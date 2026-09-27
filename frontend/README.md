# Fog Radar Quant — frontend

SvelteKit 2 + Svelte 5 (runes) + TypeScript + Tailwind v4 + shadcn-svelte + LayerChart,
con `adapter-node` (SSR). Convenciones en [`../CLAUDE.md`](../CLAUDE.md) y diseño en
[`../docs/DESIGN_SYSTEM.md`](../docs/DESIGN_SYSTEM.md).

## Desarrollo

```bash
npm install
copy .env.example .env        # (Windows) PRIVATE_API_URL / PRIVATE_API_KEY
npm run dev                   # http://localhost:5173 (el backend debe estar en :8000)
```

O levantar backend + frontend juntos desde la raíz: doble clic en `iniciar.bat` o `python iniciar_dev.py`.

- Vitrina de componentes (solo dev): http://localhost:5173/componentes

## Comandos

| Comando                       | Qué hace                                                                  |
| ----------------------------- | ------------------------------------------------------------------------- |
| `npm run check`               | svelte-check (tipos)                                                      |
| `npm run lint`                | Prettier + ESLint                                                         |
| `npm run autofix`             | Autofixer oficial de Svelte (el del Svelte MCP) sobre todos los `.svelte` |
| `npm test`                    | Tests unitarios (Vitest: lógica en Node + componentes en Chromium)        |
| `npm run test:e2e`            | E2E de humo con Playwright (compila y levanta `node build`)               |
| `npm run gen:api`             | Regenera `src/lib/api/schema.d.ts` desde el OpenAPI del backend           |
| `npm run build` / `npm start` | Build de producción / servidor Node                                       |

## Arquitectura (BFF)

El navegador solo habla con este servidor. Los datos se cargan en `+page.server.ts`
con `backend(event)` de `src/lib/server/api.ts`, que agrega la API key y la IP del
usuario (`X-Forwarded-For`). Los tipos de respuesta vienen del OpenAPI de FastAPI.

```
src/
  app.css                 tokens brand-* / viz-* + puente a shadcn
  hooks.server.ts         cabeceras de seguridad + IP del cliente
  lib/server/api.ts       cliente del backend (server-only)
  lib/api/                tipos generados (schema.d.ts) + alias
  lib/components/ui/      shadcn-svelte (copiados, adaptados al diseño)
  lib/components/shared/  DataTable, KpiCard, badges, estados…
  lib/charts/             LineChart, BarChart, ScatterChart, Sparkline (LayerChart)
  routes/                 /, /chile, /eeuu, /etf, /tendencia, /estado
```
