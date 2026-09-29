<script lang="ts">
	import { ErrorState, Section } from '$lib/components/shared';
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
		col.sharpe<Fila>(),
		col.beta<Fila>(),
		col.rango<Fila>(),
		col.score<Fila>()
	];
</script>

<Section title="Top 15 ETFs recomendados">
	{#if !t}
		{#if !data.tabla.ok}<ErrorState message={data.tabla.message} code={data.tabla.code} />{/if}
	{:else}
		<CorridaInfo fecha={t.fecha}>ranking por rendimiento ajustado al riesgo</CorridaInfo>
		<EtfTable
			rows={filas}
			{columns}
			csvFilename={csvName('etf_top15_recomendados', t.fecha)}
			csvColumns={csvColumnas<Fila>()}
			caption="Top 15 ETFs recomendados"
			emptyText="Sin datos."
		/>
	{/if}
</Section>
