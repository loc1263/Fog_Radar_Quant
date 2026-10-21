<!--
  Ranking de momentum (mensual o semanal): #, ticker, empresa, un retorno por
  ventana y el score (MomentumTable.vue del original).
-->
<script lang="ts" module>
	export type MomentumFila = {
		rank: number;
		ticker: string;
		nombre?: string | null;
		momentum_score_pct?: number | null;
	} & Record<string, unknown>;
</script>

<script lang="ts" generics="T extends MomentumFila">
	import { DataTable, TickerLink } from '$lib/components/shared';
	import type { Column } from '$lib/table';
	import { pctPts } from './format';

	interface Props {
		rows: T[];
		/** Ventanas en el orden a mostrar (p. ej. [1, 2, 4, 6, 8, 10, 12]). */
		windows: number[];
		/** Sufijo de las columnas del JSON: "m" → momentum_1m_pct, "s" → momentum_1s_pct. */
		unit: 'm' | 's';
		csvFilename: string;
		caption: string;
	}

	let { rows, windows, unit, csvFilename, caption }: Props = $props();

	const positivos = $derived(unit === 'm' ? 'pct_meses_positivos_12m' : 'pct_semanas_positivas_4s');
	const columns = $derived<Column<T>[]>([
		{ key: 'rank', label: '#', numeric: true },
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Empresa', wrap: true },
		{ key: 'pais', label: 'País' },
		{ key: 'sector', label: 'Sector', wrap: true },
		...windows.map((w): Column<T> => ({
			key: `momentum_${w}${unit}_pct`,
			label: `${w}${unit}`,
			numeric: true,
			signed: true,
			format: (v) => pctPts(v)
		})),
		{
			key: positivos,
			label: unit === 'm' ? '% meses +' : '% semanas +',
			numeric: true,
			format: (v) => pctPts(v, 0)
		},
		{
			key: 'momentum_score_pct',
			label: 'Score',
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
