/**
 * Formateo y lógica de presentación del módulo ETF (port de `lib/format.tsx`,
 * `lib/csv.ts` y `Rango52SemanasBar.tsx` del original).
 */
import type { EtfFila } from '$lib/api/types';
import { fmtNumber, fmtPct } from '$lib/format';
import type { Column, Row } from '$lib/table';

const EMPTY = '—';

function isNum(v: unknown): v is number {
	return typeof v === 'number' && Number.isFinite(v);
}

/** Rendimientos del screener: ya vienen en puntos (12.3 → "+12,30 %"). */
export function fmtRend(v: unknown, digits = 2): string {
	return fmtPct(v, { digits, isPercent: true, signed: true });
}

/** Variación con signo sin unidad (+1,2 · −3). */
export function fmtDelta(v: unknown, digits = 1): string {
	if (!isNum(v)) return EMPTY;
	const r = Math.abs(v) < 0.5 * 10 ** -digits ? 0 : v; // sin "+0,000" por ruido de redondeo
	return `${r > 0 ? '+' : ''}${fmtNumber(r, digits)}`;
}

/** Montos en USD compactos (AUM): $1,15T · $262,4B · $15,2M. */
export function fmtUsd(v: unknown): string {
	if (!isNum(v)) return EMPTY;
	const abs = Math.abs(v);
	if (abs >= 1e12) return `$${fmtNumber(v / 1e12, 2)}T`;
	if (abs >= 1e9) return `$${fmtNumber(v / 1e9, 1)}B`;
	if (abs >= 1e6) return `$${fmtNumber(v / 1e6, 1)}M`;
	return `$${fmtNumber(v, 0)}`;
}

/** Fecha ISO de la corrida (`2026-09-24`) → `24/09/2026`, sin conversión de zona. */
export function fmtFechaCorrida(v: string | null | undefined): string {
	if (!v) return EMPTY;
	const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(v);
	return m ? `${m[3]}/${m[2]}/${m[1]}` : v;
}

/** `2026-09-24` → `20260924` (sufijo de los CSV exportados, como el original). */
export function fechaTag(v: string | null | undefined): string {
	return v ? v.replaceAll('-', '').slice(0, 8) : '';
}

export function csvName(base: string, fecha: string | null | undefined): string {
	const tag = fechaTag(fecha);
	return tag ? `${base}_${tag}` : base;
}

// --- Rango de 52 semanas -------------------------------------------------------
export const UMBRAL_ALTO = 80;
export const UMBRAL_BAJO = 20;

export interface Rango52 {
	/** Posición del precio dentro del rango (0 = mínimo, 100 = máximo). */
	pos: number;
	etiqueta: 'Cerca del máximo' | 'Cerca del mínimo' | 'Rango medio';
	tono: 'accent' | 'blue' | 'label';
	title: string;
}

export function rango52(fila: Pick<EtfFila, 'precio' | 'max_52s' | 'min_52s'>): Rango52 | null {
	const { precio, max_52s: high, min_52s: low } = fila;
	if (!isNum(precio) || !isNum(low) || !isNum(high) || high <= low) return null;
	const pos = Math.min(100, Math.max(0, ((precio - low) / (high - low)) * 100));
	const distMax = ((precio - high) / high) * 100;
	const [etiqueta, tono]: [Rango52['etiqueta'], Rango52['tono']] =
		pos >= UMBRAL_ALTO
			? ['Cerca del máximo', 'accent']
			: pos <= UMBRAL_BAJO
				? ['Cerca del mínimo', 'blue']
				: ['Rango medio', 'label'];
	const title =
		`Precio ${precio.toFixed(2)} · Rango 52S ${low.toFixed(2)} – ${high.toFixed(2)} · ` +
		`${distMax >= 0 ? '+' : ''}${distMax.toFixed(1)} % vs. máximo`;
	return { pos, etiqueta, tono, title };
}

