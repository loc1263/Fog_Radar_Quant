<!-- Datos de la corrida batch que se está mostrando. -->
<script lang="ts">
	import type { UsRunResponse } from '$lib/api/types';
	import { KpiCard, KpiGrid, Notice } from '$lib/components/shared';
	import { fmtDate } from '$lib/format';
	import { fmtUsd } from './format';

	let { run }: { run: UsRunResponse } = $props();
</script>

<KpiGrid>
	<KpiCard label="Corrida" value={fmtDate(run.finished_at, true)} hint={run.run_id} />
	<KpiCard label="Universo (S&P 500 ∪ Nasdaq-100)" value={String(run.universe_total)} />
	<KpiCard
		label="Candidatos"
		value={String(run.candidates_total)}
		tone="accent"
		hint="Market cap ≥ {fmtUsd(run.min_market_cap)}"
	/>
	<KpiCard label="Evaluados con momentum" value={String(run.rows.length)} tone="blue" />
</KpiGrid>
{#if run.status === 'error'}
	<Notice kind="warning" title="La corrida terminó con error">
		<p>{run.error ?? 'Error desconocido.'} Se muestran los datos que alcanzó a evaluar.</p>
	</Notice>
{/if}
