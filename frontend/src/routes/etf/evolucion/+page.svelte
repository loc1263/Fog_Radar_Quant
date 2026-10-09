<script lang="ts">
	import { resolve } from '$app/paths';
	import type { EtfHistPermanencia, EtfHistTickerPunto } from '$lib/api/types';
	import { LineChart } from '$lib/charts';
	import {
		DataTable,
		EmptyState,
		ErrorState,
		HistDiffTable,
		KpiCard,
		KpiGrid,
		Notice,
		Section
	} from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { fmtRunKey } from '$lib/historial';
	import { fmtRend, top15PorCategoria } from '$lib/modules/etf/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const runs = $derived(data.runs.ok ? data.runs.data : []);
	const top = $derived(data.top.ok ? data.top.data : []);
	const serie = $derived(data.serie?.ok ? data.serie.data : []);
	const ultimo = $derived(serie.at(-1));
	const vecesEnTop = $derived(serie.filter((p) => p.en_top15).length);
	// El ticker pedido por URL puede no estar entre las opciones del selector.
	const opciones = $derived(
		data.ticker && !data.tickers.some((t) => t.ticker === data.ticker)
			? [{ ticker: data.ticker, nombre: ultimo?.nombre ?? null }, ...data.tickers]
			: data.tickers
	);

	const serieScore = $derived(
		serie.map((p) => ({
			corrida: fmtRunKey(p.run_key),
			score: p.score,
			simple: p.score_simple,
			momentum: p.momentum_score
		}))
	);
	const serieMetricas = $derived(
		serie.map((p) => ({ corrida: fmtRunKey(p.run_key), sharpe: p.sharpe, beta: p.beta }))
	);
	const categorias = $derived(top15PorCategoria(data.categorias, fmtRunKey));

	const colSerie: Column<EtfHistTickerPunto>[] = [
		{ key: 'run_key', label: 'Corrida', format: (v) => fmtRunKey(String(v)) },
		{
			key: 'rank',
			label: 'Puesto',
			numeric: true,
			format: (v, r) => (v == null ? '—' : `${v} / ${r.universo_depurado}`)
		},
		{ key: 'en_top15', label: 'Top 15', format: (v) => (v ? 'Sí' : 'No') },
		{ key: 'score', label: 'Score', numeric: true, format: (v) => fmtNumber(v, 1) },
		{
			key: 'momentum_rank',
			label: 'Puesto momentum',
			numeric: true,
			format: (v) => fmtNumber(v, 0)
		},
		{ key: 'sharpe', label: 'Sharpe', numeric: true, format: (v) => fmtNumber(v, 2) },
		{ key: 'perf_1y', label: 'Rend. 1A', numeric: true, signed: true, format: (v) => fmtRend(v) },
		{
			key: 'excluido_ventana',
			label: 'Excluido (outlier)',
			format: (v, r) => (v ? `${v} · z ${fmtNumber(r.excluido_z, 1)}` : '—')
		}
	];
	const colTop: Column<EtfHistPermanencia>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Nombre', wrap: true },
		{
			key: 'veces_en_top',
			label: 'Veces en el Top 15',
			numeric: true,
			format: (v, r) => `${v} / ${r.corridas}`
		},
		{ key: 'racha_actual', label: 'Racha actual', numeric: true },
		{ key: 'rank_ultima', label: 'Puesto hoy', numeric: true, format: (v) => fmtNumber(v, 0) },
		{ key: 'score_ultima', label: 'Score hoy', numeric: true, format: (v) => fmtNumber(v, 1) }
	];
	const INPUT = 'rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm';
</script>

