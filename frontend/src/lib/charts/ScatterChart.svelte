<!-- Dispersión (p. ej. frontera eficiente: volatilidad vs retorno). -->
<script lang="ts">
	import { ScatterChart } from 'layerchart';
	import * as Chart from '$lib/components/ui/chart';
	import ChartEmpty from './ChartEmpty.svelte';
	import { CHART_CLASS, chartConfig, withColors, type SeriesDef } from './theme';

	interface Props {
		/** Una serie por grupo de puntos: `{ key, label, data }`. */
		groups: (SeriesDef & { data: Record<string, unknown>[] })[];
		x: string;
		y: string;
		height?: number;
		xFormat?: (v: number) => string;
		yFormat?: (v: number) => string;
	}

	let { groups, x, y, height = 280, xFormat, yFormat }: Props = $props();

	const colored = $derived(withColors(groups));
	const config = $derived(chartConfig(groups));
	const empty = $derived(groups.every((g) => g.data.length === 0));
</script>

{#if empty}
	<ChartEmpty {height} />
{:else}
	<Chart.Container {config} class={CHART_CLASS} style="height: {height}px">
		<ScatterChart
			{x}
			{y}
			series={colored}
			legend={colored.length > 1}
			props={{
				points: { r: 4 },
				xAxis: xFormat ? { format: xFormat } : {},
				yAxis: yFormat ? { format: yFormat } : {}
			}}
		>
			{#snippet tooltip()}
				<Chart.Tooltip />
			{/snippet}
		</ScatterChart>
	</Chart.Container>
{/if}
