/**
 * Lógica pura de DataTable (orden, filtro, CSV), separada del componente
 * para poder testearla sin navegador.
 */

export type Row = Record<string, unknown>;
export type SortDir = 'asc' | 'desc';
export type SortState = { key: string; dir: SortDir } | null;

export interface Column<T extends Row = Row> {
	key: string;
	label: string;
	/** Números a la derecha con tabular-nums (§5.5). */
	numeric?: boolean;
	/** Colorea verde/rojo según signo. */
	signed?: boolean;
	sortable?: boolean;
	/** Texto de la celda (también se usa para el CSV si no hay `csv`). */
	format?: (value: unknown, row: T) => string;
	/** Valor exportado al CSV (por defecto el valor crudo). */
	csv?: (value: unknown, row: T) => string | number | null | undefined;
	/** Texto largo: puede partirse en varias líneas (min-w-[8rem]). */
	wrap?: boolean;
	/** Ayuda al pasar el mouse por el encabezado. */
	title?: string;
}

function compare(a: unknown, b: unknown): number {
	const aNil = a === null || a === undefined || (typeof a === 'number' && Number.isNaN(a));
	const bNil = b === null || b === undefined || (typeof b === 'number' && Number.isNaN(b));
	if (aNil && bNil) return 0;
	if (aNil) return 1; // vacíos siempre al final
	if (bNil) return -1;
	if (typeof a === 'number' && typeof b === 'number') return a - b;
	return String(a).localeCompare(String(b), 'es', { numeric: true, sensitivity: 'base' });
}

export function sortRows<T extends Row>(rows: readonly T[], sort: SortState): T[] {
	const copy = [...rows];
	if (!sort) return copy;
	const sign = sort.dir === 'asc' ? 1 : -1;
	return copy.sort((ra, rb) => {
		const a = ra[sort.key];
		const b = rb[sort.key];
		const c = compare(a, b);
		// Los vacíos quedan al final en ambos sentidos.
		const aNil = a === null || a === undefined;
		const bNil = b === null || b === undefined;
		return aNil || bNil ? c : c * sign;
	});
}

/** Ciclo al hacer clic: sin orden → desc (números) / asc (texto) → inverso → sin orden. */
export function nextSort(current: SortState, key: string, numeric: boolean): SortState {
	const first: SortDir = numeric ? 'desc' : 'asc';
	if (!current || current.key !== key) return { key, dir: first };
	if (current.dir === first) return { key, dir: first === 'asc' ? 'desc' : 'asc' };
	return null;
}

/**
 * Celda CSV. Los textos que empiezan con `= + - @` (o tab/CR) se prefijan con `'`
 * para que Excel no los ejecute como fórmula (inyección CSV, OWASP); los números
 * negativos, que son `number`, se exportan tal cual.
 */
export function csvCell(v: unknown): string {
	if (v === null || v === undefined) return '';
	if (typeof v === 'number') return String(v);
	let s = String(v);
	if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`;
	return /[",;\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** CSV con `;` y BOM para que Excel en español lo abra con columnas. */
export function toCsv<T extends Row>(rows: readonly T[], columns: Column<T>[]): string {
	const header = columns.map((c) => csvCell(c.label)).join(';');
	const lines = rows.map((r) =>
		columns.map((c) => csvCell(c.csv ? c.csv(r[c.key], r) : r[c.key])).join(';')
	);
	return '﻿' + [header, ...lines].join('\r\n');
}
