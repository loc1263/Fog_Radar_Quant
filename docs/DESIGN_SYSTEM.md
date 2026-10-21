# Sistema de diseño: "Dashboard ejecutivo" — Fog Radar Quant

Especificación única de la UI de **Fog Radar Quant** (SvelteKit + Tailwind v4).
Consolidada el 2026-09-27 a partir de las copias de Stock_Tendencia (original),
ETF_Choice y Stock_Choice_EEUU, que eran idénticas en las secciones 1–8.
Stock_Choice_Nacional no la usaba; se adopta al migrarlo.

Todos los valores (colores, tamaños, espaciados) están explícitos para que la
spec sea verificable pantalla por pantalla.

---

## 1. Principios

1. **Sobrio y ejecutivo.** Fondo liso, sin gradientes ni sombras. La
   jerarquía se marca con color de texto, bordes finos y tamaño, no con
   decoración.
2. **Los datos mandan.** El color se usa para **resaltar** datos clave (retornos
   positivos o negativos, señales, métricas principales), nunca para decorar ni para
   "apagar" texto secundario.
3. **Un solo acento cálido.** Ámbar oscuro para títulos de sección y métricas
   destacadas; azul marino para etiquetas, navegación y encabezados de tabla.
4. **Tokens, no colores sueltos.** Todo color sale de una variable
   `--color-brand-*`. El modo oscuro solo redefine esas variables, así que
   los componentes no llevan clases `dark:`.
5. **Ancho útil.** El contenido usa hasta 1600 px. Las tablas no generan scroll
   horizontal si hay pantalla disponible.

---

## 2. Tokens de color

### 2.1 Tabla de tokens

| Token | Uso | Claro | Oscuro |
|---|---|---|---|
| `brand-bg` | Fondo de página, nav, inputs, filas impares | `#FFFFFF` | `#0F141B` |
| `brand-text` | Texto principal | `#000000` | `#E6E9EE` |
| `brand-muted` | Texto secundario (ayudas, estados vacíos) | `#3F4A5A` | `#A9B3C1` |
| `brand-label` | Etiquetas, links del menú, subtítulos | `#1B3A6B` | `#A9C3EC` |
| `brand-strong` | Relleno de encabezado de tabla / ítem activo (texto blanco encima) | `#1B3A6B` | `#25406B` |
| `brand-accent` | Títulos de sección, métrica destacada | `#9A6E00` | `#E3B341` |
| `brand-green` | Valor positivo | `#1A7A55` | `#4CC592` |
| `brand-red` | Valor negativo, errores | `#B83232` | `#F07878` |
| `brand-blue` | Barras de progreso, dato informativo | `#1B5EA5` | `#6AA8EE` |
| `brand-border` | Todos los bordes y divisores | `#D8DCE3` | `#2C3643` |
| `brand-card` | Fondo de tarjetas, filas pares, hover | `#F7F8FA` | `#18202A` |
| `brand-amber-bg` | Fondo del encabezado, cajas de advertencia | `#FDFAF0` | `#1C1810` |
| `brand-blue-bg` | Cajas informativas (notas) | `#F2F6FD` | `#121B28` |
| `brand-neutral` | Badge "sin datos" | `#6B7280` | `#5B6472` |
| `brand-green-solid` | Badge positivo (texto blanco) | `#1A7A55` | *igual* |
| `brand-red-solid` | Badge negativo (texto blanco) | `#B83232` | *igual* |
| `brand-accent-solid` | Badge neutro/mixto (texto blanco) | `#9A6E00` | *igual* |
| `viz-grid` | Grilla de gráficos | `#E1E0D9` | `#2A323D` |
| `viz-axis` | Texto de ejes de gráficos | `#898781` | `#8D96A3` |

**Regla clave:** en modo oscuro los colores "de texto" (`label`, `accent`,
`green`, `red`, `blue`) se **aclaran** para mantener el contraste. Los rellenos
con texto blanco encima (`strong` y los `*-solid`) se mantienen oscuros.
Por eso existen tokens separados para "color como texto" y "color como fondo
de badge".

### 2.2 Paleta categórica para gráficos (series)

Distinta de la paleta de marca. Se usa **en este orden**, porque así los pares
adyacentes se distinguen también con daltonismo:

```
#2a78d6 azul · #eb6834 naranja · #1baf7a aguamarina · #eda100 amarillo
#e87ba4 magenta · #008300 verde · #4a3aa7 violeta · #e34948 rojo
```

