# Plan del workflow de GitHub Actions

Fecha: 2026-10-21. Estado: **workflows escritos** (`actualizar-datos.yml` y `publicar.yml`), sin ejecutar todavía en GitHub; faltan el spike de Yahoo y la primera corrida real (sección 9). Continúa [PLAN_GITHUB_PAGES.md](PLAN_GITHUB_PAGES.md) (Fases 0 a 5) y [ADR 0005](ADR/0005-sitio-estatico-con-datos-exportados.md).

## 1. Objetivo

Que GitHub Actions **genere los datos, construya el sitio estático y lo publique en Pages** sin intervención manual, y que el sitio nunca quede roto si un paso falla (se queda la versión anterior).

## 2. Punto de partida (ya existe)

| Pieza | Comando | Notas |
|---|---|---|
| Pipelines | `fogradar job --todos` (o un módulo) | Descarga de Yahoo. Duración orientativa: Chile ~40 s, EEUU ~4 min, ETF 7-13 min, Tendencia varios minutos (la secuencia completa ronda 30 min). |
| Exportación | `fogradar export --out ../frontend/static/data` | Sin red; 30 archivos, ~2 MB. |
| Build del sitio | `BASE_PATH=/<repo> npm run build` en `frontend/` | Salida en `frontend/build` (SPA con `404.html`). |
| CI actual | `.github/workflows/ci.yml` | Backend (fmt, vet, lint, test) y frontend (autofix, check, lint, test). No publica nada. |

No hay remoto configurado todavía: primero hay que crear el repositorio público en GitHub y hacer `git push`.

## 3. Arquitectura propuesta: dos workflows desacoplados

La idea central es **separar "obtener datos" de "publicar"**. Así el riesgo principal (Yahoo bloqueando los runners) no bloquea el resto, y los datos pueden venir también de tu PC.

```
actualizar-datos.yml (cron / manual)          publicar.yml (push a main o a datos / manual)
  jobs: fogradar job ...                        restaurar datos  <------+
  guarda data/ en la rama `datos`  -----------> fogradar export        |
                                                npm ci && npm run build |
  (alternativa: tú subes `datos`                upload-pages-artifact   |
   desde tu PC con los pipelines locales)       deploy-pages            |
```

- **`actualizar-datos.yml`**: compila `fogradar`, restaura `data/`, corre los jobs y publica el resultado en la rama `datos`.
- **`publicar.yml`**: toma `data/` de la rama `datos`, exporta, construye y despliega. Se dispara cuando cambia `main` (código) o `datos` (datos nuevos).
- **`ci.yml`** se mantiene tal cual (validación en PR y push).

Ventaja: si Yahoo bloquea las IPs de Actions, el plan B del doc anterior ("correr los jobs en el PC y subir solo los resultados") es simplemente hacer `git push` a `datos`; `publicar.yml` no cambia.

## 4. Persistencia de `data/` entre corridas

Hace falta conservar `data/` porque los pipelines dependen de lo anterior (la rotación de ETF compara con la corrida previa, Tendencia usa cachés de precios y sectores, y hay cachés de info/históricos que evitan miles de requests). `data/` pesa ~88 MB.

| Opción | Pros | Contras |
|---|---|---|
| **A. Rama `datos` huérfana** (un solo commit, `push --force` en cada corrida) | Durable, visible, sirve de plan B desde el PC, no crece el historial | Los datos quedan públicos (repo público); hay que decidir qué subir |
| B. `actions/cache` | Simple, sin commits | Se evicta tras 7 días sin uso y no es inspeccionable; un fallo de caché rompe la rotación |
| C. Artifacts | Fácil de bajar | Caducan (90 días) y no se restauran solos entre corridas |

**Retención (decidida, 2026-10-21): 7 días.** Los jobs borran solos las corridas más antiguas al terminar cada módulo (`internal/retencion`, `RETENCION_DIAS=7`), así que la rama `datos` se mantiene pequeña sin pasos extra en el workflow. Siempre se conserva la corrida más reciente y `cache/` (incluida el ancla de consistencia de ETF). La estructura de `data/` es la del [ADR 0006](ADR/0006-estructura-estandar-de-data.md).

**Recomendación: A.** Subir `data/<modulo>/runs` (los últimos 7 días) y `data/<modulo>/cache`; excluir `data/_sistema`.

## 5. Disparadores y frecuencia

**Decisión (2026-10-21): la corrida del backend es diaria a las 18:00 (America/Santiago) para los cuatro módulos: Chile, EEUU, ETFs y Tendencia**, en una única secuencia (`fogradar job --todos`). Es lo que declara `core/schedule.go` (única fuente de verdad que usa el hub para decidir si los datos están "al día").

GitHub Actions usa cron en **UTC** y no entiende husos horarios: 18:00 de Santiago son las 21:00 UTC mientras rige el horario de verano de Chile (UTC-3) y las 22:00 UTC en invierno (UTC-4). Opciones: fijar `0 21 * * *` y aceptar que en invierno corra a las 17:00 locales, o declarar los dos cron y que el job salga temprano si no es la hora local correcta.

| Workflow | Disparador | Detalle |
|---|---|---|
| `actualizar-datos.yml` | `schedule` (`0 21 * * *` UTC) + `workflow_dispatch` | Un solo job con la secuencia completa (~30 min). |
| `publicar.yml` | `push` a `main` / `datos` + `workflow_dispatch` | Con `concurrency` para que no se pisen dos despliegues. |

## 6. Pasos de cada workflow

