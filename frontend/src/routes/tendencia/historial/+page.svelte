<script lang="ts">
	import { resolve } from '$app/paths';
	import type { TdMomentumLider, TdRunPerformance } from '$lib/api/types';
	import { LineChart } from '$lib/charts';
	import {
		DataTable,
		EmptyState,
		ErrorState,
		KpiCard,
		KpiGrid,
		Section,
		TickerLink
	} from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import {
		etiquetaCorrida,
		fechaCorta,
		money,
		pct,
		pctPts,
		pivotPorGrupo
	} from '$lib/modules/td/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const f = $derived(data.filtros);
	const runs = $derived(data.runs.ok ? data.runs.data : []);
	const analisisDisponibles = $derived(data.analyses.ok ? data.analyses.data : []);

	const primera = $derived(runs[0]);
	const ultima = $derived(runs.at(-1));

	const serieSharpe = $derived(
		runs.map((r) => ({
			corrida: etiquetaCorrida(r.run_dir),
			portafolio: r.sharpe_portafolio,
			universo: r.sharpe_universo
		}))
	);
	const serieRetVol = $derived(
		runs.map((r) => ({
			corrida: etiquetaCorrida(r.run_dir),
			retorno: (r.retorno_portafolio_anual ?? 0) * 100,
			volatilidad: (r.volatilidad_portafolio_anual ?? 0) * 100
		}))
	);
	const sectores = $derived(
		pivotPorGrupo(
			data.sectors,
			(r) => r.sector,
			(r) => (r.peso_total ?? 0) * 100
		)
	);
	const trayectoria = $derived(data.trayectoria?.ok ? data.trayectoria.data : []);
	const seriePeso = $derived(
		trayectoria.map((h) => ({
			corrida: etiquetaCorrida(h.run_dir),
			peso: (h.peso_final ?? 0) * 100
		}))
	);
	const seriePrecio = $derived(
		trayectoria.map((h) => ({
			corrida: etiquetaCorrida(h.run_dir),
			actual: h.precio_actual_usd,
			proyectado: h.precio_proyectado_6m_usd
		}))
	);

	const colRuns: Column<TdRunPerformance>[] = [
		{ key: 'run_date', label: 'Fecha', format: (v) => fechaCorta(v as string) },
		{ key: 'run_dir', label: 'Corrida' },
		{ key: 'universe_size', label: 'Universo', numeric: true },
		{ key: 'n_selected', label: 'Seleccionadas', numeric: true },
		{
			key: 'sharpe_portafolio',
			label: 'Sharpe portafolio',
			numeric: true,
			format: (v) => fmtNumber(v, 2)
		},
		{
			key: 'retorno_portafolio_anual',
			label: 'Retorno anual',
			numeric: true,
			format: (v) => pct(v)
		},
		{
			key: 'volatilidad_portafolio_anual',
			label: 'Volatilidad anual',
			numeric: true,
			format: (v) => pct(v)
		}
	];
	const colLeaders: Column<TdMomentumLider>[] = [
		{ key: 'run_dir', label: 'Corrida', format: (v) => etiquetaCorrida(String(v)) },
		{ key: 'momentum_rank', label: '#', numeric: true },
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'momentum_score_pct', label: 'Score', numeric: true, format: (v) => pctPts(v) }
	];
	const INPUT = 'rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm';
</script>

