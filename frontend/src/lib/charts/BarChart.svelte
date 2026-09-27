<!-- Barras (pesos, categorías, sectores). Horizontal por defecto: etiquetas largas legibles. -->
<script lang="ts">
	import { BarChart } from 'layerchart';
	import { scaleBand } from 'd3-scale';
	import * as Chart from '$lib/components/ui/chart';
	import ChartEmpty from './ChartEmpty.svelte';
	import { CHART_CLASS, chartConfig, withColors, type SeriesDef } from './theme';

	interface Props {
		data: Record<string, unknown>[];
		/** Campo de la categoría. */
		category: string;
		series: SeriesDef[];
		orientation?: 'horizontal' | 'vertical';
		height?: number;
		valueFormat?: (v: number) => string;
	}

	let {
		data,
		category,
		series,
		orientation = 'horizontal',
		height = 240,
		valueFormat
	}: Props = $props();

	const colored = $derived(withColors(series));
	const config = $derived(chartConfig(series));
	const horizontal = $derived(orientation === 'horizontal');
	const valueAxis = $derived(valueFormat ? { format: valueFormat } : {});
</script>

{#if data.length === 0}
	<ChartEmpty {height} />
{:else}
	<Chart.Container {config} class={CHART_CLASS} style="height: {height}px">
		{#if horizontal}
			<BarChart
				{data}
				orientation="horizontal"
				y={category}
				yScale={scaleBand().padding(0.25)}
				series={colored}
				seriesLayout="group"
				legend={colored.length > 1}
				padding={{ left: 110, bottom: 20 }}
				props={{ bars: { stroke: 'none', radius: 2 }, xAxis: valueAxis }}
			>
				{#snippet tooltip()}
					<Chart.Tooltip />
				{/snippet}
			</BarChart>
		{:else}
			<BarChart
				{data}
				x={category}
				xScale={scaleBand().padding(0.25)}
				series={colored}
				seriesLayout="group"
				legend={colored.length > 1}
				props={{ bars: { stroke: 'none', radius: 2 }, yAxis: valueAxis }}
			>
				{#snippet tooltip()}
					<Chart.Tooltip />
				{/snippet}
			</BarChart>
		{/if}
	</Chart.Container>
{/if}
