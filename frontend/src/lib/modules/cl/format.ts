/** Formateo de indicadores del IPSA según la unidad declarada por el backend. */
import type { ClUnidad } from '$lib/api/types';
import { fmtCompact, fmtDate, fmtNumber, fmtPct } from '$lib/format';

const EMPTY = '—';

export function esNumerica(unidad: ClUnidad): boolean {
	return unidad !== 'texto' && unidad !== 'fecha';
}

export function fmtValor(unidad: ClUnidad, v: unknown): string {
	if (v === null || v === undefined || v === '') return EMPTY;
	switch (unidad) {
		case 'texto':
			return String(v);
		case 'numero':
			return fmtNumber(v);
		case 'multiplo':
			return typeof v === 'number' && Number.isFinite(v) ? `${fmtNumber(v)}x` : EMPTY;
		case 'pct':
			return fmtPct(v);
		case 'pct_pts':
			return fmtPct(v, { isPercent: true, digits: 2 });
		case 'monto':
		case 'cantidad':
			return fmtCompact(v);
		case 'fecha':
			return typeof v === 'number' && Number.isFinite(v) ? fmtDate(new Date(v * 1000)) : EMPTY;
	}
}

/** z-score con 2 decimales y signo explícito. */
export function fmtZ(v: number | null | undefined): string {
	if (typeof v !== 'number' || !Number.isFinite(v)) return EMPTY;
	return `${v > 0 ? '+' : ''}${fmtNumber(v)}`;
}