Más de 8 series: agrupar el resto en "Otros" en vez de inventar colores.

Para barras de composición (p. ej. asignación por país) se usa la paleta de
marca: `#1B5EA5, #9A6E00, #1A7A55, #B83232, #1B3A6B`.

### 2.3 CSS listo para copiar (Tailwind v4)

```css
@import "tailwindcss";

@theme {
  --color-brand-bg: #FFFFFF;
  --color-brand-text: #000000;
  --color-brand-muted: #3F4A5A;
  --color-brand-strong: #1B3A6B;
  --color-brand-neutral: #6B7280;
  --color-brand-label: #1B3A6B;
  --color-brand-accent: #9A6E00;
  --color-brand-red: #B83232;
  --color-brand-green: #1A7A55;
  --color-brand-blue: #1B5EA5;
  --color-brand-border: #D8DCE3;
  --color-brand-card: #F7F8FA;
  --color-brand-amber-bg: #FDFAF0;
  --color-brand-blue-bg: #F2F6FD;
  --color-brand-green-solid: #1A7A55;
  --color-brand-red-solid: #B83232;
  --color-brand-accent-solid: #9A6E00;
  --color-viz-grid: #e1e0d9;
  --color-viz-axis: #898781;
}

html { color-scheme: light; }

html.dark {
  color-scheme: dark;
  --color-brand-bg: #0F141B;
  --color-brand-text: #E6E9EE;
  --color-brand-muted: #A9B3C1;
  --color-brand-label: #A9C3EC;
  --color-brand-accent: #E3B341;
  --color-brand-red: #F07878;
  --color-brand-green: #4CC592;
  --color-brand-blue: #6AA8EE;
  --color-brand-border: #2C3643;
  --color-brand-card: #18202A;
  --color-brand-amber-bg: #1C1810;
  --color-brand-blue-bg: #121B28;
  --color-brand-strong: #25406B;
  --color-brand-neutral: #5B6472;
  --color-viz-grid: #2A323D;
  --color-viz-axis: #8D96A3;
}

.section-header {
  @apply mb-3 border-b border-brand-border pb-1.5 text-xs font-bold
         uppercase tracking-wide text-brand-accent;
}
```

**Sin Tailwind:** usa las mismas variables en `:root` / `html.dark` y
referencia `var(--color-brand-…)` en tu CSS.

---

## 3. Tipografía

- **Familia:** la sans del sistema (stack por defecto de Tailwind:
  `ui-sans-serif, system-ui, sans-serif, …`). No se carga ninguna fuente web.
- **Números:** en tablas y columnas numéricas se usa `tabular-nums` para que los
  dígitos queden alineados.

| Elemento | Tamaño | Peso | Color |
|---|---|---|---|
| Título de la página (h1) | 24 px (`text-2xl`) | bold | `brand-text` |
| Subtítulo del encabezado | 14 px (`text-sm`) | normal | `brand-label` |
| Título de sección | 12 px (`text-xs`), MAYÚSCULAS, `tracking-wide` | bold | `brand-accent` |
| Texto de cuerpo | 16 px | normal | `brand-text` |
| Links del menú | 14 px | bold | `brand-label` |
| Tablas | 14 px (`text-sm`) | normal; encabezado en bold | `brand-text` / blanco en el encabezado |
| Etiqueta de KPI | 12.5 px | normal | `brand-label` |
| Valor de KPI | 20 px (`text-xl`) | bold | color semántico |
| Badges / pills | 12 px (`text-xs`) | bold | blanco |
| Pie de página | 14 px, *itálica* | normal | `brand-label` |

---

## 4. Layout

```
┌──────────────────────────────────────────────────────────────┐
│ HEADER  bg brand-amber-bg · borde inferior · px-28 py-20      │
│  Título h1                                   [☾ Oscuro]       │
│  Subtítulo (fecha / contexto)                                 │
├──────────────────────────────────────────────────────────────┤
│ NAV (sticky top-0)  bg brand-bg · borde inferior · px-28 py-8 │
│  Resumen [Portafolio] Tendencia … Limitaciones │ Historial    │
│  ─────────────────────────────────────────────────────────── │
│  ANÁLISIS  (Asia + EEUU) (solo EEUU)      ← sub-selector      │
├──────────────────────────────────────────────────────────────┤
│ MAIN  mx-auto · max-w 1600px · px-28 · pb-64                  │
│   SECCIÓN (mt-40)                                             │
│   TÍTULO DE SECCIÓN ─────────────────────────────────────     │
│   contenido…                                                  │
├──────────────────────────────────────────────────────────────┤
│ FOOTER  borde superior · py-16 · centrado · itálica           │
└──────────────────────────────────────────────────────────────┘
```

