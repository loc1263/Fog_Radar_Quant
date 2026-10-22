<script lang="ts">
	import { ErrorState, KpiCard, KpiGrid, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { CorridaInfo } from '$lib/modules/etf';
	import { fmtFechaCorrida, fmtRend } from '$lib/modules/etf/format';

	let { data } = $props();

	const resumen = $derived(data.resumen.ok ? data.resumen.data : null);
	const r = $derived(resumen?.resumen ?? null);
	const top5 = $derived(data.top15.ok ? data.top15.data.filas.slice(0, 5) : []);

	// Un indicador por sección del menú (null = esa sección no tiene datos).
	const rot = $derived(
		data.rotacion.ok && data.rotacion.data.disponible ? data.rotacion.data : null
	);
	const div = $derived(
		data.diversificacion.ok
			? [...data.diversificacion.data.filas].sort((a, b) => b.conteo - a.conteo)
			: []
	);
	const cats = $derived(data.categorias.ok ? data.categorias.data.filas : []);
	const mejorCategoria = $derived(
		cats.length
			? cats.reduce((m, c) =>
					(c.score_promedio ?? -Infinity) > (m.score_promedio ?? -Infinity) ? c : m
				)
			: null
	);
	const simple = $derived(data.simple.ok ? data.simple.data.filas[0] : null);
	const vanguard = $derived(data.vanguard.ok ? data.vanguard.data.filas : []);
	const momentum = $derived(data.momentum.ok ? data.momentum.data.filas[0] : null);
	const cap = $derived(data.cap.ok ? data.cap.data : null);
</script>

<Section title="Resumen de la corrida">
	{#if !resumen}
		{#if !data.resumen.ok}
			<ErrorState
				title={data.resumen.code === 'no_data' ? 'Aún no hay corridas de ETFs' : undefined}
				message={data.resumen.message}
				code={data.resumen.code}
			/>
		{/if}
	{:else}
		<CorridaInfo fecha={resumen.fecha}>archivo {resumen.archivo_csv}</CorridaInfo>
		<KpiGrid>
			<KpiCard
				label="Top 5 del Top {resumen.modelo.top_n}"
				value={top5.length ? top5.map((f) => f.ticker).join(' · ') : (r?.top1_ticker ?? '—')}
				tone="accent"
				href="/etf/top15"
			/>
			<KpiCard label="Fecha de la corrida" value={fmtFechaCorrida(resumen.fecha)} />
			<KpiCard label="Universo ETF bruto" value={fmtNumber(r?.universo_bruto, 0)} />
			<KpiCard
				label="Universo depurado"
				value={fmtNumber(r?.universo_depurado, 0)}
				hint="ranking completo"
				href="/etf/universo"
			/>
			<KpiCard
				label="Excluidos por distorsión"
				value={fmtNumber(r?.n_outliers, 0)}
				hint="|z-score MAD| &gt; {fmtNumber(resumen.modelo.z_threshold, 0)}"
				tone="red"
				href="/etf/excluidos"
			/>
			<KpiCard label="Sharpe promedio" value={fmtNumber(r?.sharpe_promedio, 3)} tone="blue" />
			<KpiCard label="Beta promedio" value={fmtNumber(r?.beta_promedio, 3)} />
			{#if rot}
				<KpiCard
					label="Rotación del Top 15"
					value="{rot.nuevos.length} entran · {rot.salidas.length} salen"
					hint="vs. corrida del {fmtFechaCorrida(rot.fecha_previa)}"
					tone="accent"
					href="/etf/rotacion"
				/>
			{/if}
			{#if div.length}
				<KpiCard
					label="Diversificación del Top 15"
					value="{div.length} categorías"
					hint="más presente: {div[0].categoria} ({fmtNumber(div[0].pct, 0)} %)"
					href="/etf/diversificacion"
				/>
			{/if}
			{#if mejorCategoria}
				<KpiCard
					label="Categorías analizadas"
					value={fmtNumber(cats.length, 0)}
					hint="mejor score: {mejorCategoria.categoria} ({fmtNumber(
						mejorCategoria.score_promedio,
						1
					)})"
					tone="blue"
					href="/etf/categorias"
				/>
			{/if}
			{#if simple}
				<KpiCard
					label="Top 15 Perf. + Beta"
					value={simple.ticker}
					hint="1 año: {fmtRend(simple.perf_1y)}"
					href="/etf/top15-simple"
				/>
			{/if}
			{#if vanguard.length}
				<KpiCard
					label="Top 15 Vanguard"
					value={vanguard[0].ticker}
					hint="1 año: {fmtRend(vanguard[0].perf_1y)}"
					href="/etf/top15-vanguard"
				/>
			{/if}
			{#if momentum}
				<KpiCard
					label="Líder de Momentum"
					value={momentum.ticker}
					hint="1 mes: {fmtRend(momentum.perf_1m)}"
					tone="green"
					href="/etf/momentum"
				/>
			{/if}
			{#if cap}
				<KpiCard
					label="Benchmark Russell 3000 (1A)"
					value={fmtRend(cap.benchmark_retorno_1y)}
					hint="{cap.benchmark_ticker} · {cap.buckets.length} tramos de capitalización"
					signedValue={cap.benchmark_retorno_1y}
					href="/etf/top15-cap"
				/>
			{/if}
		</KpiGrid>
	{/if}
</Section>

{#if resumen}
	{@const m = resumen.modelo}
	<Section title="Metodología">
		<Notice size="compact" kind="neutral">
			<p>
				<b>Universo:</b> solo ETFs con datos completos, volumen ≥ {fmtNumber(m.liq_vol_min, 0)} y AUM
				≥ US$ {fmtNumber(m.liq_aum_min / 1e6, 0)} M. Se excluyen las categorías {m.categorias_excluidas.join(
					', '
				)} y los ETFs con algún rendimiento atípico (|z-score MAD| &gt; {fmtNumber(
					m.z_threshold,
					0
				)}).
			</p>
			<p>
				<b>Score compuesto</b> (percentiles 0-100 ponderados):
				{m.pesos.map((p) => `${p.etiqueta} ${fmtNumber(p.peso * 100, 2)} %`).join(' · ')}.
			</p>
			<p>Análisis cuantitativo con fines informativos; no constituye recomendación de inversión.</p>
		</Notice>
	</Section>
{/if}
