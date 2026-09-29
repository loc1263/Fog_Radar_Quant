<!--
  Tabla común del sitio (Dashboard ejecutivo §5.5): orden por clic en todos los
  encabezados, búsqueda opcional y exportación a CSV de la vista filtrada.
-->
<script lang="ts" generics="T extends Row">
	import { untrack, type Snippet } from 'svelte';
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
		/**
		 * Detalle desplegable bajo cada fila. Agrega una columna inicial con un
		 * botón ▸/▾ (una fila abierta a la vez). Requiere `rowKey`.
		 */
		detail?: Snippet<[{ row: T }]>;
		/** Identificador estable de la fila (para el detalle desplegable). */
		rowKey?: (row: T) => string;
		/** Texto accesible del botón de detalle. */
		detailLabel?: (row: T) => string;
		initialSort?: SortState;
		/** Columnas donde busca el filtro de texto (sin filtro si se omite). */
		searchKeys?: string[];
		searchPlaceholder?: string;
		/** Texto inicial del filtro (p. ej. `?q=` de la URL, desde el buscador global). */
		initialQuery?: string;
		/** Nombre del CSV (sin extensión). Sin valor, no se muestra el botón. */
		csvFilename?: string;
		/** Columnas del CSV si difieren de las visibles (p. ej. exportar todos los campos). */
		csvColumns?: Column<T>[];
		emptyText?: string;
		caption?: string;
		/** Filas por página (sin valor, todas). La búsqueda y el orden vuelven a la página 1. */
		pageSize?: number;
		class?: string;
	}

	let {
		rows,
		columns,
		cell,
		detail,
		rowKey,
		detailLabel = () => 'Ver detalle',
		initialSort = null,
		searchKeys,
		searchPlaceholder = 'Buscar…',
		initialQuery = '',
		csvFilename,
		csvColumns,
		emptyText = 'Sin datos para mostrar.',
		caption,
		pageSize,
		class: className
	}: Props = $props();

	let sort = $state<SortState>(null);
	// Solo el valor inicial: después el filtro es del usuario.
	let query = $state(untrack(() => initialQuery));
	let sortTouched = $state(false);
	let openKey = $state<string | null>(null);
	let pageIndex = $state(0);

	const expandable = $derived(!!detail && !!rowKey);
	const colCount = $derived(columns.length + (expandable ? 1 : 0));

	function toggleDetail(row: T) {
		const key = rowKey?.(row) ?? null;
		openKey = openKey === key ? null : key;
	}

	const effectiveSort = $derived(sortTouched ? sort : initialSort);
	const filtered = $derived(searchKeys ? filterRows(rows, query, searchKeys) : rows);
	const visible = $derived(sortRows(filtered, effectiveSort));
	const pageCount = $derived(pageSize ? Math.max(1, Math.ceil(visible.length / pageSize)) : 1);
	const currentPage = $derived(Math.min(pageIndex, pageCount - 1));
	const shown = $derived(
		pageSize ? visible.slice(currentPage * pageSize, (currentPage + 1) * pageSize) : visible
	);

	function toggle(column: Column<T>) {
		if (column.sortable === false) return;
		sort = nextSort(effectiveSort, column.key, !!column.numeric);
		sortTouched = true;
		pageIndex = 0;
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
		const blob = new Blob([toCsv(visible, csvColumns ?? columns)], {
			type: 'text/csv;charset=utf-8'
		});
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${csvFilename}.csv`;
		document.body.append(a);
		a.click();
		a.remove();
		// Revocar en el siguiente ciclo: algunos navegadores cancelan la descarga si es inmediato.
		setTimeout(() => URL.revokeObjectURL(url), 0);
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
						oninput={() => (pageIndex = 0)}
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
					{#if expandable}
						<th scope="col" class="w-8 bg-brand-strong px-1 py-2"
							><span class="sr-only">Detalle</span></th
						>
					{/if}
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
				{#each shown as row, i (rowKey ? rowKey(row) : i)}
					{@const isOpen = expandable && openKey === rowKey?.(row)}
					<tr
						class={cn(
							'border-b border-brand-border',
							i % 2 === 0 ? 'bg-brand-bg' : 'bg-brand-card'
						)}
					>
						{#if expandable}
							<td class="px-1 py-2 text-center">
								<button
									type="button"
									onclick={() => toggleDetail(row)}
									aria-expanded={isOpen}
									aria-label={detailLabel(row)}
									class="rounded px-1.5 text-brand-label hover:bg-brand-border"
									>{isOpen ? '▾' : '▸'}</button
								>
							</td>
						{/if}
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
					{#if isOpen && detail}
						<tr class="border-b border-brand-border bg-brand-blue-bg">
							<td colspan={colCount} class="px-4 py-4">{@render detail({ row })}</td>
						</tr>
					{/if}
				{:else}
					<tr>
						<td colspan={colCount} class="bg-brand-card p-6 text-center text-brand-muted">
							{emptyText}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	{#if pageCount > 1}
		<div class="flex items-center justify-between gap-2 text-sm text-brand-muted">
			<Button
				size="sm"
				variant="default"
				disabled={currentPage === 0}
				onclick={() => (pageIndex = currentPage - 1)}>Anterior</Button
			>
			<span class="tabular-nums">Página {currentPage + 1} de {pageCount}</span>
			<Button
				size="sm"
				variant="default"
				disabled={currentPage >= pageCount - 1}
				onclick={() => (pageIndex = currentPage + 1)}>Siguiente</Button
			>
		</div>
	{/if}
</div>
