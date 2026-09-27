<!--
  Gráfico de líneas (§5.11): 240 px, líneas de 2 px, tooltip compartido por
  eje X y leyenda solo con más de una serie.
-->
<script lang="ts">
	import { LineChart } from 'layerchart';
	import { scaleUtc } from 'd3-scale';
	import { curveMonotoneX } from 'd3-shape';
	import * as Chart from '$lib/components/ui/chart';
	import ChartEmpty from './ChartEmpty.svelte';
	import { CHART_CLASS, chartConfig, withColors, type SeriesDef } from './theme';

	interface Props {
		data: Record<string, unknown>[];
		/** Campo del eje X (fechas como Date). */
		x: string;
		series: SeriesDef[];
		height?: number;
		yFormat?: (v: number) => string;
		xFormat?: (v: Date) => string;
		/** Incluir el 0 en el eje Y (por defecto no: precios e índices se leen mejor sin él). */
		zeroBaseline?: boolean;
		/** Máximo de etiquetas en el eje X. */
		xTicks?: number;
	}

	let {
		data,
		x,
		series,
		height = 240,
		yFormat,
		xFormat = (d: Date) =>
			d.toLocaleDateString('es-CL', { month: 'short', year: '2-digit', timeZone: 'UTC' }),
		zeroBaseline = false,
		xTicks = 8
	}: Props = $props();

	const colored = $derived(withColors(series));
	const config = $derived(chartConfig(series));
</script>

{#if data.length === 0}
	<ChartEmpty {height} />
{:else}
	<Chart.Container {config} class={CHART_CLASS} style="height: {height}px">
		<LineChart
			{data}
			{x}
			xScale={scaleUtc()}
			series={colored}
			yBaseline={zeroBaseline ? 0 : null}
			yNice
			legend={colored.length > 1}
			props={{
				spline: { curve: curveMonotoneX, strokeWidth: 2 },
				highlight: { points: { r: 5 } },
				xAxis: { format: xFormat, ticks: xTicks },
				yAxis: yFormat ? { format: yFormat } : {}
			}}
		>
			{#snippet tooltip()}
				<Chart.Tooltip
					labelFormatter={(v: unknown) => (v instanceof Date ? xFormat(v) : String(v))}
				/>
			{/snippet}
		</LineChart>
	</Chart.Container>
{/if}
