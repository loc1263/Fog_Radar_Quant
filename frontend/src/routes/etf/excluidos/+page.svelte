<script lang="ts">
	import { ErrorState, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { CorridaInfo, EtfTable } from '$lib/modules/etf';
	import { col, csvName } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const t = $derived(data.tabla.ok ? data.tabla.data : null);
	const filas = $derived(t?.filas ?? []);
	type Fila = (typeof filas)[number];

	const columns: Column<Fila>[] = [
		col.ticker<Fila>(),
		col.nombre<Fila>(),
		...col.periodos<Fila>(),
		col.sharpe<Fila>(),
		col.beta<Fila>(),
		{ key: 'ventana_atipica', label: 'Ventana atípica' },
		{
			key: 'z_score',
			label: 'Z-score',
			numeric: true,
			signed: true,
			format: (v) => (typeof v === 'number' ? fmtNumber(v, 2) : '—')
		}
	];
</script>

<Section title="Excluidos por distorsión estadística">
	{#if !t}
		{#if !data.tabla.ok}<ErrorState message={data.tabla.message} code={data.tabla.code} />{/if}
	{:else}
		<CorridaInfo fecha={t.fecha}>{filas.length} ETFs excluidos</CorridaInfo>
		<Notice>
			<p>
				ETFs con al menos un rendimiento (5D, 1M, 6M o 1Y) a más de 6 desviaciones robustas (z-score
				MAD) de la mediana del universo. La exclusión responde a una distorsión estadística frente a
				un universo diversificado y no implica necesariamente mala calidad del fondo.
			</p>
		</Notice>
		<EtfTable
			rows={filas}
			{columns}
			scoreKey={null}
			csvFilename={csvName('etf_excluidos_outliers', t.fecha)}
			caption="Excluidos por distorsión"
			emptyText="Ningún ETF fue excluido por distorsión en esta corrida."
		/>
	{/if}
</Section>
