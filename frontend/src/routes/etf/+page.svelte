<script lang="ts">
	import { LineChart } from '$lib/charts';
	import { ErrorState, KpiCard, KpiGrid, Notice, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { CorridaInfo } from '$lib/modules/etf';
	import { fmtFechaCorrida } from '$lib/modules/etf/format';

	let { data } = $props();

	const resumen = $derived(data.resumen.ok ? data.resumen.data : null);
	const r = $derived(resumen?.resumen ?? null);
	const top5 = $derived(data.top15.ok ? data.top15.data.filas.slice(0, 5) : []);

	const PERIODOS = [
		{ key: 'perf_5d', label: '5D' },
		{ key: 'perf_1m', label: '1M' },
		{ key: 'perf_6m', label: '6M' },
		{ key: 'perf_1y', label: '1Y' }
	] as const;

	// Una fila por ventana, una serie por ETF del Top 5 (TendenciaChart del original).
	const tendencia = $derived(
		PERIODOS.map(({ key, label }) => ({
			periodo: label,
			...Object.fromEntries(top5.map((f) => [f.ticker, f[key] ?? null]))
		}))
	);
	const series = $derived(top5.map((f) => ({ key: f.ticker, label: f.ticker })));
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
			/>
			<KpiCard label="Fecha de la corrida" value={fmtFechaCorrida(resumen.fecha)} />
			<KpiCard label="Universo ETF bruto" value={fmtNumber(r?.universo_bruto, 0)} />
			<KpiCard
				label="Universo depurado"
				value={fmtNumber(r?.universo_depurado, 0)}
				hint="{fmtNumber(r?.n_outliers, 0)} excluidos por distorsión"
			/>
			<KpiCard label="Sharpe promedio" value={fmtNumber(r?.sharpe_promedio, 3)} tone="blue" />
			<KpiCard label="Beta promedio" value={fmtNumber(r?.beta_promedio, 3)} />
		</KpiGrid>
	{/if}
</Section>

{#if resumen && top5.length > 0}
	<Section title="Tendencia del Top 5 por ventana de rendimiento">
		<LineChart
			data={tendencia}
			x="periodo"
			categorical
			{series}
			height={280}
			zeroBaseline
			yFormat={(v) => `${fmtNumber(v, 0)} %`}
		/>
	</Section>
{/if}

{#if resumen}
	{@const m = resumen.modelo}
	<Section title="Metodología">
		<Notice kind="neutral">
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
