<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { UsCandidateRow } from '$lib/api/types';
	import { Sparkline } from '$lib/charts';
	import { DataTable, ErrorState, Section } from '$lib/components/shared';
	import { UsTicker } from '$lib/modules/us';
	import {
		candidateCsvColumns,
		fmtPrice,
		fmtRet,
		isGrowingPerformance
	} from '$lib/modules/us/format';
	import type { Column } from '$lib/table';
	import { cn } from '$lib/utils';

	let { data } = $props();

	const run = $derived(data.candidates.ok ? data.candidates.data : null);
	// Filtro en la URL (?crecientes=1): compartible y sin depender de JavaScript.
	const soloCrecientes = $derived(page.url.searchParams.get('crecientes') === '1');

	type Fila = UsCandidateRow & { tendencia: null; creciente: boolean };
	const todas = $derived<Fila[]>(
		(run?.rows ?? []).map((r) => ({ ...r, tendencia: null, creciente: isGrowingPerformance(r) }))
	);
	const crecientes = $derived(todas.filter((f) => f.creciente).length);
	const filas = $derived(soloCrecientes ? todas.filter((f) => f.creciente) : todas);

	const ret = (key: keyof UsCandidateRow, label: string): Column<Fila> => ({
		key,
		label,
		numeric: true,
		signed: true,
		format: (v) => fmtRet(v)
	});

	const columns: Column<Fila>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'name', label: 'Nombre', wrap: true },
		{ key: 'sector', label: 'Sector', wrap: true },
		{ key: 'tendencia', label: 'Tendencia (3m)', sortable: false },
		{ key: 'price', label: 'Precio', numeric: true, format: (v) => fmtPrice(v) },
		ret('return_1d', '1D'),
		ret('return_5d', '5D'),
		ret('return_3m', '3M'),
		ret('return_6m', '6M'),
		ret('return_1y', '1A'),
		ret('return_ytd', 'YTD')
	];
	const csvColumns = candidateCsvColumns<Fila>();
</script>

<Section title="Rendimiento por plazo">
	<p class="text-xs text-brand-muted">
		Retorno de cada candidato a 1 día, 5 días, 3 y 6 meses, 1 año y en lo que va del año.
		"Creciente" = cada plazo rinde más que el anterior (1D &lt; 5D &lt; 3M &lt; 6M &lt; 1A).
	</p>
	{#if !run}
		{#if !data.candidates.ok}
			<ErrorState message={data.candidates.message} code={data.candidates.code} />
		{/if}
	{:else}
		<a
			href="{resolve('/eeuu/rendimiento')}{soloCrecientes ? '' : '?crecientes=1'}"
			data-sveltekit-noscroll
			data-sveltekit-replacestate
			aria-current={soloCrecientes ? 'true' : undefined}
			class={cn(
				'inline-block rounded border px-3 py-1.5 text-sm font-bold',
				soloCrecientes
					? 'border-brand-strong bg-brand-strong text-white'
					: 'border-brand-border bg-brand-bg text-brand-label hover:bg-brand-card'
			)}
		>
			↗ Solo rendimientos crecientes ({crecientes})
		</a>
		<DataTable
			rows={filas}
			{columns}
			initialSort={{ key: 'return_1d', dir: 'desc' }}
			csvFilename="rendimiento_{run.run_id}"
			{csvColumns}
			caption="Rendimiento por plazo"
			emptyText="Ningún ticker tiene rendimientos estrictamente crecientes en todos los plazos."
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'ticker'}
					<span class="inline-flex items-center gap-1">
						<UsTicker ticker={row.ticker} />
						{#if row.creciente}
							<span class="font-num text-brand-green" title="Rendimiento creciente">↗</span>
						{/if}
					</span>
				{:else if column.key === 'tendencia'}
					<Sparkline values={data.prices[row.ticker] ?? []} width={72} height={24} />
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	{/if}
</Section>
