# ADR 0005 — Sitio estático con datos exportados (sin BFF)

- **Fecha:** 2026-10-21
- **Estado:** aceptada. Reemplaza al ADR 0001 (BFF con load functions), que se eliminó.

## Contexto

El sitio se servía con SvelteKit `adapter-node` (SSR) actuando como BFF: el navegador hablaba solo
con SvelteKit y este llamaba al backend Go con una API key. El objetivo pasó a ser publicarlo en
GitHub Pages ([plan](../PLAN_GITHUB_PAGES.md)), que solo sirve archivos: no hay servidor
intermedio ni API en línea.

## Decisión

- El backend genera una instantánea de su API con `fogradar export`: un JSON por cada GET en
  `frontend/static/data/<ruta>.json` (más los PDF/CSV de la última corrida de Tendencia).
- El frontend es una SPA con `adapter-static` (`fallback: '404.html'`, `BASE_PATH` para la subruta de
  Pages). Cada página carga sus datos en un `+page.ts` / `+layout.ts` con `datos(fetch, ruta)`
  de `$lib/datos.ts`, tipado con el OpenAPI del backend (`npm run gen:api`).
- No hay servidor de frontend, API key en el sitio ni mutaciones: se retiraron los form actions,
  los `+server.ts`, las remote functions y `hooks.server.ts`.
- Lo que requería servidor se quitó o se precalcula: la evaluación en vivo de un ticker, el
  optimizador de portafolio (el sitio muestra los portafolios precalculados), el historial y las
  búsquedas.

## Consecuencias

- Despliegue trivial (archivos estáticos) y sin superficie de ataque del lado del servidor.
- Los datos son una foto de la última exportación; el sitio muestra la fecha de generación.
- La API `fogradar api` sigue existiendo para generar tipos, pruebas y `/docs`, pero el sitio no la usa.