- **Una página por ítem del menú.** Cada ítem del menú es una ruta propia, no
  un ancla dentro de una página larga. Los filtros que se comparten entre
  páginas (p. ej. el análisis elegido) van en la URL (`?analisis=us`) y se
  mantienen al navegar.
- **Contenedor principal:** `mx-auto w-full max-w-[1600px] px-7 pb-16`.
- **Separación vertical:** secciones con `mt-10` (40 px). Dentro de una
  sección los elementos van con `mt-4` (16 px). Entre tarjetas, `gap-3`
  (12 px).
- **Padding lateral** fijo de 28 px (`px-7`) en header, nav y main, para
  que todo quede alineado a la misma línea vertical.

---

## 5. Componentes

### 5.1 Encabezado

```html
<header class="border-b border-brand-border bg-brand-amber-bg px-7 py-5">
  <div class="flex flex-wrap items-start justify-between gap-3">
    <div>
      <h1 class="text-2xl font-bold">Título del dashboard</h1>
      <div class="mt-1 text-sm text-brand-label">Generado el … · corrida …</div>
    </div>
    <!-- botón de tema (5.10) -->
  </div>
</header>
```

### 5.2 Menú de navegación

- Contenedor: `sticky top-0 z-10 border-b border-brand-border bg-brand-bg px-7 py-2`.
- Links: `rounded px-2.5 py-1 text-sm font-bold text-brand-label hover:bg-brand-card hover:text-brand-accent`.
- **Activo:** relleno `bg-brand-strong` y texto blanco.
- **Divisor** entre grupos (p. ej. antes de "Historial"): `mx-1 h-5 w-px bg-brand-border`.
- **Sub-selector** (segunda fila, solo en las páginas donde aplica):
  separado con `mt-2 border-t border-brand-border pt-2`, con una etiqueta en
  mayúsculas `text-xs font-bold uppercase tracking-wide text-brand-accent` y
  *pills*:
  - Inactiva: `rounded border border-brand-border bg-brand-bg px-2.5 py-0.5 text-xs font-bold text-brand-label hover:bg-brand-card`
  - Activa: `rounded border border-brand-strong bg-brand-strong px-2.5 py-0.5 text-xs font-bold text-white`

### 5.3 Título de sección

```html
<h2 class="section-header">Portafolio seleccionado — 15 acciones</h2>
```

Texto chico en mayúsculas, color ámbar, con una línea fina debajo que ocupa todo
el ancho. Es la marca de identidad más reconocible del diseño.

### 5.4 Tarjetas KPI

```html
<div class="flex flex-wrap gap-3">
  <div class="min-w-[160px] flex-1 rounded border border-brand-border bg-brand-card p-4">
    <div class="text-[12.5px] text-brand-label">Retorno esperado anual</div>
    <div class="mt-1.5 text-xl font-bold text-brand-green">48.2%</div>
  </div>
  …
</div>
```

Color del valor según lo que representa: `green` = rendimiento, `blue` =
riesgo/volatilidad, `accent` = métrica estrella, `text` = dato de contexto.

### 5.5 Tablas

- Envoltorio: `overflow-x-auto rounded border border-brand-border` (el scroll
  aparece solo si hace falta de verdad).
- `<table class="w-full border-collapse text-sm">`
- **Encabezado:** `bg-brand-strong text-white font-bold px-3 py-2 whitespace-nowrap`,
  clic para ordenar, con indicador `↕ / ↑ / ↓` a 11 px y 70 % de opacidad.
- **Filas:** `border-b border-brand-border odd:bg-brand-bg even:bg-brand-card`
  (filas alternadas, sin hover).
- **Celdas:** `px-3 py-2`.
  - Numéricas: alineadas a la derecha, con `tabular-nums` y `whitespace-nowrap`.
  - Texto largo (nombre, sector, listas): pueden partirse en varias líneas
    (`min-w-[8rem]`, sin `nowrap`), así la tabla no desborda.
  - Números, badges y barras: siempre en una línea.
- **Valores con signo:** `font-bold text-brand-green` si es ≥ 0 y
  `font-bold text-brand-red` si es < 0.

### 5.6 Barra de peso dentro de una celda

