/** Formateo común (es-CL). Todas las celdas numéricas pasan por acá. */

const LOCALE = 'es-CL';
/** Zona fija: el servidor SSR y el navegador deben mostrar la misma hora (sin desajuste de hidratación). */
export const TIME_ZONE = 'America/Santiago';
const EMPTY = '—';

function isNum(v: unknown): v is number {
	return typeof v === 'number' && Number.isFinite(v);
}

export function fmtNumber(v: unknown, digits = 2): string {
	if (!isNum(v)) return EMPTY;
	return v.toLocaleString(LOCALE, { minimumFractionDigits: digits, maximumFractionDigits: digits });
}

/** `value` como fracción (0.123 → "12,3 %"), salvo `isPercent`. */
export function fmtPct(v: unknown, { digits = 1, isPercent = false, signed = false } = {}): string {
	if (!isNum(v)) return EMPTY;
	const pct = isPercent ? v : v * 100;
	const s = pct.toLocaleString(LOCALE, {
		minimumFractionDigits: digits,
		maximumFractionDigits: digits
	});
	return `${signed && pct > 0 ? '+' : ''}${s} %`;
}

/** Montos grandes compactos: 1.234.000.000 → "1,23 mil M". */
export function fmtCompact(v: unknown, digits = 2): string {
	if (!isNum(v)) return EMPTY;
	return new Intl.NumberFormat(LOCALE, {
		notation: 'compact',
		maximumFractionDigits: digits
	}).format(v);
}

export function fmtDate(v: string | Date | null | undefined, withTime = false): string {
	if (!v) return EMPTY;
	const d = typeof v === 'string' ? new Date(v) : v;
	if (Number.isNaN(d.getTime())) return EMPTY;
	return d.toLocaleString(LOCALE, {
		day: '2-digit',
		month: 'short',
		year: 'numeric',
		timeZone: TIME_ZONE,
		...(withTime ? { hour: '2-digit', minute: '2-digit' } : {})
	});
}

export function fmtDuration(seconds: number | null | undefined): string {
	if (!isNum(seconds)) return EMPTY;
	if (seconds < 60) return `${seconds.toFixed(1)} s`;
	const m = Math.floor(seconds / 60);
	const s = Math.round(seconds % 60);
	return m < 60 ? `${m} min ${s} s` : `${Math.floor(m / 60)} h ${m % 60} min`;
}

/** Clase de color para valores con signo (§5.5): verde ≥ 0, rojo < 0. */
export function signedClass(v: unknown): string {
	if (!isNum(v)) return '';
	return v >= 0 ? 'font-bold text-brand-green' : 'font-bold text-brand-red';
}
