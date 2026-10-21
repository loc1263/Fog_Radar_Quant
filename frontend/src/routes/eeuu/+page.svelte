<script lang="ts">
	import { ErrorState, KpiCard, KpiGrid, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { RunInfo } from '$lib/modules/us';
	import { fmtRet, fmtScore, fmtShare, isGrowingPerformance } from '$lib/modules/us/format';

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
</script>

{#if !run}
	{#if !data.ranking.ok}
		<ErrorState
			title={data.ranking.code === 'no_data' ? 'Aún no hay corridas del screener' : undefined}
			message={data.ranking.message}
			code={data.ranking.code}
		/>
	{/if}
{:else}
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
	<RunInfo {run} />
{/if}
