<script lang="ts">
	import { BarChart } from '$lib/charts';
	import { DataTable, ErrorState, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { CorridaInfo } from '$lib/modules/etf';
	import { csvName } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const d = $derived(data.div.ok ? data.div.data : null);
	const filas = $derived(d?.filas ?? []);
	type Fila = (typeof filas)[number];
	const top = $derived(filas[0]);

	const columns: Column<Fila>[] = [
		{ key: 'categoria', label: 'Categoría' },
		{ key: 'conteo', label: 'N ETFs', numeric: true },
		{
			key: 'pct',
			label: '% del Top 15',
			numeric: true,
			format: (v) => `${fmtNumber(v, 1)} %`
		}
	];
</script>

<Section title="Diversificación del Top 15 por categoría">
	{#if !d}
		{#if !data.div.ok}<ErrorState message={data.div.message} code={data.div.code} />{/if}
	{:else}
		<CorridaInfo fecha={d.fecha} />
		{#if top}
			<Notice size="compact">
				<p>
					La categoría más representada en el Top 15 es <b>{top.categoria}</b> con {fmtNumber(
						top.pct,
						1
					)} % del Top 15.
					{#if top.pct >= 40}
						El Score compuesto no penaliza la concentración sectorial: esta tabla es solo
						informativa.
					{/if}
				</p>
			</Notice>
		{/if}
		<BarChart
			data={filas}
			category="categoria"
			series={[{ key: 'pct', label: '% del Top 15', color: 'var(--color-brand-blue)' }]}
			labelWidth={190}
			height={Math.max(220, filas.length * 34)}
			valueFormat={(v) => `${fmtNumber(v, 0)} %`}
		/>
		<DataTable
			rows={filas}
			{columns}
			csvFilename={csvName('etf_diversificacion_top15', d.fecha)}
			caption="Diversificación del Top 15"
		/>
	{/if}
</Section>
