# ADR 0002 — Convenciones de módulos (revisión de arquitectura tras el piloto Chile)

- **Fecha:** 2026-09-27 (cierre de Fase 3)
- **Estado:** aceptada; aplica a las Fases 4 (EEUU), 5 (ETF) y 6 (Tendencia)

## Contexto

Chile fue el piloto para validar la arquitectura (monorepo, un backend con un router por
módulo, SvelteKit consumiendo su API). Al portarlo aparecieron decisiones que conviene fijar antes de
migrar módulos más grandes.

## Decisiones

### 1. Estructura del módulo en el backend

```
internal/modules/<modulo>/
  scoring.go | scanner.go          lógica pura (sin HTTP ni disco)
  extractor.go                     descarga (Yahoo…); recibe dependencias inyectables
  storage.go                       rutas, "último archivo", validación, escritura atómica
  columnas (si aplica)             catálogo: clave API, etiqueta, grupo, unidad
  schemas.go                       tipos de respuesta
  service.go                       archivo/tabla → schema
  router.go                        solo HTTP (depende de service)
  pipeline.go                      batch; Run(ctx, env, opciones) -> resumen
  *_test.go                        tests con dependencias falsas (sin red)
```

### 2. Formato en disco vs. contrato de la API

- En disco se conserva **el formato del proyecto original** (nombres de columnas, archivos con
  timestamp). Así se copian los históricos sin conversión y los archivos
  existentes se siguen leyendo sin conversión.
- La API expone **claves snake_case estables** y agrupa en objetos (`pilares`, `metricas`).
  Cuando la tabla es dinámica (indicadores), el backend manda el catálogo de columnas con
  su **unidad** (`pct`, `pct_pts`, `multiplo`, `monto`, `fecha`…) y el front formatea por unidad.

- *(Fase 4)* Los modelos de **respuesta** heredan de una base con
  `json_schema_serialization_defaults_required=True`: un campo con valor por defecto
  siempre viene en la respuesta y el tipo TS generado no lo marca como opcional.
  Los parámetros opcionales de **request** se declaran `X | None = None` (null = valor
  por defecto del backend) para no duplicar constantes en el front.

### 3. Parámetros del modelo vienen de la API

Pesos, umbrales y listas de métricas se envían en la respuesta (`modelo`). El front no
duplica constantes del backend (en el original los pesos estaban copiados en dos lugares).

### 4. Fechas y zona horaria

- `Settings.timezone` (por defecto `America/Santiago`): zona de los timestamps en nombres de archivo.
- La API devuelve fechas **con offset** (`2026-09-25T18:31:29-03:00`).
- `fmtDate` usa siempre `TIME_ZONE` → el HTML del SSR y el del navegador coinciden.

### 5. Pipelines

- Registro `PIPELINES: dict[str, Pipeline(run, options)]`; las opciones se vuelven banderas
  del CLI (`--solo-scoring`) y no se aceptan en otros módulos.
- `run()` devuelve un resumen corto que queda en el registro de jobs (`/api/jobs`).
- Los archivos se escriben a `.tmp` y se renombran: la API nunca lee un archivo a medias.
- Re-procesar datos existentes conserva el timestamp de los datos de origen.

### 6. Frontend por módulo

- `src/lib/modules/<modulo>/`: lógica TS pura (con tests contra un JSON real de la
  API en `fixtures/`) y componentes propios. Lo reutilizable sube a `components/shared`
  (así nació la fila desplegable de `DataTable`).
- Tipos solo desde `schema.d.ts`; alias con prefijo de módulo en `api/types.ts`
  (`ClRankingRow`, `UsTicker…`).
- Estado de UI derivado: guardar solo lo que el usuario cambia
  (`pesosUsuario = $state<Pesos | null>(null)`, `pesos = $derived(pesosUsuario ?? defaults)`),
  sin copiar props a `$state`.
- Cargas con `datos()` (`$lib/datos`): sin datos (`sin_datos`) o con error de lectura la página
  muestra `ErrorState`, no tumba el sitio.

### 7. Tests E2E con datos

CI copia los fixtures del módulo a `data/<modulo>/` antes de levantar el backend, para que
las pruebas E2E recorran las páginas con datos reales.

## Consecuencias

- Cada módulo nuevo se puede revisar contra esta lista.
- El costo es algo más de archivos por módulo; a cambio, la lógica queda aislada y
  verificable sin red.
