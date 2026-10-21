# Hoja de ruta: Fog Radar Quant como web estática en GitHub Pages

Fecha: 2026-10-20. Estado: propuesta, sin implementar.

## Objetivo y alcance

Publicar Fog Radar Quant como un sitio estático en GitHub Pages, regenerado por GitHub Actions, y decidir con evidencia si vale la pena antes de comprometer el refactor completo.

**Dentro del alcance**

- Los cuatro módulos (ETF, EEUU, Chile, Tendencia) en modo solo lectura, con los datos de la última corrida.
- Ejecución de `fogradar job <modulo>` en Actions por cron y manual.
- Prerender del front con `adapter-static` contra la API levantada en el runner.

**Fuera del alcance (en la versión estática)**

- Evaluación en vivo de un ticker libre (módulo EEUU).
- Form actions, rutas `+server.ts` y escrituras desde la API.
- PDFs bajo demanda (se generan en el job y se publican como archivos).

**Restricción de partida:** el backend se queda en Go. Cambiar de lenguaje no resuelve ninguna limitación de Pages.

## Arquitectura propuesta

Actions genera los datos, levanta la API dentro del runner, prerenderiza el front contra ella y Pages recibe solo archivos.

```
1. Disparador        cron semanal o manual, un workflow por módulo
        |
2. Jobs de datos     fogradar job por módulo, datos en caché entre corridas
        |
3. API en el runner  fogradar api en localhost, API key como secret
        |
4. Prerender         SvelteKit adapter-static, llama a la API al construir
        |
5. Artefacto         HTML, JSON y PDFs, sin backend ni API key
        |
6. GitHub Pages      actions/deploy-pages, sitio de solo lectura
```

La API key existe únicamente como secret del runner y no debe aparecer en el HTML publicado; se verifica en la Fase 2.

## Fases

Seis fases, cada una cerrada por una puerta de decisión (go / no-go). Los criterios están en la sección de evaluación.

1. **Fase 0 · Spike.** Workflow mínimo que compila `fogradar`, ejecuta `job etf` y registra tiempos, errores 429 y tamaño de la salida. Entregable: informe de tres corridas.
2. **Fase 1 · Datos.** Elegir la persistencia (`actions/cache`, artifact o rama `data`) y definir qué se publica (resultados) y qué no (`history.db`, descargas brutas).
3. **Fase 2 · Prerender del módulo piloto.** `adapter-static` con `prerender = true`, `entries()` para cada ruta dinámica y `paths.base` para el subdirectorio de Pages. Revisar enlaces rotos y que el HTML no contenga la API key.
4. **Fase 3 · Los cuatro módulos.** Replicar el patrón en EEUU, Chile y Tendencia, y en el hub. Retirar o reemplazar las acciones dinámicas.
5. **Fase 4 · Funciones sin servidor.** Generar los PDFs en el job y mostrar la fecha de la última corrida en el pie.
6. **Fase 5 · Automatización y cierre.** Cron semanal, aviso si el workflow falla, ADR nuevo en `docs/ADR/` y actualización de `docs/METODOLOGIAS.md` si cambia alguna lógica.

## Riesgos y mitigaciones

El riesgo que puede tumbar todo el plan es Yahoo bloqueando los runners; por eso se prueba primero, en la Fase 0.

| Riesgo | Impacto | Mitigación | Se detecta con |
| --- | --- | --- | --- |
| Yahoo responde 429 o bloquea las IPs de Actions | Alto: sin datos no hay sitio | Espaciado y reintento del cliente común; lotes pequeños; plan B: ejecutar los jobs en el PC o en un VPS y subir solo los resultados | Spike de la Fase 0 |
| Los datos de `data/<modulo>/` no persisten entre corridas | Alto: cada corrida descarga todo de nuevo | `actions/cache`, artifact o rama `data` con los resultados; caché de históricos de `Client.History` | Fase 1 |
| El prerender no encuentra todas las rutas (tickers, ETF, sectores) | Medio: páginas 404 | `entries()` por ruta dinámica y chequeo de enlaces rotos en CI | Fase 2 |
| Repo público expone código y datos | Medio: depende de lo que se quiera ocultar | Decidir antes de publicar; repo privado exige un plan de pago | Decisión previa |
| Job supera 6 horas o el sitio supera 1 GB | Medio | Un workflow por módulo; no publicar `history.db` ni brutos | Métricas de Fase 4 |
| Se filtra la API key en el build | Bajo: solo existe en el runner | Secret de Actions; verificar que no quede en el HTML publicado | Revisión de Fase 2 |
| El sitio queda desactualizado sin que nadie lo note | Bajo | Mostrar la fecha de la última corrida en el pie; alerta si el cron falla | Fase 5 |

