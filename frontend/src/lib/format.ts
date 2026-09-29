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

const ISO_SIN_ZONA = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?$/;

/** Minutos que `TIME_ZONE` está adelantada respecto de UTC en el instante `d`. */
function offsetMinutos(d: Date): number {
	const p = Object.fromEntries(
		new Intl.DateTimeFormat('en-US', {
			timeZone: TIME_ZONE,
			hourCycle: 'h23',
			year: 'numeric',
			month: '2-digit',
			day: '2-digit',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		})
			.formatToParts(d)
			.map((x) => [x.type, x.value])
	);
	const pared = Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute, +p.second);
	return Math.round((pared - d.getTime()) / 60000);
}

/**
 * `Date` de un ISO; si viene **sin zona** (p. ej. `2026-09-28T22:11:08`) se interpreta
 * en `TIME_ZONE` y no en la zona del proceso: así SSR (servidor en UTC) y navegador
 * muestran la misma hora.
 */
export function parseFecha(v: string): Date {
	if (!ISO_SIN_ZONA.test(v)) return new Date(v);
	const comoUtc = new Date(`${v}Z`);
	const primera = new Date(comoUtc.getTime() - offsetMinutos(comoUtc) * 60000);
	return new Date(comoUtc.getTime() - offsetMinutos(primera) * 60000);
}

export function fmtDate(v: string | Date | null | undefined, withTime = false): string {
	if (!v) return EMPTY;
	const d = typeof v === 'string' ? parseFecha(v) : v;
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
