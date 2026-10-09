<script lang="ts">
	import { resolve } from '$app/paths';
	import type { UsHistCorrida, UsHistPermanencia, UsHistTickerPunto } from '$lib/api/types';
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
	import { fmtRunId } from '$lib/historial';
	import {
		CATEGORY_KEYS,
		CATEGORY_LABELS,
		fmtPrice,
		fmtRet,
		fmtScore
	} from '$lib/modules/us/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const runs = $derived(data.runs.ok ? data.runs.data : []);
	const top = $derived(data.top.ok ? data.top.data : []);
	const serie = $derived(data.serie?.ok ? data.serie.data : []);
	const ultimo = $derived(serie.at(-1));
	const vecesEnTop = $derived(serie.filter((p) => p.in_top).length);

	const fecha = (runId: string) => fmtRunId(runId).slice(0, 10);
	/** Etiqueta del eje X: fecha y hora (puede haber más de una corrida por día). */
	const corrida = (runId: string) => fmtRunId(runId).slice(0, 16);
	const serieScore = $derived(
		serie.map((p) => ({
			corrida: corrida(p.run_id),
			score: p.score,
			...Object.fromEntries(CATEGORY_KEYS.map((k) => [k, p.pilares[k]]))
		}))
	);
	const serieMomentum = $derived(
		serie.map((p) => ({ corrida: corrida(p.run_id), momentum: (p.momentum_12_1 ?? 0) * 100 }))
	);

	const colTop: Column<UsHistPermanencia>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Nombre', wrap: true },
		{
			key: 'veces_en_top',
			label: 'Veces en el top',
			numeric: true,
			format: (v, r) => `${v} / ${r.corridas}`
		},
		{ key: 'racha_actual', label: 'Racha actual', numeric: true },
		{ key: 'rank_ultima', label: 'Puesto hoy', numeric: true, format: (v) => fmtNumber(v, 0) },
		{ key: 'score_ultima', label: 'Score hoy', numeric: true, format: fmtScore }
	];
	const colSerie: Column<UsHistTickerPunto>[] = [
		{ key: 'run_id', label: 'Corrida', format: (v) => fmtRunId(String(v)) },
		{
			key: 'rank',
			label: 'Puesto (momentum)',
			numeric: true,
			format: (v, r) => `${v} / ${r.evaluated_total ?? '—'}`
		},
		{ key: 'in_top', label: 'En el top', format: (v) => (v ? 'Sí' : 'No') },
		{ key: 'price', label: 'Precio', numeric: true, format: fmtPrice },
		{ key: 'momentum_12_1', label: 'Momentum 12-1', numeric: true, signed: true, format: fmtRet },
		{ key: 'score', label: 'Score', numeric: true, format: fmtScore },
		{ key: 'score_label', label: 'Lectura' }
	];
	const colRuns: Column<UsHistCorrida>[] = [
		{ key: 'run_id', label: 'Corrida', format: (v) => fmtRunId(String(v)) },
		{ key: 'universe_total', label: 'Universo', numeric: true },
		{ key: 'candidates_total', label: 'Candidatos', numeric: true },
		{ key: 'evaluated_total', label: 'Evaluados', numeric: true },
		{ key: 'top1_ticker', label: 'N.º 1 (momentum)' },
		{ key: 'top1_score', label: 'Score N.º 1', numeric: true, format: fmtScore }
	];
	const INPUT = 'rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm';
</script>

