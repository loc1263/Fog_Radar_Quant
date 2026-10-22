<script lang="ts">
	import { BarChart } from '$lib/charts';
	import { DataTable, ErrorState, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { CorridaInfo, TickersCategoria } from '$lib/modules/etf';
	import { csvName, fmtRend } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const c = $derived(data.cats.ok ? data.cats.data : null);
	// Como el original: las 15 mejores categorías por Sharpe promedio.
	const filas = $derived((c?.filas ?? []).slice(0, 15).map((f, i) => ({ ...f, pos: i + 1 })));
	type Fila = (typeof filas)[number];

	const num = (key: string, label: string, digits: number): Column<Fila> => ({
		key,
		label,
		numeric: true,
		format: (v) => (typeof v === 'number' ? fmtNumber(v, digits) : '—')
	});
	const columns: Column<Fila>[] = [
		{ key: 'pos', label: '#', numeric: true },
		{ key: 'categoria', label: 'Categoría', wrap: true },
		{ key: 'n', label: 'N', numeric: true, title: 'Clic en el número para ver los tickers' },
		num('sharpe_promedio', 'Sharpe prom.', 2),
		num('treynor_mediana', 'Treynor med.', 2),
		num('beta_promedio', 'Beta prom.', 2),
		{
			key: 'perf_1y_promedio',
			label: 'Perf. 1Y prom.',
			numeric: true,
			signed: true,
			format: (v) => fmtRend(v, 1)
		},
		{
			key: 'perf_6m_promedio',
			label: 'Perf. 6M prom.',
			numeric: true,
			signed: true,
			format: (v) => fmtRend(v, 1)
		},
		num('score_promedio', 'Score prom.', 1),
		{
			key: 'pct_positivos_1y',
			label: '% pos. 1Y',
			numeric: true,
			format: (v) => (typeof v === 'number' ? `${fmtNumber(v, 0)} %` : '—')
		}
	];
	const csvColumns: Column<Fila>[] = [
		...columns.filter((col) => col.key !== 'pos'),
		{ key: 'tickers', label: 'Tickers', csv: (v) => (Array.isArray(v) ? v.join(', ') : null) }
	];
</script>

<Section title="Ranking por categoría">
	{#if !c}
		{#if !data.cats.ok}
			<ErrorState
				title="El análisis de Categoría no está disponible"
				message={data.cats.message}
				code={data.cats.code}
			/>
		{/if}
	{:else}
		<CorridaInfo fecha={c.fecha}
			>{c.filas.length} categorías con al menos {c.min_n_categoria} ETFs</CorridaInfo
		>
		<Notice size="compact">
			<p>
				Rankea categorías completas (no ETFs individuales) dentro del universo ya depurado de
				distorsiones, por Sharpe Ratio promedio. El Treynor va por mediana (es inestable con Beta
				cercano a 0). Se muestran las 15 primeras.
			</p>
		</Notice>
		<h3 class="text-sm font-bold text-brand-label">Top categorías por Sharpe Ratio promedio</h3>
		<BarChart
			data={filas}
			category="categoria"
			series={[
				{ key: 'sharpe_promedio', label: 'Sharpe promedio', color: 'var(--color-brand-blue)' }
			]}
			labelWidth={190}
			height={Math.max(260, filas.length * 26)}
			valueFormat={(v) => fmtNumber(v, 1)}
		/>
		<DataTable
			rows={filas}
			{columns}
			{csvColumns}
			csvFilename={csvName('etf_categorias_ranking', c.fecha)}
			caption="Ranking por categoría"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'n'}
					<TickersCategoria tickers={row.tickers} categoria={row.categoria} />
				{:else if column.key === 'score_promedio'}
					<span class="font-num text-brand-accent">{text}</span>
				{:else if column.key === 'categoria'}
					<span class="font-num">{text}</span>
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	{/if}
</Section>