### `actualizar-datos.yml` (un solo job: la secuencia diaria)
1. `actions/checkout` de `main` y de la rama `datos` en `./data`.
2. `actions/setup-go` con `backend-go/go.mod` y caché de módulos.
3. `go build -o fogradar ./cmd/fogradar` con `CGO_ENABLED=0`.
4. `DATA_DIR=$GITHUB_WORKSPACE/data ./fogradar job --todos` (Chile → EEUU → Tendencia → ETFs).
5. Si el job terminó bien: commit único y `git push --force` a `datos`.
6. Si falla: no se sube nada (la rama conserva los datos anteriores) y se avisa (ver 8).

### `publicar.yml`
1. Checkout de `main` y de `datos` en `./data`.
2. `setup-go` + build de `fogradar`.
3. `./fogradar export --out ../frontend/static/data` con `DATA_DIR=../data`.
4. `setup-node` 22 + `npm ci` en `frontend/`.
5. `BASE_PATH=/${{ github.event.repository.name }} npm run build`.
6. **Verificaciones**: que existan `build/404.html` y `build/data/api/hub.json`; que `build/` pese < 1 GB; que no haya `localhost` ni rutas absolutas en el HTML.
7. `actions/upload-pages-artifact` (`frontend/build`) y `actions/deploy-pages`.

Permisos: `contents: write` (solo `actualizar-datos`), `pages: write` e `id-token: write` (solo `publicar`). El repo debe tener **Pages → Source: GitHub Actions**.

## 7. Riesgos

| Riesgo | Mitigación |
|---|---|
| Yahoo responde 429 o bloquea las IPs de Actions (el que puede tumbar el plan) | Spike previo (sección 9); espaciado adaptivo del cliente; plan B: subir `datos` desde el PC |
| Una corrida a medias deja `data/` inconsistente | Subir a `datos` solo si **todos** los módulos del job terminaron bien |
| Dos ejecuciones simultáneas | `concurrency: group` por workflow y `cancel-in-progress: false` |
| El sitio se queda desactualizado sin que nadie lo note | Fecha de generación visible en el hub (ya está) + aviso si falla el cron |
| Cambio de horario desplaza el cron (UTC) | Aceptar 1 h de diferencia en invierno o duplicar el cron con una guarda de hora local |
| `data/` público | Decisión ya tomada (repo público); revisar que no haya claves en `.env` ni logs |

## 8. Avisos de fallo

- GitHub envía un correo cuando falla un workflow programado (comportamiento por defecto).
- Opcional: un paso final `if: failure()` que abra o actualice un issue con el enlace a la corrida (`gh issue create`).

## 9. Fases de implementación y criterios de avance

1. **Spike de Yahoo** (`spike-etf.yml`, solo `workflow_dispatch`): compila, corre `job etf` y registra tiempo, 429 y tamaño. Se corre 3 veces. *Avanzar si:* 3 corridas seguidas sin 429 fatal. *Si falla:* usar solo el plan B (datos desde el PC).
2. **`publicar.yml` con datos subidos a mano** a la rama `datos`: valida el despliegue a Pages de punta a punta sin depender de Yahoo. *Avanzar si:* el sitio abre en `https://<usuario>.github.io/<repo>/` con todas las páginas.
3. **`actualizar-datos.yml`** manual, un módulo por vez (Chile → EEUU → ETF → Tendencia). *Avanzar si:* la segunda corrida reutiliza cachés (al menos 50 % más rápida).
4. **Cron** en horarios reales durante 2 semanas. *Adoptar si:* 0 fallos sin aviso.
5. **Cierre:** actualizar `README.md`, `docs/PLAN_GITHUB_PAGES.md` y `CLAUDE.md`.

## 10. Decisiones abiertas (antes de implementar)

- [ ] ¿Rama `datos` (A, recomendada), `actions/cache` (B) o artifacts (C)?
- [x] Frecuencia: diaria 18:00 para los 4 módulos (decidida).
- [ ] ¿Nombre del repositorio y usuario/organización? (define `BASE_PATH` y la URL de Pages)
- [ ] ¿Aviso por issue además del correo por defecto?
- [ ] ¿Se publican todos los módulos desde el primer despliegue, o primero solo ETF (piloto del plan original)?

## 10 bis. Implementación (2026-10-21)

- **Rama `datos`** (opción A): cada corrida sube `data/` como una sola confirmación huérfana con `push --force`
  (sin `_sistema/logs` ni `locks`; el log se guarda como artefacto 7 días).
- **Hora:** dos cron (21:00 y 22:00 UTC) y un primer paso que deja correr solo el que cae a las 18:00 en Santiago.
- **Fallos parciales:** si un módulo falla, igual se suben y publican los datos de los que terminaron bien y el
  workflow termina en rojo (aviso por correo); el hub marca "atrasado" el módulo que falló.
- **Disparo de la publicación:** un push hecho con `GITHUB_TOKEN` no dispara otros workflows y la rama `datos` no
  tiene workflows, así que `actualizar-datos` llama a `gh workflow run publicar.yml` al terminar. Si subes datos
  desde tu PC a `datos`, dispara la publicación a mano (`gh workflow run publicar.yml`).
- **`BASE_PATH`** sale del nombre del repositorio (`/${{ github.event.repository.name }}`).
- **Para activarlo:** crear el repositorio público, `git push`, y en Settings → Pages elegir *Source: GitHub Actions*.
  La primera vez, lanzar "Actualizar datos" a mano (crea la rama `datos`); luego "Publicar sitio".

## 11. Archivos que se crearán

```
.github/workflows/spike-etf.yml          (no se escribió: el spike es lanzar "Actualizar datos" 3 veces a mano)
.github/workflows/publicar.yml            (escrito)
.github/workflows/actualizar-datos.yml    (escrito)
docs/PLAN_WORKFLOW_ACTIONS.md            (este documento, se actualiza con el resultado de cada fase)
```
