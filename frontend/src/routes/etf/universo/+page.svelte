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
		col.sharpe<Fila>(),
		col.beta<Fila>(),
		col.rango<Fila>(),
		col.score<Fila>()
	];
</script>

<Section title="Universo depurado completo">
	{#if !t}
		{#if !data.tabla.ok}<ErrorState message={data.tabla.message} code={data.tabla.code} />{/if}
	{:else}
		<CorridaInfo fecha={t.fecha}>{filas.length} ETFs</CorridaInfo>
		<Notice>
			<p>
				Universo depurado completo (sin recortar al Top 15), ordenado por Score. Uso principal:
				buscar un ticker que no llegó al Top 15 y ver dónde quedó rankeado (columna #).
			</p>
		</Notice>
		<EtfTable
			rows={filas}
			{columns}
			searchKeys={['ticker', 'nombre', 'categoria']}
			searchPlaceholder="Ticker, nombre o categoría…"
			pageSize={50}
			csvFilename={csvName('etf_ranking_completo', t.fecha)}
			csvColumns={csvColumnas<Fila>()}
			caption="Universo depurado"
		/>
	{/if}
</Section>
