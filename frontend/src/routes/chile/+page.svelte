<script lang="ts">
	import type { ClPilarKey, ClRankingRow } from '$lib/api/types';
	import {
		DataTable,
		ErrorState,
		KpiCard,
		KpiGrid,
		Notice,
		Section,
		TickerLink
	} from '$lib/components/shared';
	import { Badge } from '$lib/components/ui/badge';
	import { fmtDate, fmtNumber, fmtPct } from '$lib/format';
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

	const lider = $derived(filas[0]);

	// Indicadores destacados (sección Indicadores): el mejor valor de cada métrica entre las acciones con dato.
	const indicadores = $derived(data.indicadores.ok ? data.indicadores.data : null);
	function extremo(clave: string, mayor: boolean, soloPositivos = false) {
		let mejor: { simbolo: string; valor: number } | null = null;
		for (const f of indicadores?.filas ?? []) {
			const v = f.valores[clave];
			if (typeof v !== 'number' || !Number.isFinite(v) || (soloPositivos && v <= 0)) continue;
			if (!mejor || (mayor ? v > mejor.valor : v < mejor.valor))
				mejor = { simbolo: f.simbolo, valor: v };
		}
		return mejor;
	}
	const mejorRoe = $derived(extremo('roe', true));
	const mejorDividendo = $derived(extremo('dividend_yield', true));
	const menorPe = $derived(extremo('trailing_pe', false, true));
	const mejorMargen = $derived(extremo('profit_margin', true));
	const cobertura = $derived(
		filas.length ? filas.reduce((acc, f) => acc + f.cobertura, 0) / filas.length : null
	);
	const duplicados = $derived.by(() => {
		const grupos: Record<number, string[]> = {};
		for (const f of filas) {
			if (f.grupo_duplicado !== null) (grupos[f.grupo_duplicado] ??= []).push(f.simbolo);
		}
		return Object.values(grupos);
	});

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
	<Section title="Resumen">
		<KpiGrid>
			<KpiCard label="Acciones analizadas" value={String(filas.length)} />
			<KpiCard
				label="Mejor Score Compuesto"
				value={lider?.simbolo ?? '—'}
				tone="accent"
				hint={lider ? `Score ${fmtNumber(lider.score_compuesto)}` : undefined}
			/>
			<KpiCard
				label="Cobertura promedio de datos"
				value={fmtPct(cobertura, { digits: 0 })}
				tone="blue"
			/>
			<KpiCard
				label="Grupos holding/filial"
				value={String(duplicados.length)}
				tone={duplicados.length ? 'red' : 'text'}
				hint={duplicados.map((g) => g.join(' / ')).join(' · ') || undefined}
			/>
			<KpiCard
				label="Datos del"
				value={fmtDate(ranking.generado_en, true)}
				hint={ranking.generado_desde}
			/>
			{#if mejorRoe}
				<KpiCard
					label="Mayor ROE"
					value={mejorRoe.simbolo}
					hint={fmtPct(mejorRoe.valor, { digits: 1 })}
					tone="green"
				/>
			{/if}
			{#if mejorMargen}
				<KpiCard
					label="Mayor margen neto"
					value={mejorMargen.simbolo}
					hint={fmtPct(mejorMargen.valor, { digits: 1 })}
					tone="green"
				/>
			{/if}
			{#if mejorDividendo}
				<KpiCard
					label="Mayor dividend yield"
					value={mejorDividendo.simbolo}
					hint={fmtPct(mejorDividendo.valor, { digits: 1, isPercent: true })}
					tone="blue"
				/>
			{/if}
			{#if menorPe}
				<KpiCard
					label="Menor P/E (trailing)"
					value={menorPe.simbolo}
					hint="{fmtNumber(menorPe.valor, 1)}x"
					tone="accent"
				/>
			{/if}
		</KpiGrid>
		{#if duplicados.length}
			<Notice kind="warning" title="Estados financieros duplicados">
				<p>
					{duplicados.map((g) => g.join(' y ')).join('; ')} reportan Revenue y EBITDA casi idénticos (típico
					de una holding y su filial). Tenerlas juntas en un portafolio duplica la exposición.
				</p>
			</Notice>
		{/if}
	</Section>

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
		<Notice kind="neutral">
			<p>
				Cada métrica se recorta a los percentiles {fmtPct(ranking.modelo.winsor[0], { digits: 0 })}
				y {fmtPct(ranking.modelo.winsor[1], { digits: 0 })} del universo y se transforma en z-score; los
				múltiplos y la deuda se invierten para que "mayor" siempre sea "mejor". El score de cada pilar
				es el promedio de sus z-scores disponibles y el Score Compuesto, el promedio ponderado de los
				pilares con dato.
			</p>
			<p>
				Los scores son relativos a las acciones analizadas, no una nota absoluta de calidad. Es una
				herramienta de apoyo, no una recomendación de inversión.
			</p>
		</Notice>
	</Section>
{/if}
