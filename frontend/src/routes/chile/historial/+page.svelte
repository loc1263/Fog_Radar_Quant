<script lang="ts">
	import { resolve } from '$app/paths';
	import type { ClHistCorrida, ClHistTickerPunto } from '$lib/api/types';
	import { LineChart } from '$lib/charts';
	import {
		DataTable,
		EmptyState,
		ErrorState,
		HistDiffTable,
		KpiCard,
		KpiGrid,
		Notice,
		Section,
		TickerLink
	} from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { fmtTs } from '$lib/historial';
	import { fmtZ } from '$lib/modules/cl/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const runs = $derived(data.runs.ok ? data.runs.data : []);
	const serie = $derived(data.serie?.ok ? data.serie.data : []);
	const ultimo = $derived(serie.at(-1));
	const previo = $derived(serie.at(-2));
	/** Puestos ganados vs. la corrida anterior (positivo = subió). */
	const deltaRank = $derived(
		ultimo?.rank != null && previo?.rank != null ? previo.rank - ultimo.rank : null
	);

	const PILARES = [
		['valor', 'Valor'],
		['calidad', 'Calidad'],
		['crecimiento', 'Crecimiento'],
		['salud_financiera', 'Salud financiera'],
		['dividendo', 'Dividendo'],
		['momentum', 'Momentum']
	] as const;

	const serieScore = $derived(
		serie.map((p) => ({ corrida: fmtTs(p.ts), score: p.score_compuesto }))
	);
	const seriePilares = $derived(
		serie.map((p) => ({
			corrida: fmtTs(p.ts),
			...Object.fromEntries(PILARES.map(([k]) => [k, p.pilares[k]]))
		}))
	);

	const colSerie: Column<ClHistTickerPunto>[] = [
		{ key: 'ts', label: 'Corrida', format: (v) => fmtTs(String(v)) },
		{
			key: 'rank',
			label: 'Puesto',
			numeric: true,
			format: (v, r) => `${fmtNumber(v, 0)} / ${r.n_tickers}`
		},
		{
			key: 'score_compuesto',
			label: 'Score compuesto',
			numeric: true,
			format: (v) => fmtZ(v as number)
		},
		{
			key: 'metricas_disponibles',
			label: 'Métricas',
			numeric: true,
			format: (v) => fmtNumber(v, 0)
		}
	];
	const colRuns: Column<ClHistCorrida>[] = [
		{ key: 'ts', label: 'Corrida', format: (v) => fmtTs(String(v)) },
		{ key: 'n_tickers', label: 'Acciones', numeric: true },
		{ key: 'top1_ticker', label: 'N.º 1' },
		{ key: 'top1_score', label: 'Score N.º 1', numeric: true, format: (v) => fmtZ(v as number) },
		{ key: 'archivo', label: 'Archivo' }
	];
	const INPUT = 'rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm';
</script>

