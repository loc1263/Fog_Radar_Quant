/**
 * Métodos de portafolio y lectura de los formularios (usado por las form
 * actions del servidor y por la página). Lógica pura, sin I/O.
 */
import type { UsPortfolioMethod, UsRankBy } from '$lib/api/types';

export interface MethodInfo {
	/** Segmento de URL (`/eeuu/portafolio/[metodo]`). */
	slug: string;
	api: UsPortfolioMethod;
	label: string;
	description: string;
}

export const METHODS: MethodInfo[] = [
	{
		slug: 'markowitz',
		api: 'markowitz',
		label: 'Markowitz',
		description:
			'Portafolio tangente de máximo Sharpe sobre la frontera eficiente media-varianza (long-only), estimado con el histórico de precios de la última corrida.'
	},
	{
		slug: 'black-litterman',
		api: 'black_litterman',
		label: 'Black-Litterman',
		description:
			'Combina el equilibrio de mercado (pesos por market cap) con views automáticas derivadas del score de salud de cada ticker. Puedes editar cualquier view antes de optimizar.'
	},
	{
		slug: 'hrp',
		api: 'hrp',
		label: 'HRP',
		description:
			'Hierarchical Risk Parity: asigna pesos por clustering jerárquico de la correlación entre tickers (López de Prado). No estima retornos esperados, solo covarianza.'
	}
];

export function methodBySlug(slug: string): MethodInfo | undefined {
	return METHODS.find((m) => m.slug === slug);
}

export const RANK_LABELS: Record<UsRankBy, string> = {
	expected_return: 'Retorno esperado',
	sharpe: 'Sharpe'
};

/** Selección inicial del universo: top N por momentum 12-1. */
export const DEFAULT_SELECTION = 15;

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

export function parseRankBy(value: FormDataEntryValue | null): UsRankBy {
	return value === 'sharpe' ? 'sharpe' : 'expected_return';
}
