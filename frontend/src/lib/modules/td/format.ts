/**
 * Formato y lógica de presentación del módulo Tendencia (port de `format.js`,
 * `viz.js` y `latestData.js` del original Vue).
 */
import type { TdAnalisis, TdDashboard, TdHolding } from '$lib/api/types';
import { fmtNumber, fmtPct } from '$lib/format';

const EMPTY = '—';

function isNum(v: unknown): v is number {
	return typeof v === 'number' && Number.isFinite(v);
}

/** Fracción → porcentaje (0,1234 → "12,3 %"). */
export function pct(v: unknown, digits = 1): string {
	return fmtPct(v, { digits });
}

/** Valor que ya viene en puntos porcentuales (12,34 → "12,3 %"). */
export function pctPts(v: unknown, digits = 1, signed = false): string {
	return fmtPct(v, { digits, isPercent: true, signed });
}

export function money(v: unknown, digits = 2): string {
	return isNum(v) ? `US$ ${fmtNumber(v, digits)}` : EMPTY;
}

// --- Señales técnicas ------------------------------------------------------------
export type Senal = 'positive' | 'negative' | 'mixed' | 'neutral';

const SENALES: Record<string, Senal> = {
	Alcista: 'positive',
	Bajista: 'negative',
	'Lateral / mixta': 'mixed',
	'Datos insuficientes': 'neutral'
};

export function senal(v: string | null | undefined): Senal {
	return (v && SENALES[v]) || 'neutral';
}

// --- Análisis (?analisis=us) -------------------------------------------------------
/** Clave del backend → valor en la URL: "" → "", "_us" → "us". */
export function analisisSlug(key: string): string {
	return key.replace(/^_/, '');
}

/** El análisis pedido en la URL, o el principal si no existe. */
export function elegirAnalisis(d: TdDashboard, slug: string | null): TdAnalisis | null {
	return d.analyses.find((a) => analisisSlug(a.key) === (slug ?? '')) ?? d.analyses[0] ?? null;
}

/** Sufijo para títulos: "" para el principal, " (solo EEUU)" para los demás. */
export function sufijoAnalisis(a: TdAnalisis | null): string {
	return a?.key ? ` (${a.label})` : '';
}

// --- Portafolio ------------------------------------------------------------------
export const TAMANO_POR_DEFECTO = 10;

export interface Portafolio {
	/** Tamaño elegido (el portafolio completo usa `n_selected`). */
	n: number;
	completo: boolean;
	holdings: TdHolding[];
	retorno: number;
	volatilidad: number;
	sharpe: number;
}

/**
 * Portafolio a mostrar: una de las opciones reducidas (re-optimizadas solo sobre
 * las N mejores) o el completo. Corridas antiguas no traen opciones: completo.
 */
export function elegirPortafolio(a: TdAnalisis, n: number | null): Portafolio {
	const completo: Portafolio = {
		n: a.n_selected,
		completo: true,
		holdings: a.holdings,
		retorno: a.performance.portafolio_15_finales.retorno_esperado_anual,
		volatilidad: a.performance.portafolio_15_finales.volatilidad_anual,
		sharpe: a.performance.portafolio_15_finales.sharpe_ratio
	};
	const opciones = a.portfolio_options;
	if (opciones.length === 0) return completo;
	if (n !== null && n === a.n_selected) return completo;
	const o =
		opciones.find((x) => x.n === n) ??
		opciones.find((x) => x.n === TAMANO_POR_DEFECTO) ??
		opciones[0];
	return {
		n: o.n,
		completo: false,
		holdings: o.holdings,
		retorno: o.performance.retorno_esperado_anual,
		volatilidad: o.performance.volatilidad_anual,
		sharpe: o.performance.sharpe_ratio
	};
}

/** Peso por país (fracciones), mayor primero (CountryAllocationBar del original). */
export function pesoPorPais(holdings: Pick<TdHolding, 'pais' | 'peso_final'>[]) {
	const acc = new Map<string, number>();
	for (const h of holdings) {
		const k = h.pais ?? 'Otro';
		acc.set(k, (acc.get(k) ?? 0) + (h.peso_final ?? 0));
	}
	return [...acc.entries()]
		.map(([label, value]) => ({ label, value }))
		.sort((a, b) => b.value - a.value);
}

export function separarTickers(v: string | null | undefined): string[] {
	return (v ?? '')
		.split(',')
		.map((t) => t.trim())
		.filter(Boolean);
}

// --- Historial -------------------------------------------------------------------
/** `20260924_02` → `24/09 #2` (`_01` → `24/09`): etiqueta corta y única por corrida. */
export function etiquetaCorrida(runDir: string): string {
	const m = /^(\d{4})(\d{2})(\d{2})_(\d{2})$/.exec(runDir);
	if (!m) return runDir;
	const seq = Number(m[4]);
	return `${m[3]}/${m[2]}${seq > 1 ? ` #${seq}` : ''}`;
}

/** `2026-09-24` → `24/09/2026`. */
export function fechaCorta(v: string | null | undefined): string {
	const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(v ?? '');
	return m ? `${m[3]}/${m[2]}/${m[1]}` : EMPTY;
}

const MAX_SERIES = 8;

/**
 * Filas planas {run_dir, grupo, valor} → una fila por corrida con una columna por
 * grupo (lo que necesita un gráfico de líneas). Más allá de 7 grupos, los de menor
 * peso acumulado se suman en "Otros" (pivotByGroup del original).
 */
export function pivotPorGrupo<R extends { run_dir: string }>(
	rows: R[],
	grupo: (r: R) => string | null | undefined,
	valor: (r: R) => number | null | undefined
): { filas: Record<string, string | number | null>[]; grupos: string[] } {
	const runs = [...new Set(rows.map((r) => r.run_dir))].sort();
	const totales = new Map<string, number>();
	for (const r of rows) {
		const g = grupo(r) ?? 'Otro';
		totales.set(g, (totales.get(g) ?? 0) + (valor(r) ?? 0));
	}
	const orden = [...totales.entries()].sort((a, b) => b[1] - a[1]).map(([g]) => g);
	const kept = orden.slice(0, MAX_SERIES - 1);
	const folded = new Set(orden.slice(MAX_SERIES - 1));
	const grupos = folded.size ? [...kept, 'Otros'] : kept;

	const celdas = new Map<string, number>();
	for (const r of rows) {
		const g0 = grupo(r) ?? 'Otro';
		const g = folded.has(g0) ? 'Otros' : g0;
		const k = `${r.run_dir}|${g}`;
		celdas.set(k, (celdas.get(k) ?? 0) + (valor(r) ?? 0));
	}
	const filas = runs.map((run) => ({
		corrida: etiquetaCorrida(run),
		...Object.fromEntries(grupos.map((g) => [g, celdas.get(`${run}|${g}`) ?? null]))
	}));
	return { filas, grupos };
}
