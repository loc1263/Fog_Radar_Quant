/**
 * Estilo común de gráficos (Dashboard ejecutivo §2.2 y §5.11).
 * Los colores son variables CSS: el cambio de tema no requiere redibujar.
 */

/** Paleta categórica en el orden del sistema de diseño (máx. 8 series). */
export const SERIES_COLORS = Array.from({ length: 8 }, (_, i) => `var(--color-viz-${i + 1})`);

export function seriesColor(index: number): string {
	return SERIES_COLORS[index % SERIES_COLORS.length];
}

/** Clases para el contenedor: ejes 11 px en viz-axis, grilla en viz-grid. */
export const CHART_CLASS = [
	'aspect-auto w-full',
	'[&_.lc-axis-tick-label]:fill-viz-axis [&_.lc-axis-tick-label]:text-[11px]',
	'[&_.lc-grid-x-rule]:stroke-viz-grid [&_.lc-grid-y-rule]:stroke-viz-grid',
	'[&_.lc-legend-swatch-label]:text-[11px] [&_.lc-legend-swatch-label]:fill-brand-muted'
].join(' ');

export interface SeriesDef {
	key: string;
	label: string;
	/** Color propio (por defecto, la paleta en orden). */
	color?: string;
}

export function withColors(series: SeriesDef[]) {
	return series.map((s, i) => ({ ...s, color: s.color ?? seriesColor(i) }));
}

/** Config para el ChartContainer de shadcn (tooltip usa label y color). */
export function chartConfig(series: SeriesDef[]) {
	return Object.fromEntries(
		withColors(series).map((s) => [s.key, { label: s.label, color: s.color }])
	);
}
