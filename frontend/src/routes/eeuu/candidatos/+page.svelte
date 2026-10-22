<script lang="ts">
	import type { UsCandidateRow } from '$lib/api/types';
	import { Sparkline } from '$lib/charts';
	import { DataTable, ErrorState, Section } from '$lib/components/shared';
	import { UsTicker } from '$lib/modules/us';
	import { candidateCsvColumns, fmtPrice, fmtRet, fmtUsd } from '$lib/modules/us/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const run = $derived(data.candidates.ok ? data.candidates.data : null);

	type Fila = UsCandidateRow & { tendencia: null };
	const filas = $derived<Fila[]>((run?.rows ?? []).map((r) => ({ ...r, tendencia: null })));

	const columns: Column<Fila>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'name', label: 'Empresa', wrap: true },
		{ key: 'sector', label: 'Sector', wrap: true },
		{ key: 'tendencia', label: 'Tendencia (3m)', sortable: false },
		{ key: 'market_cap', label: 'Market cap', numeric: true, format: (v) => fmtUsd(v) },
		{ key: 'price', label: 'Precio', numeric: true, format: (v) => fmtPrice(v) },
		{
			key: 'momentum_12_1',
			label: 'Momentum 12-1',
			numeric: true,
			signed: true,
			format: (v) => fmtRet(v)
		},
		{
			key: 'return_6m',
			label: 'Retorno 6m',
			numeric: true,
			signed: true,
			format: (v) => fmtRet(v)
		},
		{ key: 'return_1y', label: 'Retorno 1a', numeric: true, signed: true, format: (v) => fmtRet(v) }
	];
	const csvColumns = candidateCsvColumns<Fila>();
</script>

<Section title="Todos los candidatos">
	<p class="text-xs text-brand-muted">
		Todos los tickers de S&amp;P 500 ∪ Nasdaq-100 que superaron el umbral de market cap en la última
		corrida (no solo el top 15). El CSV incluye todos los datos de cada ticker.
	</p>
	{#if !run}
		{#if !data.candidates.ok}
			<ErrorState message={data.candidates.message} code={data.candidates.code} />
		{/if}
	{:else}
		<DataTable
			rows={filas}
			{columns}
			initialSort={{ key: 'market_cap', dir: 'desc' }}
			csvFilename="candidatos_{run.run_id}"
			{csvColumns}
			caption="Candidatos de la última corrida"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'ticker'}
					<UsTicker ticker={row.ticker} />
				{:else if column.key === 'tendencia'}
					<Sparkline values={data.prices[row.ticker] ?? []} width={72} height={24} />
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	{/if}
</Section>
