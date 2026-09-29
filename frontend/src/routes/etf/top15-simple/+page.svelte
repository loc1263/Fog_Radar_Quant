<script lang="ts">
	import { ErrorState, Notice, Section } from '$lib/components/shared';
	import { CorridaInfo, EtfTable } from '$lib/modules/etf';
	import { col, conPosicion, csvColumnas, csvName } from '$lib/modules/etf/format';

	let { data } = $props();

	const t = $derived(data.tabla.ok ? data.tabla.data : null);
	const filas = $derived(conPosicion(t?.filas ?? []));
	type Fila = (typeof filas)[number];

	const columns = [
		col.pos<Fila>(),
		col.ticker<Fila>(),
		col.nombre<Fila>(),
		...col.periodos<Fila>(),
		col.beta<Fila>(),
		col.score<Fila>('score_simple', 'Score simple')
	];
	const csvColumns = csvColumnas<Fila>('score_simple', 'Score Simple').filter(
		(c) => c.key !== 'sharpe'
	);
</script>

<Section title="Top 15 — rendimiento y Beta">
	{#if !t}
		{#if !data.tabla.ok}<ErrorState message={data.tabla.message} code={data.tabla.code} />{/if}
	{:else}
		<CorridaInfo fecha={t.fecha} />
		<Notice>
			<p>
				Ranking considerando únicamente el rendimiento por período (5D, 1M, 6M, 1Y) y el Beta (menor
				|Beta| = mejor), 20 % cada uno, sobre el mismo universo depurado del ranking principal.
			</p>
		</Notice>
		<EtfTable
			rows={filas}
			{columns}
			scoreKey="score_simple"
			csvFilename={csvName('etf_top15_simple', t.fecha)}
			{csvColumns}
			caption="Top 15 por rendimiento y Beta"
			emptyText="Sin datos."
		/>
	{/if}
</Section>
