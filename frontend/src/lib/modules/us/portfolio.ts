/**
 * Lectura de los formularios del portafolio (usado por la form action del
 * servidor y por la página). Lógica pura, sin I/O.
 */

/** Atajos de selección: los N mejores por score (el universo inicial es todos los candidatos). */
export const TOP_UNIVERSES = [15, 30];

const TICKER_RE = /^[A-Z0-9.\-^=]{1,15}$/;

/** Tickers marcados (`name="tickers"`), normalizados, válidos y sin repetir. */
export function parseTickers(form: FormData): string[] {
	const out = new Set<string>();
	for (const raw of form.getAll('tickers')) {
		const t = String(raw).trim().toUpperCase();
		if (TICKER_RE.test(t)) out.add(t);
	}
	return [...out];
}

/**
 * Views de Black-Litterman (`name="view:<TICKER>"`, en % anual) → fracción.
 * Solo de tickers seleccionados; vacíos o no numéricos se ignoran.
 */
export function parseViews(form: FormData, selected: string[]): Record<string, number> | undefined {
	const allowed = new Set(selected);
	const views: Record<string, number> = {};
	for (const [key, raw] of form.entries()) {
		if (!key.startsWith('view:')) continue;
		const ticker = key.slice(5).toUpperCase();
		const text = String(raw).trim().replace(',', '.');
		if (!allowed.has(ticker) || text === '') continue;
		const n = Number(text);
		if (Number.isFinite(n)) views[ticker] = n / 100;
	}
	return Object.keys(views).length ? views : undefined;
}

export function parseInt10(value: FormDataEntryValue | null, fallback: number): number {
	const n = Number.parseInt(String(value ?? ''), 10);
	return Number.isFinite(n) ? n : fallback;
}

/** Porcentaje del formulario (`25` o `25,5`) → fracción; vacío o inválido → `fallback`. */
export function parseFraction(value: FormDataEntryValue | null, fallback: number): number {
	const text = String(value ?? '')
		.trim()
		.replace(',', '.');
	if (text === '') return fallback;
	const n = Number(text);
	return Number.isFinite(n) ? n / 100 : fallback;
}
