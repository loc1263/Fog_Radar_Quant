<script lang="ts">
	import { CompositionBar, ErrorState, Notice, Section, WeightBar } from '$lib/components/shared';
	import { SignalBadge } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { CorridaInfo, EtfTable } from '$lib/modules/etf';
	import { col, conPosicion, csvName, fmtRend, pesoPorBucket } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const cap = $derived(data.cap.ok ? data.cap.data : null);
	const filas = $derived(conPosicion(cap?.filas ?? []));
	type Fila = (typeof filas)[number];
	const pesoMax = $derived(Math.max(0, ...filas.map((f) => f.peso_portfolio ?? 0)));
	const composicion = $derived(pesoPorBucket(filas));

	const columns: Column<Fila>[] = [
		col.pos<Fila>(),
		col.ticker<Fila>(),
		col.nombre<Fila>(),
		{ key: 'cap_bucket', label: 'Bucket' },
		col.rend<Fila>('perf_6m', '6M'),
		col.rend<Fila>('perf_1y', '1Y'),
		col.beta<Fila>(),
		{
			...col.rend<Fila>('alpha_capm_aprox', 'Alpha aprox.'),
			title: 'Retorno 1Y − Beta × retorno 1Y del benchmark (CAPM de un punto)'
		},
		col.score<Fila>('momentum_score', 'Score momentum'),
		{
			key: 'peso_portfolio',
			label: 'Peso',
			numeric: true,
			format: (v) => `${fmtNumber(v, 2)} %`
		}
	];
	const raw = (key: string, label: string): Column<Fila> => ({ key, label });
	const csvColumns: Column<Fila>[] = [
		raw('ticker', 'Ticker'),
		raw('nombre', 'Nombre'),
		raw('categoria', 'Categoría'),
		raw('cap_bucket', 'Cap Bucket'),
		raw('perf_6m', 'Performance 6M (%)'),
		raw('perf_1y', 'Performance 1Y (%)'),
		raw('beta', 'Beta'),
		raw('alpha_capm_aprox', 'Alpha CAPM aprox (%)'),
		raw('momentum_score', 'Momentum Score'),
		raw('peso_portfolio', 'Peso Portfolio (%)')
	];
</script>

<Section title="Top 15 por capitalización">
	{#if !cap}
		{#if !data.cap.ok}
			<ErrorState
				title="El análisis Top 15 Cap. no está disponible"
				message={data.cap.message}
				code={data.cap.code}
			/>
		{/if}
	{:else}
		<CorridaInfo fecha={cap.fecha}>
			benchmark del Alpha: <b>{cap.benchmark_ticker ?? '—'}</b> ({fmtRend(cap.benchmark_retorno_1y)} en
			1 año)
		</CorridaInfo>
		<Notice>
			<p>
				Estrategia tipo Vanguard U.S. Momentum Factor (VFMO) aplicada sobre ETFs de renta variable
				de EE.UU.: los 5 ETFs de mayor score de momentum de cada bucket de capitalización (Large /
				Mid / Small), 15 en total, ponderados en proporción al score. El score promedia,
				normalizados, el retorno 1Y, el 6M y un Alpha CAPM aproximado.
			</p>
		</Notice>

		{#if cap.buckets.length > 0}
			<div class="flex flex-wrap gap-3">
				{#each cap.buckets as b (b.cap_bucket)}
					<div class="min-w-[160px] flex-1 rounded border border-brand-border bg-brand-card p-4">
						<div class="text-[12.5px] text-brand-label">{b.cap_bucket}</div>
						<div class="mt-1.5 text-xl font-bold tabular-nums">
							{b.n_seleccionados} de {b.n_universo} ETFs
						</div>
						<div class="mt-1 text-sm text-brand-label">
							AUM representado (informativo):
							<span class="font-bold text-brand-text tabular-nums"
								>{fmtNumber(b.pct_aum_cubierto, 1)} %</span
							>
						</div>
					</div>
				{/each}
			</div>
		{/if}
		{#if composicion.length > 0}
			<div>
				<div class="mb-2 text-sm font-bold text-brand-label">
					Peso de la cartera por capitalización
				</div>
				<CompositionBar segments={composicion} />
			</div>
		{/if}

		<EtfTable
			rows={filas}
			{columns}
			scoreKey="momentum_score"
			{csvColumns}
			csvFilename={csvName('etf_russell3000_portfolio', cap.fecha)}
			caption="Top 15 por capitalización"
		>
			{#snippet extra({ column, row, text })}
				{#if column.key === 'cap_bucket'}
					<SignalBadge signal="category" label={row.cap_bucket ?? '—'} />
				{:else if column.key === 'peso_portfolio'}
					<WeightBar
						value={row.peso_portfolio ?? 0}
						max={pesoMax}
						label="{fmtNumber(row.peso_portfolio, 2)} %"
					/>
				{:else}
					{text}
				{/if}
			{/snippet}
		</EtfTable>
	{/if}
</Section>
