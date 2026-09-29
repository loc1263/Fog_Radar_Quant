<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { ErrorState, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { CorridaInfo, EtfTable } from '$lib/modules/etf';
	import { col, csvName, fmtDelta, fmtFechaCorrida } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const rot = $derived(data.rotacion.ok ? data.rotacion.data : null);
	const mantienen = $derived((rot?.mantienen ?? []).map((f) => ({ ...f, rango52: null })));
	const nuevos = $derived((rot?.nuevos ?? []).map((f) => ({ ...f, rango52: null })));
	const salidas = $derived((rot?.salidas ?? []).map((f) => ({ ...f, rango52: null })));
	type Fila = (typeof mantienen)[number];

	const n = (key: string, label: string): Column<Fila> => ({ key, label, numeric: true });
	const delta = (key: string, label: string, digits: number): Column<Fila> => ({
		key,
		label,
		numeric: true,
		signed: true,
		format: (v) => fmtDelta(v, digits)
	});
	const score = (key: string, label: string): Column<Fila> => ({
		key,
		label,
		numeric: true,
		format: (v) => (typeof v === 'number' ? fmtNumber(v, 1) : '—')
	});

	const colsMantienen: Column<Fila>[] = [
		col.ticker<Fila>(),
		col.nombre<Fila>(),
		n('rank_actual', '# Act.'),
		n('rank_previo', '# Prev.'),
		{ ...n('racha', 'Racha'), title: 'Corridas consecutivas en el Top 15' },
		col.rango<Fila>(),
		score('score', 'Score'),
		delta('delta_score', 'ΔScore', 1),
		delta('delta_rank', 'ΔRank', 0)
	];
	const colsNuevos: Column<Fila>[] = [
		col.ticker<Fila>(),
		col.nombre<Fila>(),
		n('rank_actual', '# Act.'),
		col.rango<Fila>(),
		score('score', 'Score'),
		{ key: 'procedencia', label: 'Procedencia', wrap: true }
	];
	const colsSalidas: Column<Fila>[] = [
		col.ticker<Fila>(),
		col.nombre<Fila>(),
		n('rank_previo', '# Prev.'),
		{ ...col.rango<Fila>(), label: 'Rango 52S (al salir)' },
		score('score_previo', 'Score prev.'),
		{ key: 'destino', label: 'Destino', wrap: true }
	];
</script>

<Section title="Rotación vs. corrida anterior">
	{#if !rot}
		{#if !data.rotacion.ok}
			<ErrorState message={data.rotacion.message} code={data.rotacion.code} />
		{/if}
	{:else if !rot.disponible}
		<Notice title="Sin corrida anterior para comparar">
			<p>
				Todavía no hay una corrida archivada. Esta sección se completa automáticamente con la
				próxima corrida del pipeline.
			</p>
		</Notice>
	{:else}
		<CorridaInfo fecha={rot.fecha}
			>comparada con la del {fmtFechaCorrida(rot.fecha_previa)}</CorridaInfo
		>

		<h3 class="flex items-center gap-2 text-sm font-bold text-brand-label">
			Se mantienen en el Top 15 <Badge variant="positive">{mantienen.length}</Badge>
		</h3>
		<EtfTable
			rows={mantienen}
			columns={colsMantienen}
			scoreKey={null}
			initialSort={{ key: 'rank_actual', dir: 'asc' }}
			csvFilename={csvName('etf_rotacion_mantienen', rot.fecha)}
			caption="ETFs que se mantienen en el Top 15"
			emptyText="Ningún ETF se mantuvo (rotación total)."
		/>

		<h3 class="mt-8 flex items-center gap-2 text-sm font-bold text-brand-label">
			Nuevos ingresos <Badge variant="category">{nuevos.length}</Badge>
		</h3>
		<EtfTable
			rows={nuevos}
			columns={colsNuevos}
			scoreKey={null}
			csvFilename={csvName('etf_rotacion_nuevos', rot.fecha)}
			caption="Nuevos ingresos al Top 15"
			emptyText="No hubo nuevos ingresos en esta corrida."
		/>

		<h3 class="mt-8 flex items-center gap-2 text-sm font-bold text-brand-label">
			Salidas del Top 15 <Badge variant="negative">{salidas.length}</Badge>
		</h3>
		<EtfTable
			rows={salidas}
			columns={colsSalidas}
			scoreKey={null}
			csvFilename={csvName('etf_rotacion_salidas', rot.fecha)}
			caption="Salidas del Top 15"
			emptyText="No hubo salidas en esta corrida."
		/>
	{/if}
</Section>