```html
<div class="flex items-center justify-end gap-2">
  <div class="h-2.5 w-24 overflow-hidden rounded bg-brand-border">
    <div class="h-full bg-brand-blue" style="width: 72%"></div>
  </div>
  <span class="w-12 text-right tabular-nums">11.9%</span>
</div>
```

El ancho es relativo al **máximo de la columna**, no a 100 %.

### 5.7 Badges de estado

```html
<span class="inline-block whitespace-nowrap rounded-full px-2.5 py-0.5
             text-xs font-bold text-white bg-brand-green-solid">Alcista</span>
```

| Estado | Fondo |
|---|---|
| Positivo | `brand-green-solid` |
| Negativo | `brand-red-solid` |
| Mixto / lateral | `brand-accent-solid` |
| Sin datos | `brand-neutral` |

### 5.8 Barra de composición (100 % apilada)

- Barra de 22 px de alto: `flex overflow-hidden rounded border border-brand-border`,
  un segmento por categoría con `width: X%`, ordenados de mayor a menor.
- Leyenda debajo: `mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-brand-label`,
  cada ítem con un cuadrado de color de 10 px (`h-2.5 w-2.5 rounded-sm`) y el
  texto "País (61.9%)".

### 5.9 Cajas de mensaje

| Tipo | Clases |
|---|---|
| Informativa (notas de contexto) | `rounded border border-brand-border bg-brand-blue-bg p-4` |
| Advertencia / disclaimer | `space-y-2 rounded border border-brand-border bg-brand-amber-bg p-4` |
| Panel neutro / filtros / estado vacío | `rounded border border-brand-border bg-brand-card p-4` (vacío: `p-6 text-center text-brand-muted`) |
| Error de carga | bloque centrado `max-w-2xl p-10 text-center`, título `text-xl font-bold text-brand-red`, texto `text-brand-muted`, `<pre>` en `bg-brand-card p-3 rounded`, botón "Reintentar" |

### 5.10 Botones e inputs

- **Botón secundario (el único estilo que se usa):**
  `rounded border border-brand-border bg-brand-bg px-3 py-1.5 text-sm font-bold text-brand-label hover:bg-brand-card`
- **Inputs / select:** `rounded border border-brand-border bg-brand-bg px-2 py-1.5`,
  con la etiqueta encima: `mb-1 text-sm font-bold text-brand-label`.
- **Código inline:** `rounded bg-brand-card px-1.5 py-0.5`.
- **Botón de tema:** botón secundario con un ícono SVG de 16 px
  (`stroke="currentColor"`, `stroke-width="2"`) y un texto que dice a qué modo
  cambia ("Oscuro" con luna / "Claro" con sol).

### 5.11 Gráficos (LayerChart)

Spec visual (independiente de la librería; se implementa con **LayerChart** en
`src/lib/charts/`):

- Alto fijo de 240 px (mínimo; los gráficos de detalle pueden ser más altos).
- Líneas de 2 px, curva suave leve, puntos con radio 3 (5 al pasar el mouse).
- Tooltip compartido por eje X: muestra todas las series de una fecha a la vez.
- Leyenda solo si hay más de una serie; marcas de 12 px, texto de 11 px en
  `brand-muted`.
- Ejes a 11 px en `viz-axis`, grilla en `viz-grid`. Como LayerChart dibuja SVG,
  los colores se pasan como `var(--color-viz-*)` / clases Tailwind y el cambio
  de tema **no requiere redibujar**.
- Series con la paleta categórica de 2.2, en ese orden.
- Estado vacío: caja `bg-brand-card` del mismo alto con el texto "Sin datos
  para este rango." centrado.
- Tipos que se necesitan en el sitio: línea (tendencia, historial), barras
  (pesos, categorías, sectores), scatter (frontera eficiente), sparkline
  (precio en tablas), barra apilada 100 % (composición, ver 5.8).

### 5.12 Pie de página

```html
<footer class="border-t border-brand-border py-4 text-center text-sm italic text-brand-label">
  Fog Radar Quant · Información generada automáticamente con fines informativos. No constituye asesoría financiera.
</footer>
```

---

## 6. Modo claro / oscuro

**Comportamiento:**

1. Si el usuario eligió un modo, se usa ese (se guarda en `localStorage`, clave `fog-radar-theme`).
2. Si nunca eligió, se sigue `prefers-color-scheme` del sistema, y el tema cambia
   solo si cambia la preferencia del sistema.
