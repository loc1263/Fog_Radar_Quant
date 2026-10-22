<script lang="ts">
	import { ErrorState, KpiCard, KpiGrid, Notice, Section } from '$lib/components/shared';
	import { fmtDate, fmtNumber, fmtPct } from '$lib/format';

	let { data } = $props();

	const ranking = $derived(data.ranking.ok ? data.ranking.data : null);
	const filas = $derived(ranking?.filas ?? []);
	const lider = $derived(filas[0]);
	const cobertura = $derived(
		filas.length
			? filas.reduce(
					(acc, f) =>
						acc + (f.metricas_totales > 0 ? f.metricas_disponibles / f.metricas_totales : 0),
					0
				) / filas.length
			: null
	);
	const duplicados = $derived.by(() => {
		const grupos: Record<number, string[]> = {};
		for (const f of filas) {
			if (f.grupo_duplicado !== null) (grupos[f.grupo_duplicado] ??= []).push(f.simbolo);
		}
		return Object.values(grupos);
	});

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
				href="/chile/ranking"
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
					href="/chile/indicadores"
				/>
			{/if}
			{#if mejorMargen}
				<KpiCard
					label="Mayor margen neto"
					value={mejorMargen.simbolo}
					hint={fmtPct(mejorMargen.valor, { digits: 1 })}
					tone="green"
					href="/chile/indicadores"
				/>
			{/if}
			{#if mejorDividendo}
				<KpiCard
					label="Mayor dividend yield"
					value={mejorDividendo.simbolo}
					hint={fmtPct(mejorDividendo.valor, { digits: 1, isPercent: true })}
					tone="blue"
					href="/chile/indicadores"
				/>
			{/if}
			{#if menorPe}
				<KpiCard
					label="Menor P/E (trailing)"
					value={menorPe.simbolo}
					hint="{fmtNumber(menorPe.valor, 1)}x"
					tone="accent"
					href="/chile/indicadores"
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
{/if}
