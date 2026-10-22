<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { keptQuery } from '$lib/nav';
	import { KpiCard, KpiGrid, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { elegirAnalisis, pct, sufijoAnalisis } from '$lib/modules/td/format';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const a = $derived(d ? elegirAnalisis(d, page.url.searchParams.get('analisis')) : null);
	// El contexto de mercado es macro: solo se muestra en el análisis principal.
	const q = $derived(keptQuery(page.url, ['analisis']));
	const notas = $derived(a?.key === '' ? (d?.market_regime_notes ?? []) : []);
	// TD-DATA-1: aviso visible cuando se excluyen tickers por falta de tipo de cambio.
	const sinFx = $derived((d?.excluidos ?? []).filter((e) => e.motivo.startsWith('falta FX')));

	// Un indicador por sección del menú (Portafolio, Tendencia 6m, Sectores, Momentum, Limitaciones).
	const mayorPeso = $derived(a?.holdings[0] ?? null);
	const alcistas = $derived(
		(a?.holdings ?? []).filter((h) => h.senal_tecnica === 'Alcista').length
	);
	const retorno6m = $derived.by(() => {
		const v = (a?.holdings ?? [])
			.map((h) => h.retorno_esperado_6m_pct)
			.filter((x): x is number => typeof x === 'number');
		return v.length ? v.reduce((s, x) => s + x, 0) / v.length : null;
	});
	const sectorTop = $derived(
		a?.sectors.length ? a.sectors.reduce((m, s) => (s.peso_total > m.peso_total ? s : m)) : null
	);
	const liderMensual = $derived(a?.momentum[0] ?? null);
	const liderSemanal = $derived(a?.momentum_semanal[0] ?? null);
	const masSube = $derived(a?.retorno_reciente[0] ?? null);
</script>

{#if d && a}
	{@const perf = a.performance}
	<Section title="Resumen ejecutivo{sufijoAnalisis(a)}">
		<KpiGrid>
			<KpiCard
				label="Retorno esperado anual"
				value={pct(perf.portafolio_15_finales.retorno_esperado_anual)}
				tone="green"
			/>
			<KpiCard
				label="Volatilidad anual"
				value={pct(perf.portafolio_15_finales.volatilidad_anual)}
				tone="blue"
			/>
			<KpiCard
				label="Sharpe Ratio (portafolio)"
				value={fmtNumber(perf.portafolio_15_finales.sharpe_ratio, 2)}
				tone="accent"
			/>
			<KpiCard
				label="Sharpe Ratio (universo)"
				value={fmtNumber(perf.universo_completo.sharpe_ratio, 2)}
			/>
			<KpiCard label="Tasa libre de riesgo" value={pct(perf.tasa_libre_riesgo_anual, 2)} />
			{#if mayorPeso}
				<KpiCard
					label="Portafolio: mayor peso"
					value={mayorPeso.ticker}
					hint="{pct(mayorPeso.peso_final)} · {a.n_selected} acciones"
					tone="accent"
					href="/tendencia/portafolio"
					search={q}
				/>
			{/if}
			{#if a.holdings.length}
				<KpiCard
					label="Tendencia 6m: señal alcista"
					value="{alcistas} de {a.holdings.length}"
					hint={retorno6m === null
						? undefined
						: `retorno esperado 6m: ${fmtNumber(retorno6m, 1)} %`}
					tone="green"
					href="/tendencia/proyeccion"
					search={q}
				/>
			{/if}
			{#if sectorTop}
				<KpiCard
					label="Sectores: mayor peso"
					value={sectorTop.sector}
					hint="{pct(sectorTop.peso_total)} · {a.sectors.length} sectores"
					tone="blue"
					href="/tendencia/sectores"
					search={q}
				/>
			{/if}
			{#if liderMensual}
				<KpiCard
					label="Momentum mensual: líder"
					value={liderMensual.ticker}
					hint="score {fmtNumber(liderMensual.momentum_score_pct, 1)} %"
					href="/tendencia/momentum"
					search={q}
				/>
			{/if}
			{#if liderSemanal}
				<KpiCard
					label="Momentum semanal: líder"
					value={liderSemanal.ticker}
					hint="score {fmtNumber(liderSemanal.momentum_score_pct, 1)} %"
					href="/tendencia/momentum"
					search={q}
				/>
			{/if}
			{#if masSube}
				<KpiCard
					label="Mayor retorno reciente"
					value={masSube.ticker}
					hint="{fmtNumber(masSube.retorno_pct, 1)} %"
					tone="green"
				/>
			{/if}
			<KpiCard
				label="Limitaciones: tickers excluidos"
				value={fmtNumber(d.excluidos.length, 0)}
				hint={sinFx.length ? `${sinFx.length} por falta de tipo de cambio` : undefined}
				tone={d.excluidos.length ? 'red' : 'text'}
				href="/tendencia/limitaciones"
				search={q}
			/>
		</KpiGrid>
		<p class="text-xs text-brand-muted">
			Markowitz sobre el universo '{a.label}' ({a.universe_size} acciones, {d.universe_source ??
				'—'}), con 5 años de precios semanales en USD: seleccionó las {a.n_selected} acciones que maximizan
			el Sharpe del portafolio.
		</p>
		{#if sinFx.length > 0}
			<Notice size="compact" kind="warning" title="Tickers excluidos por moneda">
				<p>
					{sinFx.map((e) => `${e.ticker} (${e.nombre})`).join(', ')}: no hay tipo de cambio para
					convertirlos a USD, así que no participan del análisis. Detalle en
					<a href={resolve('/tendencia/limitaciones')} class="font-bold underline">Limitaciones</a>.
				</p>
			</Notice>
		{/if}
		{#if notas.length > 0}
			<Notice size="compact" title="Contexto de mercado">
				{#each notas as nota, i (i)}<p>{nota}</p>{/each}
			</Notice>
		{/if}
	</Section>
{/if}