{#if !data.runs.ok}
	<ErrorState message={data.runs.message} code={data.runs.code} />
{:else if runs.length === 0}
	<EmptyState>
		Todavía no hay corridas en el historial. Cada corrida se guarda sola; las archivadas se cargan
		con <code>fogradar job etf --backfill</code>.
	</EmptyState>
{:else}
	<Section title="Filtros">
		<form
			method="GET"
			class="flex flex-wrap items-end gap-4 rounded border border-brand-border bg-brand-card p-4"
		>
			<label class="text-sm">
				<span class="mb-1 block font-bold text-brand-label">ETF</span>
				<select name="ticker" class={INPUT} value={data.ticker ?? ''}>
					{#each opciones as t (t.ticker)}
						<option value={t.ticker}>{t.ticker} — {t.nombre ?? ''}</option>
					{/each}
				</select>
			</label>
			{#if data.par}
				<label class="text-sm">
					<span class="mb-1 block font-bold text-brand-label">Comparar desde</span>
					<select name="desde" class={INPUT} value={data.par.desde}>
						{#each runs as r (r.run_key)}<option value={r.run_key}>{fmtRunKey(r.run_key)}</option
							>{/each}
					</select>
				</label>
				<label class="text-sm">
					<span class="mb-1 block font-bold text-brand-label">hasta</span>
					<select name="hasta" class={INPUT} value={data.par.hasta}>
						{#each runs as r (r.run_key)}<option value={r.run_key}>{fmtRunKey(r.run_key)}</option
							>{/each}
					</select>
				</label>
			{/if}
			<button
				type="submit"
				class="rounded border border-brand-strong bg-brand-strong px-3 py-1.5 text-sm font-bold text-white hover:opacity-90"
				>Aplicar</button
			>
			<a
				href={resolve('/etf/evolucion')}
				class="rounded border border-brand-border bg-brand-bg px-3 py-1.5 text-sm font-bold text-brand-label hover:bg-brand-card"
				>Limpiar</a
			>
		</form>
	</Section>

	<Section title="Trayectoria de {data.ticker ?? 'un ETF'}">
		{#if data.serie && !data.serie.ok}
			<ErrorState message={data.serie.message} code={data.serie.code} retry={false} />
		{:else if serie.length === 0}
			<EmptyState>Ningún ETF pasó por el Top 15 en las corridas guardadas.</EmptyState>
		{:else}
			<KpiGrid>
				<KpiCard label="Score (última)" value={fmtNumber(ultimo?.score, 1)} tone="accent" />
				<KpiCard
					label="Puesto (última)"
					value={ultimo?.rank == null ? '—' : `${ultimo.rank} / ${ultimo.universo_depurado}`}
				/>
				<KpiCard label="Corridas en el Top 15" value="{vecesEnTop} / {serie.length}" />
				<KpiCard label="Categoría" value={ultimo?.category ?? '—'} />
			</KpiGrid>
			<div class="grid gap-4 md:grid-cols-2">
				<div>
					<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
						Scores (0-100)
					</div>
					<LineChart
						data={serieScore}
						x="corrida"
						categorical
						series={[
							{ key: 'score', label: 'Score principal' },
							{ key: 'simple', label: 'Score rend. + Beta' },
							{ key: 'momentum', label: 'Score momentum' }
						]}
						yFormat={(v) => fmtNumber(v, 0)}
					/>
				</div>
				<div>
					<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
						Sharpe y Beta
					</div>
					<LineChart
						data={serieMetricas}
						x="corrida"
						categorical
						series={[
							{ key: 'sharpe', label: 'Sharpe' },
							{ key: 'beta', label: 'Beta' }
						]}
						yFormat={(v) => fmtNumber(v, 2)}
					/>
				</div>
			</div>
			<DataTable
				rows={serie}
				columns={colSerie}
				initialSort={{ key: 'run_key', dir: 'desc' }}
				caption="Trayectoria del ETF"
				csvFilename="etf_trayectoria_{data.ticker}"
			/>
		{/if}
	</Section>

	<Section title="Rotación del Top 15 entre dos corridas">
		{#if !data.par}
			<EmptyState>Hacen falta al menos dos corridas para comparar.</EmptyState>
		{:else if data.diff && !data.diff.ok}
			<ErrorState message={data.diff.message} code={data.diff.code} retry={false} />
		{:else if data.diff?.ok}
			<Notice>
				<p>
					{fmtRunKey(data.diff.data.desde)} → {fmtRunKey(data.diff.data.hasta)}. A diferencia de la
					pestaña Rotación (vigente contra la anterior), acá se pueden comparar dos corridas
					cualesquiera del historial.
				</p>
			</Notice>
			<HistDiffTable diff={data.diff.data} scoreDigits={1} csvFilename="etf_rotacion_historica">
				{#snippet ticker({ row })}
					<a
						class="font-bold text-brand-accent hover:underline"
						href="{resolve('/etf/evolucion')}?ticker={encodeURIComponent(row.ticker)}"
						>{row.ticker}</a
					>
				{/snippet}
			</HistDiffTable>
		{/if}
	</Section>

	<Section
		title="Permanencia en el Top 15 (últimas {Math.min(data.ventana, runs.length)} corridas)"
	>
		{#if !data.top.ok}
			<ErrorState message={data.top.message} code={data.top.code} retry={false} />
		{:else}
			<DataTable
				rows={top}
				columns={colTop}
				caption="Permanencia en el Top 15"
				csvFilename="etf_permanencia_top15"
				searchKeys={['ticker', 'nombre']}
				pageSize={30}
			>
				{#snippet cell({ column, row, text })}
					{#if column.key === 'ticker'}
						<a
							class="font-bold text-brand-accent hover:underline"
							href="{resolve('/etf/evolucion')}?ticker={encodeURIComponent(row.ticker)}"
							>{row.ticker}</a
						>
					{:else}{text}{/if}
				{/snippet}
			</DataTable>
		{/if}
	</Section>

	{#if categorias.grupos.length > 0}
		<Section title="Categorías en el Top 15, corrida a corrida">
			<p class="text-sm text-brand-muted">
				Cuántos ETFs de cada categoría Morningstar entraron al Top 15 en cada corrida (las más
				frecuentes del período; el resto en "Otras").
			</p>
			<LineChart
				data={categorias.filas}
				x="corrida"
				categorical
				series={categorias.grupos.map((g) => ({ key: g, label: g }))}
				height={280}
				zeroBaseline
				yFormat={(v) => fmtNumber(v, 0)}
			/>
		</Section>
	{/if}
{/if}