<Section title="Filtros">
	<form
		method="GET"
		class="flex flex-wrap items-end gap-4 rounded border border-brand-border bg-brand-card p-4"
	>
		<label class="text-sm">
			<span class="mb-1 block font-bold text-brand-label">Análisis</span>
			<select name="analisis" class={INPUT} value={f.analisis}>
				{#each analisisDisponibles as a (a.analysis_key)}
					<option value={a.analysis_key.replace(/^_/, '')}>{a.label}</option>
				{:else}
					<option value="">Asia + EEUU</option>
				{/each}
			</select>
		</label>
		<label class="text-sm">
			<span class="mb-1 block font-bold text-brand-label">Desde</span>
			<input type="date" name="desde" value={f.desde ?? ''} class={INPUT} />
		</label>
		<label class="text-sm">
			<span class="mb-1 block font-bold text-brand-label">Hasta</span>
			<input type="date" name="hasta" value={f.hasta ?? ''} class={INPUT} />
		</label>
		<button
			type="submit"
			class="rounded border border-brand-strong bg-brand-strong px-3 py-1.5 text-sm font-bold text-white hover:opacity-90"
			>Aplicar</button
		>
		<a
			href={resolve('/tendencia/historial')}
			class="rounded border border-brand-border bg-brand-bg px-3 py-1.5 text-sm font-bold text-brand-label hover:bg-brand-card"
			>Limpiar</a
		>
	</form>
</Section>

{#if !data.runs.ok}
	<ErrorState message={data.runs.message} code={data.runs.code} />
{:else if runs.length === 0}
	<EmptyState>
		No hay corridas guardadas todavía para este análisis o rango. Cada corrida del pipeline se
		guarda sola; las anteriores se cargan con
		<code>python -m fog_radar.jobs tendencia --backfill</code>.
	</EmptyState>
{:else}
	<Section title="Resumen del período">
		<KpiGrid>
			<KpiCard label="Corridas en el período" value={String(runs.length)} />
			<KpiCard
				label="Rango"
				value="{fechaCorta(primera?.run_date)} → {fechaCorta(ultima?.run_date)}"
			/>
			<KpiCard
				label="Sharpe portafolio (última corrida)"
				value={fmtNumber(ultima?.sharpe_portafolio, 2)}
				tone="accent"
			/>
			<KpiCard
				label="Retorno anual esperado (última corrida)"
				value={pct(ultima?.retorno_portafolio_anual)}
				tone="green"
			/>
		</KpiGrid>
	</Section>

	<Section title="Sharpe Ratio en el tiempo">
		<LineChart
			data={serieSharpe}
			x="corrida"
			categorical
			series={[
				{ key: 'portafolio', label: 'Sharpe portafolio' },
				{ key: 'universo', label: 'Sharpe universo completo' }
			]}
			yFormat={(v) => fmtNumber(v, 2)}
		/>
	</Section>

	<Section title="Retorno y volatilidad del portafolio en el tiempo">
		<LineChart
			data={serieRetVol}
			x="corrida"
			categorical
			series={[
				{ key: 'retorno', label: 'Retorno esperado anual' },
				{ key: 'volatilidad', label: 'Volatilidad anual' }
			]}
			yFormat={(v) => `${fmtNumber(v, 0)} %`}
		/>
	</Section>

	<Section title="Evolución de la asignación por sector">
		<p class="text-sm text-brand-muted">
			Peso total del portafolio en cada sector, corrida a corrida. Más de 7 sectores se agrupan en
			"Otros" para mantener el gráfico legible.
		</p>
		<LineChart
			data={sectores.filas}
			x="corrida"
			categorical
			series={sectores.grupos.map((g) => ({ key: g, label: g }))}
			height={280}
			yFormat={(v) => `${fmtNumber(v, 0)} %`}
		/>
	</Section>

	<Section title="Trayectoria de un ticker">
		{#if data.tickers.length === 0}
			<EmptyState>
				Ningún ticker apareció seleccionado en el portafolio final de este análisis todavía.
			</EmptyState>
		{:else}
			<form method="GET" class="flex flex-wrap items-end gap-3 text-sm">
				<input type="hidden" name="analisis" value={f.analisis} />
				{#if f.desde}<input type="hidden" name="desde" value={f.desde} />{/if}
				{#if f.hasta}<input type="hidden" name="hasta" value={f.hasta} />{/if}
				<label>
					<span class="mb-1 block font-bold text-brand-label">Ticker</span>
					<select name="ticker" class={INPUT} value={f.ticker ?? ''}>
						{#each data.tickers as t (t.ticker)}
							<option value={t.ticker}>{t.ticker} — {t.nombre ?? ''}</option>
						{/each}
					</select>
				</label>
				<button
					type="submit"
					class="rounded border border-brand-border bg-brand-bg px-3 py-1.5 font-bold text-brand-label hover:bg-brand-card"
					>Ver</button
				>
				{#if f.ticker}<span class="py-1.5"
						>Ver <TickerLink ticker={f.ticker} /> en Yahoo Finance ↗</span
					>{/if}
			</form>
			{#if data.trayectoria && !data.trayectoria.ok}
				<ErrorState message={data.trayectoria.message} code={data.trayectoria.code} retry={false} />
			{:else}
				<div class="grid gap-4 md:grid-cols-2">
					<div>
						<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
							Peso en el portafolio
						</div>
						<LineChart
							data={seriePeso}
							x="corrida"
							categorical
							series={[{ key: 'peso', label: 'Peso en portafolio' }]}
							yFormat={(v) => `${fmtNumber(v, 1)} %`}
						/>
					</div>
					<div>
						<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
							Precio actual vs. proyectado a 6 meses
						</div>
						<LineChart
							data={seriePrecio}
							x="corrida"
							categorical
							series={[
								{ key: 'actual', label: 'Precio actual (USD)' },
								{ key: 'proyectado', label: 'Proyectado a 6m (USD)' }
							]}
							yFormat={(v) => money(v, 0)}
						/>
					</div>
				</div>
			{/if}
		{/if}
	</Section>

	{#if data.leaders.length > 0}
		<Section title="Líderes de momentum por corrida (top 3)">
			<DataTable
				rows={data.leaders}
				columns={colLeaders}
				initialSort={{ key: 'run_dir', dir: 'desc' }}
				caption="Líderes de momentum"
				csvFilename="tendencia_lideres_momentum"
				pageSize={30}
			>
				{#snippet cell({ column, row, text })}
					{#if column.key === 'ticker' && row.ticker}<TickerLink
							ticker={row.ticker}
						/>{:else}{text}{/if}
				{/snippet}
			</DataTable>
		</Section>
	{/if}

	<Section title="Todas las corridas">
		<DataTable
			rows={runs}
			columns={colRuns}
			initialSort={{ key: 'run_dir', dir: 'desc' }}
			caption="Todas las corridas"
			csvFilename="tendencia_corridas"
			pageSize={50}
		/>
	</Section>
{/if}
