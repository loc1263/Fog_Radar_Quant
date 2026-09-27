import { error, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { UsPortfolioLimits } from '$lib/api/types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';
import {
	methodBySlug,
	parseInt10,
	parseRankBy,
	parseTickers,
	parseViews
} from '$lib/modules/us/portfolio';

/** La búsqueda combinatoria puede tardar ~15 s con el tope de 2.500 combinaciones. */
const LONG_TIMEOUT_MS = 60_000;

const DEFAULT_LIMITS: UsPortfolioLimits = {
	min_tickers: 2,
	min_combination_size: 2,
	max_combination_size: 10,
	max_combinations: 2500,
	default_risk_free_rate: 0.04
};

function method(slug: string) {
	const m = methodBySlug(slug);
	if (!m) error(404, { message: 'Método de portafolio desconocido.', code: 'not_found' });
	return m;
}

export const load: PageServerLoad = async (event) => {
	const api = backend(event);
	const [candidates, limits] = await Promise.all([
		tryCall(api.GET('/api/us/rankings/latest/candidates')),
		tryCall(api.GET('/api/us/portfolio/limits'))
	]);
	return {
		method: method(event.params.metodo),
		candidates,
		limits: limits.ok ? limits.data : DEFAULT_LIMITS
	};
};

export const actions: Actions = {
	optimizar: async (event) => {
		const m = method(event.params.metodo);
		const form = await event.request.formData();
		const tickers = parseTickers(form);
		const views = m.api === 'black_litterman' ? parseViews(form, tickers) : undefined;
		const res = await tryCall(
			backend(event).POST('/api/us/portfolio/optimize', {
				body: { method: m.api, tickers, views: views ?? null, risk_free_rate: null }
			})
		);
		if (!res.ok)
			return fail(res.status ?? 400, { accion: 'optimizar' as const, error: res.message });
		return { accion: 'optimizar' as const, optimizacion: res.data };
	},

	combinacion: async (event) => {
		const m = method(event.params.metodo);
		const form = await event.request.formData();
		const res = await tryCall(
			backend(event, { timeoutMs: LONG_TIMEOUT_MS }).POST('/api/us/portfolio/best_combination', {
				body: {
					method: m.api,
					tickers: parseTickers(form),
					k: parseInt10(form.get('k'), 5),
					rank_by: parseRankBy(form.get('rank_by')),
					top_n: 5,
					risk_free_rate: null
				}
			})
		);
		if (!res.ok)
			return fail(res.status ?? 400, { accion: 'combinacion' as const, error: res.message });
		return { accion: 'combinacion' as const, combinacion: res.data };
	}
};