{#if !data.runs.ok}
	<ErrorState message={data.runs.message} code={data.runs.code} />
{:else if runs.length === 0}
	<EmptyState>
		Todavía no hay corridas en el historial. Cada corrida del screener se guarda sola; las
		anteriores se cargan con <code>fogradar job us_stocks --backfill</code>.
	</EmptyState>
{:else}
	<Section title="Resumen">
		<KpiGrid>
			<KpiCard label="Corridas guardadas" value={String(runs.length)} />
			<KpiCard
				label="Rango"
				value="{fecha(runs[0].run_id)} → {fecha(runs[runs.length - 1].run_id)}"
			/>
			<KpiCard
				label="Tickers que pasaron por el top (últimas {Math.min(data.ventana, runs.length)})"
				value={String(top.length)}
				tone="accent"
			/>
		</KpiGrid>
	</Section>

	<Section title="Filtros">
		<form
			method="GET"
			class="flex flex-wrap items-end gap-4 rounded border border-brand-border bg-brand-card p-4"
		>
			<label class="text-sm">
				<span class="mb-1 block font-bold text-brand-label">Ticker</span>
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
						{#each runs as r (r.run_id)}<option value={r.run_id}>{fmtRunId(r.run_id)}</option
							>{/each}
					</select>
				</label>
				<label class="text-sm">
					<span class="mb-1 block font-bold text-brand-label">hasta</span>
					<select name="hasta" class={INPUT} value={data.par.hasta}>
						{#each runs as r (r.run_id)}<option value={r.run_id}>{fmtRunId(r.run_id)}</option
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
				href={resolve('/eeuu/historial')}
				class="rounded border border-brand-border bg-brand-bg px-3 py-1.5 text-sm font-bold text-brand-label hover:bg-brand-card"
				>Limpiar</a
			>
		</form>
	</Section>

	<Section title="Evolución de {data.ticker ?? 'un ticker'}">
		{#if data.serie && !data.serie.ok}
			<ErrorState message={data.serie.message} code={data.serie.code} retry={false} />
		{:else if serie.length === 0}
			<EmptyState>Ningún ticker pasó por el top en las corridas guardadas.</EmptyState>
		{:else}
			<KpiGrid>
				<KpiCard label="Score de salud (última)" value={fmtScore(ultimo?.score)} tone="accent" />
				<KpiCard
					label="Puesto por momentum (última)"
					value="{ultimo?.rank ?? '—'} / {ultimo?.evaluated_total ?? '—'}"
				/>
				<KpiCard label="Corridas en el top" value="{vecesEnTop} / {serie.length}" />
				<KpiCard label="Precio (última)" value={fmtPrice(ultimo?.price)} />
			</KpiGrid>
			<div class="grid gap-4 md:grid-cols-2">
				<div>
					<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
						Score de salud y pilares (0-100)
					</div>
					<LineChart
						data={serieScore}
						x="corrida"
						categorical
						series={[
							{ key: 'score', label: 'Score' },
							...CATEGORY_KEYS.map((k) => ({ key: k, label: CATEGORY_LABELS[k] }))
						]}
						yFormat={(v) => fmtNumber(v, 0)}
					/>
				</div>
				<div>
					<div class="mb-1 text-xs font-bold tracking-wide text-brand-label uppercase">
						Momentum 12-1
					</div>
					<LineChart
						data={serieMomentum}
						x="corrida"
						categorical
						series={[{ key: 'momentum', label: 'Momentum 12-1' }]}
						yFormat={(v) => `${fmtNumber(v, 0)} %`}
					/>
				</div>
			</div>
			<DataTable
				rows={serie}
				columns={colSerie}
				initialSort={{ key: 'run_id', dir: 'desc' }}
				caption="Trayectoria del ticker"
				csvFilename="us_historial_{data.ticker}"
			/>
		{/if}
	</Section>

	<Section title="Qué entró y salió del top">
		{#if !data.par}
			<EmptyState>Hacen falta al menos dos corridas para comparar.</EmptyState>
		{:else if data.diff && !data.diff.ok}
			<ErrorState message={data.diff.message} code={data.diff.code} retry={false} />
		{:else if data.diff?.ok}
			<Notice>
				<p>
					{fmtRunId(data.diff.data.desde)} → {fmtRunId(data.diff.data.hasta)}. El top son los
					{data.diff.data.top_n} candidatos con mayor momentum 12-1; el score es el de salud (0-100).
				</p>
			</Notice>
			<HistDiffTable diff={data.diff.data} scoreDigits={1} csvFilename="us_rotacion_top">
				{#snippet ticker({ row })}
					<a
						class="font-bold text-brand-accent hover:underline"
						href="{resolve('/eeuu/historial')}?ticker={encodeURIComponent(row.ticker)}"
						>{row.ticker}</a
					>
				{/snippet}
			</HistDiffTable>
		{/if}
	</Section>

	<Section title="Permanencia en el top (últimas {Math.min(data.ventana, runs.length)} corridas)">
		{#if !data.top.ok}
			<ErrorState message={data.top.message} code={data.top.code} retry={false} />
		{:else}
			<DataTable
				rows={top}
				columns={colTop}
				caption="Permanencia en el top"
				csvFilename="us_permanencia_top"
				searchKeys={['ticker', 'nombre']}
				pageSize={30}
			>
				{#snippet cell({ column, row, text })}
					{#if column.key === 'ticker'}
						<a
							class="font-bold text-brand-accent hover:underline"
							href="{resolve('/eeuu/historial')}?ticker={encodeURIComponent(row.ticker)}"
							>{row.ticker}</a
						>
					{:else}{text}{/if}
				{/snippet}
			</DataTable>
		{/if}
	</Section>

	<Section title="Corridas guardadas">
		<DataTable
			rows={runs}
			columns={colRuns}
			initialSort={{ key: 'run_id', dir: 'desc' }}
			caption="Corridas"
			csvFilename="us_corridas"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'top1_ticker' && row.top1_ticker}<TickerLink
						ticker={row.top1_ticker}
					/>{:else}{text}{/if}
			{/snippet}
		</DataTable>
	</Section>
{/if}
