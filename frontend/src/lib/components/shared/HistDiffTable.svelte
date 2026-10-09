<!--
  Comparación de dos corridas (GET /api/<modulo>/history/diff): KPIs de entradas y
  salidas + tabla con el cambio de puesto y de score de cada ticker.
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HistDiff, HistDiffFila } from '$lib/api/types';
	import { fmtNumber } from '$lib/format';
	import { ESTADOS, resumenDiff } from '$lib/historial';
	import type { Column } from '$lib/table';
	import DataTable from './DataTable.svelte';
	import KpiCard from './KpiCard.svelte';
	import KpiGrid from './KpiGrid.svelte';
	import SignalBadge from './SignalBadge.svelte';
	import TickerLink from './TickerLink.svelte';

	interface Props {
		diff: HistDiff;
		/** Decimales del score (z-score: 3; puntaje 0-100: 1). */
		scoreDigits?: number;
		csvFilename: string;
		/** Mostrar los tickers con estado "fuera" (fuera del top en ambas corridas). */
		mostrarFuera?: boolean;
		/** Render propio del ticker (p. ej. enlace a su evolución). */
		ticker?: Snippet<[{ row: HistDiffFila }]>;
	}

	let { diff, scoreDigits = 2, csvFilename, mostrarFuera = false, ticker }: Props = $props();

	const filas = $derived(
		mostrarFuera ? diff.filas : diff.filas.filter((f) => f.estado !== 'fuera')
	);
	const r = $derived(resumenDiff(diff.filas));
	const n = (v: unknown) => fmtNumber(v, scoreDigits);
	const delta = (v: unknown, d: number) =>
		typeof v === 'number' ? `${v > 0 ? '+' : ''}${fmtNumber(v, d)}` : '—';

	const columns: Column<HistDiffFila>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Nombre', wrap: true },
		{ key: 'estado', label: 'Estado', format: (v) => ESTADOS[v as HistDiffFila['estado']].label },
		{ key: 'rank_desde', label: 'Puesto antes', numeric: true, format: (v) => fmtNumber(v, 0) },
		{ key: 'rank_hasta', label: 'Puesto ahora', numeric: true, format: (v) => fmtNumber(v, 0) },
		{
			key: 'delta_rank',
			label: 'Δ puesto',
			numeric: true,
			signed: true,
			format: (v) => delta(v, 0),
			title: 'Positivo = subió posiciones'
		},
		{ key: 'score_desde', label: 'Score antes', numeric: true, format: n },
		{ key: 'score_hasta', label: 'Score ahora', numeric: true, format: n },
		{
			key: 'delta_score',
			label: 'Δ score',
			numeric: true,
			signed: true,
			format: (v) => delta(v, scoreDigits)
		}
	];
</script>

<KpiGrid>
	{#if diff.top_n}
		<KpiCard label="Entraron al top {diff.top_n}" value={String(r.nuevos)} tone="green" />
		<KpiCard label="Salieron del top {diff.top_n}" value={String(r.salidas)} tone="red" />
		<KpiCard label="Se mantienen" value={String(r.siguen)} />
	{:else}
		<KpiCard label="Suben de puesto" value={String(r.suben)} tone="green" />
		<KpiCard label="Bajan de puesto" value={String(r.bajan)} tone="red" />
		<KpiCard label="Sin cambio" value={String(r.siguen - r.suben - r.bajan)} />
	{/if}
</KpiGrid>

<DataTable
	rows={filas}
	{columns}
	{csvFilename}
	caption="Comparación de corridas"
	searchKeys={['ticker', 'nombre']}
	searchPlaceholder="Buscar ticker o nombre"
	pageSize={40}
>
	{#snippet cell({ column, row, text })}
		{#if column.key === 'ticker'}
			{#if ticker}{@render ticker({ row })}{:else}<TickerLink ticker={row.ticker} />{/if}
		{:else if column.key === 'estado'}
			<SignalBadge signal={ESTADOS[row.estado].signal} label={text} />
		{:else}
			{text}
		{/if}
	{/snippet}
</DataTable>