## Criterios de evaluación (go / no-go)

Cada fase termina con una decisión explícita; si un criterio crítico falla, se detiene el plan y se pasa a las alternativas. Los umbrales son un punto de partida para discutir; ajustarlos antes de empezar la Fase 0.

| Fase | Criterio de éxito | Umbral sugerido | Decisión | Resultado |
| --- | --- | --- | --- | --- |
| 0. Spike | Un job ETF completa en Actions con datos reales de Yahoo | 3 corridas seguidas sin 429 fatal | Seguir / abortar | Pendiente |
| 1. Datos | Los resultados persisten y la 2.ª corrida reutiliza caché | 2.ª corrida al menos 50 % más rápida | Seguir / ajustar | Pendiente |
| 2. Prerender | Todas las rutas del módulo ETF generan HTML sin errores | 0 enlaces rotos; sin secretos en el HTML | Seguir / ajustar | Pendiente |
| 3. Módulos | Los 4 módulos publican y se ven igual que en local | Revisión visual y `npm run check` en verde | Seguir / ajustar | Pendiente |
| 4. Funciones | Los PDFs funcionan sin servidor | Sitio < 1 GB | Seguir / recortar | Pendiente |
| 5. Automatización | Cron semanal estable durante 4 semanas | 0 fallos sin aviso | Adoptar / descartar | Pendiente |

## Decisiones tomadas (2026-10-21)

- Se eliminó todo el BFF (servidor de frontend, `hooks.server.ts`, `lib/server`, `adapter-node`, `PRIVATE_API_*`).
- Repo **público**; módulo piloto **ETF**; por ahora solo desarrollo local (sin Actions ni Pages reales).
- **SPA pura con JSON estático** (no prerender con BFF; ver [ADR 0005](ADR/0005-sitio-estatico-con-datos-exportados.md)): `fogradar export` genera `frontend/static/data/` y las páginas usan `+page.ts` con `$lib/datos`.
- Implementado: `fogradar export`, `adapter-static` (`BASE_PATH`) y todas las vistas: ETF, Chile, EEUU, Tendencia (con sus descargas: PDF y CSV de la última corrida), hub y Estado (instantánea de la exportación). Pendiente: los workflows de GitHub Actions y la publicación en Pages.
- EEUU en la versión estática: sin evaluación en vivo de un ticker ni optimizador a pedido; el portafolio es el **precalculado** del batch (`GET /api/us/portfolio/precalculado`: todos los candidatos, Top 30 y Top 15 con los parámetros por defecto). Las sparklines usan `prices` de los últimos 60 días (el export fija `days=60`).
- **Sin búsquedas (2026-10-21):** se eliminaron el buscador global (menú, hub, `/buscar`, `GET /api/buscar` y su índice) y los filtros de texto de las tablas; las tablas conservan orden, paginación y CSV.
- **Sin historial (2026-10-21):** se eliminaron las vistas de historial de todos los módulos (Chile, EEUU, ETF —Historial y Evolución— y Tendencia). La API `/history/*` y `history.db` siguen en el backend (ADR 0003), pero el export no las incluye.

## Decisiones abiertas y alternativas

**Decisiones que faltan antes de la Fase 0**

- [ ] ¿Repo público o privado? Define si Pages es gratis y qué datos quedan visibles.
- [ ] ¿Qué funciones se aceptan perder: evaluación en vivo, PDFs bajo demanda, escrituras?
- [ ] ¿Con qué frecuencia se actualiza: diaria, semanal o manual?
- [ ] ¿Se mantiene el sitio con servidor (Docker + Caddy) en paralelo, o la versión estática lo reemplaza?
- [ ] ¿Qué módulo se usa como piloto? Se sugiere ETF (a confirmar).

**Si la vía estática se descarta**

- Desplegar el front Node y el backend Go como contenedores en un servicio (Fly.io, Render, Railway o un VPS), como ya plantea el despliegue con Docker + Caddy.
- Front estático en Pages o Cloudflare Pages, con el backend Go en un servidor aparte. Exige exponer la API al navegador, así que la API key quedaría visible.
- Mantener Actions solo para CI y los jobs programados, y publicar los resultados en otro destino.
