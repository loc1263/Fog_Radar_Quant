<!-- Mini gráfico de precio para celdas de tabla: verde si sube, rojo si baja. -->
<script lang="ts">
	import { LineChart } from 'layerchart';

	interface Props {
		values: (number | null)[];
		width?: number;
		height?: number;
	}

	let { values, width = 96, height = 28 }: Props = $props();

	const points = $derived(
		values
			.map((v, i) => ({ i, v }))
			.filter((p): p is { i: number; v: number } => typeof p.v === 'number' && Number.isFinite(p.v))
	);
	const up = $derived(points.length > 1 && points[points.length - 1].v >= points[0].v);
	const color = $derived(up ? 'var(--color-brand-green)' : 'var(--color-brand-red)');
</script>

{#if points.length > 1}
	<div
		style:width="{width}px"
		style:height="{height}px"
		class="inline-block align-middle"
		aria-hidden="true"
	>
		<LineChart
			data={points}
			x="i"
			y="v"
			yBaseline={null}
			axis={false}
			grid={false}
			rule={false}
			series={[{ key: 'v', color }]}
			props={{ spline: { strokeWidth: 1.5 } }}
		/>
	</div>
{:else}
	<span class="text-brand-muted">—</span>
{/if}