// --- Columnas comunes ----------------------------------------------------------
type EtfRow = Row & Partial<EtfFila>;

const rend = <T extends EtfRow>(key: string, label: string): Column<T> => ({
	key,
	label,
	numeric: true,
	signed: true,
	format: (v) => fmtRend(v)
});

const num =
	(digits: number) =>
	(v: unknown): string =>
		isNum(v) ? fmtNumber(v, digits) : EMPTY;

export const col = {
	pos: <T extends EtfRow>(): Column<T> => ({ key: 'pos', label: '#', numeric: true }),
	ticker: <T extends EtfRow>(): Column<T> => ({ key: 'ticker', label: 'Ticker' }),
	nombre: <T extends EtfRow>(): Column<T> => ({
		key: 'nombre',
		label: 'Nombre / Categoría',
		wrap: true,
		csv: (_v, r) => r.nombre ?? null
	}),
	rend,
	periodos: <T extends EtfRow>(ytd = false): Column<T>[] => [
		rend<T>('perf_5d', '5D'),
		rend<T>('perf_1m', '1M'),
		rend<T>('perf_6m', '6M'),
		...(ytd ? [rend<T>('perf_ytd', 'YTD')] : []),
		rend<T>('perf_1y', '1Y')
	],
	sharpe: <T extends EtfRow>(): Column<T> => ({
		key: 'sharpe',
		label: 'Sharpe',
		numeric: true,
		format: num(2)
	}),
	beta: <T extends EtfRow>(): Column<T> => ({
		key: 'beta',
		label: 'Beta',
		numeric: true,
		format: num(2)
	}),
	rango: <T extends EtfRow>(): Column<T> => ({
		key: 'rango52',
		label: 'Rango 52S',
		sortable: false,
		csv: () => null,
		title: 'Posición del precio entre el mínimo y el máximo de 52 semanas'
	}),
	score: <T extends EtfRow>(key = 'score', label = 'Score'): Column<T> => ({
		key,
		label,
		numeric: true,
		format: num(1)
	})
};

/**
 * Columnas del CSV exportado (mismas que `etfExportColumns` del original,
 * con la columna de score que corresponda a cada tabla).
 */
export function csvColumnas<T extends EtfRow>(
	scoreKey = 'score',
	scoreLabel = 'Score'
): Column<T>[] {
	const raw = (key: string, label: string): Column<T> => ({ key, label });
	return [
		raw('ticker', 'Ticker'),
		raw('nombre', 'Nombre'),
		raw('categoria', 'Categoría'),
		raw('perf_5d', 'Performance 5D (%)'),
		raw('perf_1m', 'Performance 1M (%)'),
		raw('perf_6m', 'Performance 6M (%)'),
		raw('perf_1y', 'Performance 1Y (%)'),
		raw('sharpe', 'Sharpe Ratio'),
		raw('beta', 'Beta'),
		raw(scoreKey, scoreLabel)
	];
}

/** Agrega la posición original (#) y el marcador de la columna de rango. */
export function conPosicion<T extends object>(filas: T[]): (T & { pos: number; rango52: null })[] {
	return filas.map((f, i) => ({ ...f, pos: i + 1, rango52: null }));
}

/** Peso agregado de la cartera por bucket de capitalización (0-1), mayor primero. */
export function pesoPorBucket(
	filas: { cap_bucket?: string | null; peso_portfolio?: number | null }[]
): { label: string; value: number }[] {
	const acc = new Map<string, number>();
	for (const f of filas) {
		const k = f.cap_bucket ?? 'Otros';
		acc.set(k, (acc.get(k) ?? 0) + (isNum(f.peso_portfolio) ? f.peso_portfolio : 0));
	}
	return [...acc.entries()]
		.map(([label, pct]) => ({ label, value: pct / 100 }))
		.filter((s) => s.value > 0)
		.sort((a, b) => b.value - a.value);
}
