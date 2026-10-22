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
		col.score<Fila>('score_comp')
	];
</script>

<Section title="Top 15 Momentum / crecimiento">
	{#if !t}
		{#if !data.tabla.ok}
			<ErrorState
				title="El análisis de Momentum no está disponible"
				message={data.tabla.message}
				code={data.tabla.code}
			/>
		{/if}
	{:else}
		<CorridaInfo fecha={t.fecha} />
		<Notice size="compact">
			<p>
				Complemento al ranking principal, orientado a momentum. Score = 55 % momentum ponderado (5D
				10 % · 1M 20 % · 6M 30 % · 1Y 40 %) + 25 % calidad (Sharpe/Treynor) + 20 % consistencia del
				6M frente a la corrida archivada más antigua.
			</p>
		</Notice>
		<EtfTable
			rows={filas}
			{columns}
			scoreKey="score_comp"
			csvFilename={csvName('etf_momentum_top15', t.fecha)}
			csvColumns={csvColumnas<Fila>('score_comp', 'Score_comp')}
			caption="Top 15 Momentum"
			emptyText="El análisis de Momentum/Crecimiento no se corrió en esta corrida."
		/>
	{/if}
</Section>
