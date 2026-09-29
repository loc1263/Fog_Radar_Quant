<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { KpiCard, KpiGrid, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { elegirAnalisis, pct, sufijoAnalisis } from '$lib/modules/td/format';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const a = $derived(d ? elegirAnalisis(d, page.url.searchParams.get('analisis')) : null);
	// El contexto de mercado es macro: solo se muestra en el análisis principal.
	const notas = $derived(a?.key === '' ? (d?.market_regime_notes ?? []) : []);
	// TD-DATA-1: aviso visible cuando se excluyen tickers por falta de tipo de cambio.
	const sinFx = $derived((d?.excluidos ?? []).filter((e) => e.motivo.startsWith('falta FX')));
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
		</KpiGrid>
		<p>
			El modelo de Markowitz/covarianza, aplicado sobre el universo '{a.label}' ({a.universe_size}
			acciones, universo {d.universe_source ?? '—'}) con 5 años de historial semanal de precios en
			USD, seleccionó las {a.n_selected} acciones que maximizan el Sharpe Ratio del portafolio.
		</p>
		{#if sinFx.length > 0}
			<Notice kind="warning" title="Tickers excluidos por moneda">
				<p>
					{sinFx.map((e) => `${e.ticker} (${e.nombre})`).join(', ')}: no hay tipo de cambio para
					convertirlos a USD, así que no participan del análisis. Detalle en
					<a href={resolve('/tendencia/limitaciones')} class="font-bold underline">Limitaciones</a>.
				</p>
			</Notice>
		{/if}
		{#if notas.length > 0}
			<Notice title="Contexto de mercado">
				{#each notas as nota, i (i)}<p>{nota}</p>{/each}
			</Notice>
		{/if}
	</Section>
{/if}
