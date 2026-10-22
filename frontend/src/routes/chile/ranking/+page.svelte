<script lang="ts">
	import type { ClPilarKey, ClRankingRow } from '$lib/api/types';
	import { DataTable, ErrorState, Notice, Section, TickerLink } from '$lib/components/shared';
	import { Badge } from '$lib/components/ui/badge';
	import { fmtNumber, fmtPct } from '$lib/format';
	import { DetalleAccion, PanelPesos, ZScore } from '$lib/modules/cl';
	import { fmtZ } from '$lib/modules/cl/format';
	import {
		PILAR_KEYS,
		pesosDesdeModelo,
		recalcularRanking,
		type Pesos
	} from '$lib/modules/cl/scoring';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const ranking = $derived(data.ranking.ok ? data.ranking.data : null);
	const pilares = $derived(ranking?.modelo.pilares ?? []);
	const defaults = $derived(pesosDesdeModelo(pilares));

	// null = pesos por defecto del modelo; se guarda solo lo que el usuario cambia.
	let pesosUsuario = $state<Pesos | null>(null);
	const pesos = $derived(pesosUsuario ?? defaults);

	let sector = $state('todos');

	type Fila = ClRankingRow & Record<`p_${ClPilarKey}`, number | null> & { cobertura: number };

	// Ranking sobre todo el universo; el filtro de sector solo oculta filas
	// (el puesto de cada acción no cambia al filtrar).
	const filas = $derived.by((): Fila[] =>
		recalcularRanking(ranking?.filas ?? [], pesos).map((f) => ({
			...f,
			...(Object.fromEntries(PILAR_KEYS.map((k) => [`p_${k}`, f.pilares[k]])) as Record<
				`p_${ClPilarKey}`,
				number | null
			>),
			cobertura: f.metricas_totales > 0 ? f.metricas_disponibles / f.metricas_totales : 0
		}))
	);
	const sectores = $derived(
		[...new Set(filas.map((f) => f.sector ?? 'Sin sector'))].sort((a, b) =>
			a.localeCompare(b, 'es')
		)
	);
	const visibles = $derived(
		sector === 'todos' ? filas : filas.filter((f) => (f.sector ?? 'Sin sector') === sector)
	);

	const columns = $derived<Column<Fila>[]>([
		{ key: 'ranking', label: '#', numeric: true },
		{ key: 'simbolo', label: 'Acción', wrap: true, csv: (v) => String(v) },
		{ key: 'nombre', label: 'Nombre', wrap: true },
		{ key: 'sector', label: 'Sector', wrap: true },
		{
			key: 'score_compuesto',
			label: 'Score',
			numeric: true,
			signed: true,
			format: (v) => fmtNumber(v),
			title: 'Promedio ponderado de los pilares disponibles (z-scores relativos al IPSA)'
		},
		...pilares.map((p): Column<Fila> => ({
			key: `p_${p.key}`,
			label: p.label,
			numeric: true,
			format: (v) => fmtZ(v as number | null),
			title: `z-score del pilar ${p.label} (peso ${fmtNumber(pesos[p.key])})`
		})),
		{
			key: 'cobertura',
			label: 'Cobertura',
			numeric: true,
			format: (_, r) => `${r.metricas_disponibles}/${r.metricas_totales}`,
			title: 'Métricas con dato sobre el total del modelo'
		}
	]);

	function cambiarPeso(key: ClPilarKey, value: number) {
		pesosUsuario = { ...pesos, [key]: value };
	}
</script>

{#if !ranking}
	{#if !data.ranking.ok}
		<ErrorState
			title={data.ranking.code === 'no_data' ? 'Aún no hay ranking del IPSA' : undefined}
			message={data.ranking.message}
			code={data.ranking.code}
		/>
	{/if}
{:else}
	<Section title="Pesos del modelo">
		<PanelPesos
			{pilares}
			{pesos}
			{defaults}
			onchange={cambiarPeso}
			onreset={() => (pesosUsuario = null)}
		/>
	</Section>

	<Section title="Ranking IPSA">
		<label class="block w-fit">
			<span class="mb-1 block text-sm font-bold text-brand-label">Sector</span>
			<select
				bind:value={sector}
				class="rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm"
			>
				<option value="todos">Todos los sectores</option>
				{#each sectores as s (s)}
					<option value={s}>{s}</option>
				{/each}
			</select>
		</label>
		<DataTable
			rows={visibles}
			{columns}
			csvFilename="ranking_ipsa"
			caption="Ranking multi-factor del IPSA"
			rowKey={(r) => r.simbolo}
			detailLabel={(r) => `Ver métricas de ${r.simbolo}`}
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'simbolo'}
					<span class="inline-flex flex-wrap items-center gap-1.5">
						<TickerLink ticker={row.simbolo} />
						{#if row.grupo_duplicado !== null}
							<Badge
								variant="mixed"
								title="Estados financieros casi idénticos a otro ticker (holding/filial)"
								>grupo {row.grupo_duplicado}</Badge
							>
						{/if}
					</span>
				{:else if column.key.startsWith('p_')}
					<ZScore value={row[column.key as `p_${ClPilarKey}`]} />
				{:else}
					{text}
				{/if}
			{/snippet}
			{#snippet detail({ row })}
				<DetalleAccion fila={row} />
			{/snippet}
		</DataTable>
	</Section>

	<Section title="Metodología">
		<Notice size="compact" kind="neutral">
			<p>
				Cada métrica se recorta a los percentiles {fmtPct(ranking.modelo.winsor[0], { digits: 0 })}
				y {fmtPct(ranking.modelo.winsor[1], { digits: 0 })} y pasa a z-score (múltiplos y deuda se invierten:
				"mayor" = "mejor"). Score por pilar = promedio de sus z-scores; Score Compuesto = promedio ponderado
				de los pilares con dato.
			</p>
			<p>
				Scores relativos a las acciones analizadas, no una nota absoluta. Herramienta de apoyo, no
				recomendación de inversión.
			</p>
		</Notice>
	</Section>
{/if}
