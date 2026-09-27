<!--
  Tabla común del sitio (Dashboard ejecutivo §5.5): orden por clic en todos los
  encabezados, búsqueda opcional y exportación a CSV de la vista filtrada.
-->
<script lang="ts" generics="T extends Row">
	import type { Snippet } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';
	import { signedClass } from '$lib/format';
	import {
		filterRows,
		nextSort,
		sortRows,
		toCsv,
		type Column,
		type Row,
		type SortState
	} from '$lib/table';

	interface Props {
		rows: T[];
		columns: Column<T>[];
		/**
		 * Render propio de las celdas. Recibe `text` (el valor ya formateado)
		 * para que las columnas sin tratamiento especial muestren `{text}`.
		 */
		cell?: Snippet<[{ column: Column<T>; row: T; value: unknown; text: string }]>;
		initialSort?: SortState;
		/** Columnas donde busca el filtro de texto (sin filtro si se omite). */
		searchKeys?: string[];
		searchPlaceholder?: string;
		/** Nombre del CSV (sin extensión). Sin valor, no se muestra el botón. */
		csvFilename?: string;
		emptyText?: string;
		caption?: string;
		class?: string;
	}

	let {
		rows,
		columns,
		cell,
		initialSort = null,
		searchKeys,
		searchPlaceholder = 'Buscar…',
		csvFilename,
		emptyText = 'Sin datos para mostrar.',
		caption,
		class: className
	}: Props = $props();

	let sort = $state<SortState>(null);
	let query = $state('');
	let sortTouched = $state(false);

	const effectiveSort = $derived(sortTouched ? sort : initialSort);
	const filtered = $derived(searchKeys ? filterRows(rows, query, searchKeys) : rows);
	const visible = $derived(sortRows(filtered, effectiveSort));

	function toggle(column: Column<T>) {
		if (column.sortable === false) return;
		sort = nextSort(effectiveSort, column.key, !!column.numeric);
		sortTouched = true;
	}

	function indicator(column: Column<T>): string {
		if (effectiveSort?.key !== column.key) return '↕';
		return effectiveSort.dir === 'asc' ? '↑' : '↓';
	}

	function ariaSort(column: Column<T>): 'ascending' | 'descending' | 'none' {
		if (effectiveSort?.key !== column.key) return 'none';
		return effectiveSort.dir === 'asc' ? 'ascending' : 'descending';
	}

	function display(column: Column<T>, row: T): string {
		const value = row[column.key];
		if (column.format) return column.format(value, row);
		return value === null || value === undefined || value === '' ? '—' : String(value);
	}

	function downloadCsv() {
		const blob = new Blob([toCsv(visible, columns)], { type: 'text/csv;charset=utf-8' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${csvFilename}.csv`;
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<div class={cn('space-y-2', className)}>
	{#if searchKeys || csvFilename}
		<div class="flex flex-wrap items-end justify-between gap-2">
			{#if searchKeys}
				<label class="block">
					<span class="mb-1 block text-sm font-bold text-brand-label">Buscar</span>
					<input
						type="search"
						bind:value={query}
						placeholder={searchPlaceholder}
						class="w-64 rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm"
					/>
				</label>
			{:else}
				<span></span>
			{/if}
			<div class="flex items-center gap-3 text-sm text-brand-muted">
				<span class="tabular-nums">{visible.length} de {rows.length}</span>
				{#if csvFilename}
					<Button size="sm" onclick={downloadCsv} disabled={visible.length === 0}>
						Descargar CSV
					</Button>
				{/if}
			</div>
		</div>
	{/if}

	<div class="overflow-x-auto rounded border border-brand-border">
		<table class="w-full border-collapse text-sm">
			{#if caption}<caption class="sr-only">{caption}</caption>{/if}
			<thead>
				<tr>
					{#each columns as column (column.key)}
						<th
							scope="col"
							aria-sort={ariaSort(column)}
							title={column.title}
							class={cn(
								'bg-brand-strong px-2 py-2 font-bold text-white xl:px-3 xl:whitespace-nowrap',
								column.numeric ? 'text-right' : 'text-left'
							)}
						>
							{#if column.sortable === false}
								{column.label}
							{:else}
								<button
									type="button"
									onclick={() => toggle(column)}
									class={cn(
										'inline-flex items-center gap-1 font-bold',
										column.numeric && 'flex-row-reverse'
									)}
								>
									<span>{column.label}</span>
									<span class="text-[11px] opacity-70" aria-hidden="true">{indicator(column)}</span>
								</button>
							{/if}
						</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each visible as row, i (i)}
					<tr class="border-b border-brand-border odd:bg-brand-bg even:bg-brand-card">
						{#each columns as column (column.key)}
							<td
								class={cn(
									'px-2 py-2 xl:px-3',
									column.numeric && 'text-right whitespace-nowrap tabular-nums',
									column.wrap && 'min-w-[8rem]',
									column.signed && signedClass(row[column.key])
								)}
							>
								{#if cell}
									{@render cell({
										column,
										row,
										value: row[column.key],
										text: display(column, row)
									})}
								{:else}
									{display(column, row)}
								{/if}
							</td>
						{/each}
					</tr>
				{:else}
					<tr>
						<td colspan={columns.length} class="bg-brand-card p-6 text-center text-brand-muted">
							{emptyText}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>
