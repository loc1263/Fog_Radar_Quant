<!--
  DataTable con las celdas comunes del módulo ETF: ticker con enlace, nombre +
  categoría, barra de rango de 52 semanas y score destacado. Las columnas
  propias de cada sección se renderizan con `extra` (o como texto).
-->
<script lang="ts" module>
	import type { Row } from '$lib/table';

	export type EtfRow = Row & {
		ticker?: string;
		nombre?: string | null;
		categoria?: string | null;
		precio?: number | null;
		max_52s?: number | null;
		min_52s?: number | null;
	};
</script>

<script lang="ts" generics="T extends EtfRow">
	import type { Snippet } from 'svelte';
	import { DataTable, TickerLink } from '$lib/components/shared';
	import type { Column, SortState } from '$lib/table';
	import NombreCategoria from './NombreCategoria.svelte';
	import Rango52 from './Rango52.svelte';

	interface Props {
		rows: T[];
		columns: Column<T>[];
		/** Columna del score principal (se destaca en ámbar). */
		scoreKey?: string | null;
		extra?: Snippet<[{ column: Column<T>; row: T; value: unknown; text: string }]>;
		csvFilename?: string;
		csvColumns?: Column<T>[];
		searchKeys?: string[];
		searchPlaceholder?: string;
		initialQuery?: string;
		initialSort?: SortState;
		pageSize?: number;
		emptyText?: string;
		caption?: string;
	}

	let { scoreKey = 'score', extra, ...rest }: Props = $props();
</script>

<DataTable {...rest}>
	{#snippet cell({ column, row, value, text })}
		{#if column.key === 'ticker' && row.ticker}
			<TickerLink ticker={row.ticker} etf />
		{:else if column.key === 'nombre'}
			<NombreCategoria nombre={row.nombre} categoria={row.categoria} />
		{:else if column.key === 'rango52'}
			<Rango52 precio={row.precio} max={row.max_52s} min={row.min_52s} />
		{:else if column.key === scoreKey}
			<span class="font-bold text-brand-accent">{text}</span>
		{:else if extra}
			{@render extra({ column, row, value, text })}
		{:else}
			{text}
		{/if}
	{/snippet}
</DataTable>