{#if !data.runs.ok}
	<ErrorState message={data.runs.message} code={data.runs.code} />
{:else if runs.length === 0}
	<EmptyState>
		Todavía no hay corridas en el historial. Cada corrida del pipeline se guarda sola; los CSV
		anteriores se cargan con <code>python -m fog_radar.jobs cl_stocks --backfill</code>.
	</EmptyState>
{:else}
	<Section title="Filtros">
		<form
			method="GET"
			class="flex flex-wrap items-end gap-4 rounded border border-brand-border bg-brand-card p-4"
		>
			<label class="text-sm">
				<span class="mb-1 block font-bold text-brand-label">Acción</span>
				<select name="ticker" class={INPUT} value={data.ticker ?? ''}>
					{#each data.tickers as t (t.ticker)}
						<option value={t.ticker}>{t.ticker} — {t.nombre ?? ''}</option>
					{/each}
				</select>
			</label>
			{#if data.par}
				<label class="text-sm">
					<span class="mb-1 block font-bold text-brand-label">Comparar desde</span>
					<select name="desde" class={INPUT} value={data.par.desde}>
						{#each runs as r (r.ts)}<option value={r.ts}>{fmtTs(r.ts)}</option>{/each}
					</select>
				</label>
				<label class="text-sm">
					<span class="mb-1 block font-bold text-brand-label">hasta</span>
					<select name="hasta" class={INPUT} value={data.par.hasta}>
						{#each runs as r (r.ts)}<option value={r.ts}>{fmtTs(r.ts)}</option>{/each}
					</select>
				</label>
			{/if}
			<button
				type="submit"
				class="rounded border border-brand-strong bg-brand-strong px-3 py-1.5 text-sm font-bold text-white hover:opacity-90"
				>Aplicar</button
			>
			<a
				href={resolve('/chile/historial')}
				class="rounded border border-brand-border bg-brand-bg px-3 py-1.5 text-sm font-bold text-brand-label hover:bg-brand-card"
				>Limpiar</a
			>
		</form>
	</Section>

	<Section title="Evolución de {data.ticker ?? 'la acción'}">
		{#if data.serie && !data.serie.ok}
			<ErrorState message={data.serie.message} code={data.serie.code} retry={false} />
		{:else if serie.length === 0}
			<EmptyState>Esta acción no aparece en el historial.</EmptyState>
		{:else}
			<KpiGrid>
				<KpiCard
					label="Puesto en la última corrida"
					value="{fmtNumber(ultimo?.rank, 0)} / {ultimo?.n_tickers ?? '—'}"
					tone="accent"
				/>
				<KpiCard
					label="Cambio de puesto vs. corrida anterior"
					value={deltaRank === null ? '—' : `${deltaRank > 0 ? '+' : ''}${deltaRank}`}
					signedValue={deltaRank}
				/>
				<KpiCard label="Score compuesto" value={fmtZ(ultimo?.score_compuesto)} />
				<KpiCard label="Corridas con datos" value={String(serie.length)} />
			</KpiGrid>
			<div class="grid gap-4 md:grid-cols-2">
				<div>
					<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
						Score compuesto (z-score)
					</div>
					<LineChart
						data={serieScore}
						x="corrida"
						categorical
						series={[{ key: 'score', label: 'Score compuesto' }]}
						yFormat={(v) => fmtNumber(v, 2)}
					/>
				</div>
				<div>
					<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
						Score por pilar
					</div>
					<LineChart
						data={seriePilares}
						x="corrida"
						categorical
						series={PILARES.map(([key, label]) => ({ key, label }))}
						yFormat={(v) => fmtNumber(v, 2)}
					/>
				</div>
			</div>
			<DataTable
				rows={serie}
				columns={colSerie}
				initialSort={{ key: 'ts', dir: 'desc' }}
				caption="Trayectoria de la acción"
				csvFilename="cl_historial_{data.ticker}"
			/>
		{/if}
	</Section>

	<Section title="Cambios de posición entre corridas">
		{#if !data.par}
			<EmptyState>Hacen falta al menos dos corridas para comparar.</EmptyState>
		{:else if data.diff && !data.diff.ok}
			<ErrorState message={data.diff.message} code={data.diff.code} retry={false} />
		{:else if data.diff?.ok}
			<Notice>
				<p>
					{fmtTs(data.diff.data.desde)} → {fmtTs(data.diff.data.hasta)}. El IPSA rankea las 30
					acciones en cada corrida: lo que cambia es el puesto. Δ puesto positivo = subió.
				</p>
			</Notice>
			<HistDiffTable diff={data.diff.data} scoreDigits={3} csvFilename="cl_cambios_ranking">
				{#snippet ticker({ row })}
					<a
						class="font-bold text-brand-accent hover:underline"
						href="{resolve('/chile/historial')}?ticker={encodeURIComponent(row.ticker)}"
						>{row.ticker}</a
					>
				{/snippet}
			</HistDiffTable>
		{/if}
	</Section>

	<Section title="Corridas guardadas">
		<DataTable
			rows={runs}
			columns={colRuns}
			initialSort={{ key: 'ts', dir: 'desc' }}
			caption="Corridas"
			csvFilename="cl_corridas"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'top1_ticker' && row.top1_ticker}<TickerLink
						ticker={row.top1_ticker}
					/>{:else}{text}{/if}
			{/snippet}
		</DataTable>
	</Section>
{/if}
