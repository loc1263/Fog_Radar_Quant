<script lang="ts">
	import type { UsCandidateRow, UsScoreCategories } from '$lib/api/types';
	import { DataTable, ErrorState, Notice, Section, SignalBadge } from '$lib/components/shared';
	import { UsScore, UsTicker } from '$lib/modules/us';
	import {
		CATEGORY_KEYS,
		candidateCsvColumns,
		fmtScore,
		scoreClass,
		scoreSignal
	} from '$lib/modules/us/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const run = $derived(data.candidates.ok ? data.candidates.data : null);

	type CatKey = `c_${keyof UsScoreCategories}`;
	type Fila = UsCandidateRow & { overall: number | null; senal: string } & Record<
			CatKey,
			number | null
		>;

	const filas = $derived<Fila[]>(
		(run?.rows ?? []).map((r) => ({
			...r,
			overall: r.score.overall ?? null,
			senal: r.score.overall == null ? 'Sin datos' : r.score.label,
			...(Object.fromEntries(
				CATEGORY_KEYS.map((k) => [`c_${k}`, r.score.categories[k] ?? null])
			) as Record<CatKey, number | null>)
		}))
	);

	const SHORT: Record<keyof UsScoreCategories, string> = {
		valuation: 'Valoración',
		profitability: 'Rentabilidad',
		financial_health: 'Salud fin.',
		shareholder_return: 'Retorno acc.',
		technical: 'Técnico'
	};

	const columns: Column<Fila>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'sector', label: 'Sector', wrap: true },
		{ key: 'overall', label: 'Score', numeric: true, format: (v) => fmtScore(v) },
		{ key: 'senal', label: 'Señal' },
		...CATEGORY_KEYS.map((k): Column<Fila> => ({
			key: `c_${k}`,
			label: SHORT[k],
			numeric: true,
			format: (v) => fmtScore(v)
		}))
	];
	const csvColumns = candidateCsvColumns<Fila>();
</script>

<Section title="Evaluación de salud">
	<p class="max-w-3xl text-sm text-brand-muted">
		Score 0-100 a partir de los datos fundamentales y técnicos de cada candidato (valoración 25 %,
		rentabilidad 25 %, salud financiera 20 %, retorno al accionista 10 %, técnico 20 %). Cada
		categoría promedia sus métricas disponibles.
	</p>
	{#if !run}
		{#if !data.candidates.ok}
			<ErrorState message={data.candidates.message} code={data.candidates.code} />
		{/if}
	{:else}
		<DataTable
			rows={filas}
			{columns}
			initialSort={{ key: 'overall', dir: 'desc' }}
			csvFilename="evaluacion_{run.run_id}"
			{csvColumns}
			caption="Evaluación de salud de los candidatos"
		>
			{#snippet cell({ column, row, value, text })}
				{#if column.key === 'ticker'}
					<UsTicker ticker={row.ticker} />
				{:else if column.key === 'overall'}
					<UsScore score={row.overall} />
				{:else if column.key === 'senal'}
					<SignalBadge signal={scoreSignal(row.overall)} label={row.senal} />
				{:else if column.key.startsWith('c_')}
					<span class={scoreClass(value as number | null)}>{text}</span>
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
		<Notice kind="neutral">
			<p>
				Heurística de reglas simples (múltiplos razonables, rentabilidad positiva, poca deuda, buena
				liquidez, RSI/MACD/tendencia sanos) para comparar rápido entre candidatos. No es
				asesoramiento financiero.
			</p>
		</Notice>
	{/if}
</Section>
