<script lang="ts">
	import { ErrorState, Notice, Section } from '$lib/components/shared';
	import { CorridaInfo, EtfTable } from '$lib/modules/etf';
	import { col, conPosicion, csvName } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const t = $derived(data.tabla.ok ? data.tabla.data : null);
	const filas = $derived(conPosicion(t?.filas ?? []));
	type Fila = (typeof filas)[number];

	const columns = [
		col.pos<Fila>(),
		col.ticker<Fila>(),
		col.nombre<Fila>(),
		...col.periodos<Fila>(true),
		col.sharpe<Fila>(),
		col.beta<Fila>(),
		col.rango<Fila>(),
		col.score<Fila>()
	];
	const raw = (key: string, label: string): Column<Fila> => ({ key, label });
	const csvColumns: Column<Fila>[] = [
		raw('ticker', 'Ticker'),
		raw('nombre', 'Nombre'),
		raw('categoria', 'Categoría'),
		raw('perf_5d', 'Performance 5D (%)'),
		raw('perf_1m', 'Performance 1M (%)'),
		raw('perf_6m', 'Performance 6M (%)'),
		raw('perf_ytd', 'Performance YTD (%)'),
		raw('perf_1y', 'Performance 1Y (%)'),
		raw('sharpe', 'Sharpe Ratio'),
		raw('beta', 'Beta'),
		raw('score', 'Score')
	];
</script>

<Section title="Top 15 Vanguard">
	{#if !t}
		{#if !data.tabla.ok}<ErrorState message={data.tabla.message} code={data.tabla.code} />{/if}
	{:else}
		<CorridaInfo fecha={t.fecha} />
		<Notice>
			<p>
				Top 15 ETFs de Vanguard únicamente, ordenados por Score (mismo ranking principal que la
				sección "Top 15"). Clic en un encabezado para reordenar.
			</p>
		</Notice>
		<EtfTable
			rows={filas}
			{columns}
			csvFilename={csvName('etf_top15_vanguard', t.fecha)}
			{csvColumns}
			caption="Top 15 Vanguard"
			emptyText="Ningún ETF de Vanguard en el universo depurado de esta corrida."
		/>
	{/if}
</Section>