3. El tema se aplica con la clase `dark` en `<html>` **antes** de pintar, para
   evitar un destello en claro. Con SSR esto se hace con un script en línea en
   `src/app.html`.

**Script en línea en `src/app.html` (dentro de `<head>`, antes de `%sveltekit.head%`):**

```html
<script>
  try {
    var t = localStorage.getItem('fog-radar-theme')
    if (t !== 'light' && t !== 'dark') {
      t = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    }
    if (t === 'dark') document.documentElement.classList.add('dark')
  } catch (e) {}
</script>
```

**Módulo de tema (Svelte 5, `src/lib/theme.svelte.ts`; validado con `svelte-autofixer`):**

```ts
const KEY = 'fog-radar-theme';
type Theme = 'light' | 'dark';

function stored(): Theme | null {
	try {
		const t = localStorage.getItem(KEY);
		return t === 'light' || t === 'dark' ? t : null;
	} catch {
		return null;
	}
}

class ThemeState {
	current = $state<Theme>('light');

	/** Llamar una vez desde el +layout.svelte raíz (onMount). */
	init() {
		const media = window.matchMedia('(prefers-color-scheme: dark)');
		this.current = stored() ?? (media.matches ? 'dark' : 'light');
		media.addEventListener('change', (e) => {
			if (!stored()) this.set(e.matches ? 'dark' : 'light', false);
		});
	}

	set(t: Theme, persist = true) {
		this.current = t;
		document.documentElement.classList.toggle('dark', t === 'dark');
		if (persist) {
			try {
				localStorage.setItem(KEY, t);
			} catch {
				/* modo privado: se ignora */
			}
		}
	}

	toggle() {
		this.set(this.current === 'dark' ? 'light' : 'dark');
	}
}

export const theme = new ThemeState();
```

`<body class="bg-brand-bg text-brand-text">`: el body también usa tokens.

---

## 7. Reglas de uso (checklist)

- ✅ Usar solo tokens `brand-*` / `viz-*`. ❌ Nada de `bg-white`, `text-slate-*`,
  `text-gray-*` ni hex sueltos en los componentes.
- ✅ Texto blanco solo sobre `brand-strong` o `*-solid`.
- ✅ Títulos de sección siempre con `.section-header`.
- ✅ Bordes de 1 px `brand-border` y esquinas `rounded` (4 px). ❌ Sin sombras
  ni gradientes.
- ✅ Números alineados a la derecha con `tabular-nums`.
- ✅ Verde = positivo, rojo = negativo, ámbar = mixto o destacado. No reutilizar
  estos colores con otro significado. Categorías que no son señal (p. ej.
  Large/Mid/Small-cap) usan `brand-strong`.
- ✅ **Todas** las tablas ordenables por click en el encabezado (componente
  `DataTable` común). En los proyectos originales esto solo estaba en algunas.
- ✅ Cada ítem de menú o pestaña es una **ruta real** de SvelteKit (URL
  compartible), no un `#hash`.
- ✅ Revisar cada pantalla nueva en **ambos** modos.
- ✅ Revisar las tablas a 1024 px de ancho: no deben tener scroll horizontal
  (bajo 1280 px se permite `px-2` y encabezados en dos líneas; desde `xl`,
  `px-3` + `whitespace-nowrap`).

---

## 8. Stack de referencia

- SvelteKit 2 + Svelte 5 (runes) + TypeScript, `adapter-static` (SPA)
- Tailwind CSS v4 (`@tailwindcss/vite`), tokens definidos con `@theme` en `src/app.css`
- shadcn-svelte (bits-ui) para componentes base
- LayerChart para gráficos

## 9. Dónde vivirá cada pieza (a crear en la Fase 2)

| Pieza del documento | Archivo |
|---|---|
| Tokens (2.1/2.3) y `.section-header` (5.3) | `frontend/src/app.css` |
| Script anti-flash (6) | `frontend/src/app.html` |
| Módulo de tema (6) | `frontend/src/lib/theme.svelte.ts` |
| Encabezado, menú, main, pie (4, 5.1, 5.2, 5.12) | `frontend/src/routes/+layout.svelte` + `lib/components/shared/` |
| Tablas (5.5) | `frontend/src/lib/components/shared/DataTable.svelte` |
| Tarjetas KPI (5.4), badges (5.7), barras (5.6/5.8), mensajes (5.9) | `frontend/src/lib/components/shared/` + `ui/` |
| Gráficos (2.2, 5.11) | `frontend/src/lib/charts/` |
