<script lang="ts">
	import type { UsCandidateRow } from '$lib/api/types';
	import { Sparkline } from '$lib/charts';
	import { DataTable, ErrorState, KpiCard, KpiGrid, Notice, Section } from '$lib/components/shared';
	import { RunInfo, UsTicker } from '$lib/modules/us';
	import { fmtNumber } from '$lib/format';
	import {
		fmtPrice,
		fmtRet,
		fmtScore,
		fmtShare,
		fmtUsd,
		isGrowingPerformance
	} from '$lib/modules/us/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const run = $derived(data.ranking.ok ? data.ranking.data : null);

	// Un indicador por sección del menú (Candidatos, Evaluación, Rendimiento, Portafolio).
	const cand = $derived(data.candidates.ok ? data.candidates.data.rows : []);
	const conScore = $derived(cand.filter((r) => r.score.overall != null));
	const mejorScore = $derived(
		conScore.length
			? conScore.reduce((m, r) => ((r.score.overall ?? 0) > (m.score.overall ?? 0) ? r : m))
			: null
	);
	const scorePromedio = $derived(
		conScore.length
			? conScore.reduce((s, r) => s + (r.score.overall ?? 0), 0) / conScore.length
			: null
	);
	const crecientes = $derived(cand.filter((r) => isGrowingPerformance(r)).length);
	const portafolio = $derived(
		data.precalc.ok
			? (data.precalc.data.portafolios.find((p) => p.clave === 'top15') ??
					data.precalc.data.portafolios[0])
			: null
	);

	type Fila = UsCandidateRow & { pos: number; tendencia: null };
	const filas = $derived<Fila[]>(
		(run?.rows ?? []).map((r, i) => ({ ...r, pos: i + 1, tendencia: null }))
	);

	const columns: Column<Fila>[] = [
		{ key: 'pos', label: '#', numeric: true },
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'name', label: 'Empresa', wrap: true },
		{ key: 'sector', label: 'Sector', wrap: true },
		{ key: 'tendencia', label: 'Tendencia (3m)', sortable: false, csv: () => null },
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
</script>

{#if run}
	<Section title="Resumen de la corrida">
		<KpiGrid>
			<KpiCard
				label="Líder momentum 12-1"
				value={run.rows[0]?.ticker ?? '—'}
				hint={run.rows[0] ? fmtRet(run.rows[0].momentum_12_1) : undefined}
				tone="accent"
			/>
			{#if mejorScore}
				<KpiCard
					label="Evaluación: mejor score"
					value={mejorScore.ticker}
					hint="score {fmtScore(mejorScore.score.overall)} · promedio {fmtScore(scorePromedio)}"
					tone="green"
				/>
			{/if}
			{#if cand.length}
				<KpiCard
					label="Rendimiento creciente"
					value="{crecientes} de {cand.length}"
					hint="rendimientos crecientes por plazo"
					tone="blue"
				/>
			{/if}
			{#if portafolio}
				<KpiCard
					label="Portafolio ({portafolio.titulo})"
					value="Sharpe {fmtNumber(portafolio.resultado.metrics.sharpe, 2)}"
					hint="retorno {fmtRet(portafolio.resultado.metrics.expected_return)} · vol. {fmtShare(
						portafolio.resultado.metrics.volatility,
						1
					)}"
					tone="accent"
				/>
			{/if}
		</KpiGrid>
	</Section>
{/if}

<Section title="Top {run?.top_n ?? 15} por momentum 12-1">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<p class="max-w-3xl text-sm text-brand-muted">
			Ranking cuantitativo por momentum 12-1 (Jegadeesh &amp; Titman) de acciones de EEUU con
			capitalización sobre el umbral. Datos de la corrida batch más reciente; esta página no
			descarga nada.
		</p>
	</div>
	{#if !run}
		{#if !data.ranking.ok}
			<ErrorState
				title={data.ranking.code === 'no_data' ? 'Aún no hay corridas del screener' : undefined}
				message={data.ranking.message}
				code={data.ranking.code}
			/>
		{/if}
	{:else}
		<RunInfo {run} />
		<DataTable
			rows={filas}
			{columns}
			csvFilename="top15_{run.run_id}"
			caption="Top 15 por momentum"
			emptyText="Ningún ticker superó el umbral en esta corrida."
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
		<Notice kind="neutral">
			<p>
				Momentum 12-1: retorno entre hace 12 meses y hace 1 mes (excluye el último mes para evitar
				la reversión de corto plazo). No es una recomendación de inversión.
			</p>
		</Notice>
	{/if}
</Section>
