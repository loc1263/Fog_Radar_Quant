# ADR 0001 — BFF con load functions (no remote functions, por ahora)

- **Fecha:** 2026-09-27 (Fase 2)
- **Estado:** aceptada; revisada en la Fase 4 (2026-09-27): remote functions siguen experimentales en SvelteKit 2.63 → EEUU usa form actions + `use:enhance`. Revisar de nuevo en la Fase 8.

## Contexto

El frontend SvelteKit actúa como BFF: el navegador nunca llama a FastAPI. Hay dos
formas de mover datos entre el servidor SvelteKit y el navegador:

1. **Load functions + form actions / `+server.ts`** (estables desde SvelteKit 1).
2. **Remote functions** (`query`, `form`, `command`, `prerender` en archivos `.remote.ts`),
   tipadas de punta a punta y con deduplicación y *single-flight mutations*.

Según la documentación oficial (consultada vía Svelte MCP), las remote functions siguen
siendo **experimentales**: requieren `kit.experimental.remoteFunctions` y
`compilerOptions.experimental.async`, y "pueden tener bugs y cambiar sin aviso".

## Decisión

- **Fases 2 y 3:** usar **load functions** (`+page.server.ts` / `+layout.server.ts`) con el
  cliente `backend(event)` de `$lib/server/api.ts`. Las páginas de los módulos son de solo
  lectura, y las load functions cubren el caso completo con SSR.
- **Fase 4 (EEUU):** evaluar `command`/`query` de remote functions para las interacciones
  (evaluar ticker en vivo, optimizar portafolio, mejor combinación). Si siguen siendo
  experimentales, usar **form actions** con `use:enhance`.
- Cualquier remote function que se adopte llama al backend a través del mismo
  `$lib/server/api.ts` (API key + `X-Forwarded-For`) y valida sus argumentos con un schema
  (Valibot), porque expone un endpoint HTTP.

## Consecuencias

- Ninguna dependencia de APIs experimentales en el núcleo del sitio.
- Migrar después a remote functions es local a cada página: el cliente del backend no cambia.
