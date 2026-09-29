<script lang="ts">
	import { LineChart } from '$lib/charts';
	import {
		DataTable,
		EmptyState,
		ErrorState,
		Notice,
		Section,
		TickerLink
	} from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { fmtDelta, fmtFechaCorrida, historialConDeltas } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const h = $derived(data.historial.ok ? data.historial.data : null);
	const filas = $derived(historialConDeltas(h?.filas ?? []));
	type Fila = (typeof filas)[number];

	const serie = $derived(
		(h?.filas ?? []).map((f) => ({
			fecha: new Date(`${f.fecha}T12:00:00Z`),
			sharpe: f.sharpe_promedio,
			beta: f.beta_promedio
		}))
	);

	const n3 = (v: unknown) => (typeof v === 'number' ? fmtNumber(v, 3) : '—');
	const columns: Column<Fila>[] = [
		{ key: 'fecha', label: 'Fecha', format: (v) => fmtFechaCorrida(v as string) },
		{
			key: 'universo_bruto',
			label: 'Universo bruto',
			numeric: true,
			format: (v) => fmtNumber(v, 0)
		},
		{
			key: 'universo_depurado',
			label: 'Universo depurado',
			numeric: true,
			format: (v) => fmtNumber(v, 0)
		},
		{
			key: 'n_outliers',
			label: 'Outliers',
			numeric: true,
			format: (v, r) => `${fmtNumber(v, 0)} (${fmtNumber(r.pct_outliers, 1)} %)`
		},
		{ key: 'sharpe_promedio', label: 'Sharpe prom.', numeric: true, format: n3 },
		{
			key: 'delta_sharpe',
			label: 'Δ Sharpe',
			numeric: true,
			signed: true,
			format: (v) => fmtDelta(v, 3)
		},
		{ key: 'beta_promedio', label: 'Beta prom.', numeric: true, format: n3 },
		{
			key: 'delta_beta',
			label: 'Δ Beta',
			numeric: true,
			signed: true,
			format: (v) => fmtDelta(v, 3)
		},
		{ key: 'top1_ticker', label: 'Ticker #1' },
		{ key: 'top1_score', label: 'Score #1', numeric: true, format: (v) => fmtNumber(v, 1) }
	];
</script>

<Section title="Historial de corridas">
	{#if !h}
		{#if !data.historial.ok}
			<ErrorState message={data.historial.message} code={data.historial.code} />
		{/if}
	{:else if h.filas.length === 0}
		<EmptyState>
			Todavía no hay historial de corridas (se completa a partir de la primera corrida procesada).
		</EmptyState>
	{:else}
		<h3 class="text-sm font-bold text-brand-label">
			Evolución de Sharpe y Beta promedio (universo depurado)
		</h3>
		<LineChart
			data={serie}
			x="fecha"
			series={[
				{ key: 'sharpe', label: 'Sharpe promedio' },
				{ key: 'beta', label: 'Beta promedio' }
			]}
			yFormat={(v) => fmtNumber(v, 2)}
			xFormat={(d) =>
				d.toLocaleDateString('es-CL', { day: '2-digit', month: 'short', timeZone: 'UTC' })}
		/>
		<Notice>
			<p>
				Corrida más reciente primero. Las columnas Δ comparan cada corrida contra la inmediatamente
				anterior (no contra la primera del historial).
			</p>
		</Notice>
		<DataTable rows={filas} {columns} csvFilename="etf_historial_corridas" caption="Historial">
			{#snippet cell({ column, row, text })}
				{#if column.key === 'top1_ticker' && row.top1_ticker}
					<TickerLink ticker={row.top1_ticker} />
				{:else if column.key === 'top1_score'}
					<span class="font-bold text-brand-accent">{text}</span>
				{:else if column.key === 'fecha'}
					<span class="font-bold">{text}</span>
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	{/if}
</Section>
