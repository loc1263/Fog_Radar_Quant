<!--
  Ranking por retorno puro de las últimas semanas (sin score ni consistencia): #, ticker, empresa,
  país, sector, precio y retorno de la ventana.
-->
<script lang="ts">
	import { DataTable, TickerLink } from '$lib/components/shared';
	import type { TdRetornoReciente } from '$lib/api/types';
	import type { Column } from '$lib/table';
	import { money, pctPts } from './format';

	interface Props {
		rows: TdRetornoReciente[];
		/** Velas semanales de la ventana (viene de `parametros.retorno_semanas`). */
		semanas?: number;
		csvFilename: string;
		caption: string;
	}

	let { rows, semanas, csvFilename, caption }: Props = $props();

	const columns = $derived<Column<TdRetornoReciente>[]>([
		{ key: 'rank', label: '#', numeric: true },
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Empresa', wrap: true },
		{ key: 'pais', label: 'País' },
		{ key: 'sector', label: 'Sector', wrap: true },
		{ key: 'precio_actual_usd', label: 'Precio', numeric: true, format: (v) => money(v) },
		{
			key: 'retorno_pct',
			label: semanas ? `Retorno ${semanas}s` : 'Retorno',
			numeric: true,
			signed: true,
			format: (v) => pctPts(v)
		}
	]);
</script>

<DataTable {rows} {columns} {csvFilename} {caption} initialSort={{ key: 'rank', dir: 'asc' }}>
	{#snippet cell({ column, row, text })}
		{#if column.key === 'ticker'}
			<TickerLink ticker={row.ticker} />
		{:else}
			{text}
		{/if}
	{/snippet}
</DataTable>
