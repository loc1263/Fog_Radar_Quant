import type { Actions, PageServerLoad } from './$types';
import { fail } from '@sveltejs/kit';
import type { UsPortfolioLimits } from '$lib/api/types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';
import { parseFraction, parseInt10, parseTickers, parseViews } from '$lib/modules/us/portfolio';

const DEFAULT_LIMITS: UsPortfolioLimits = {
	min_tickers: 2,
	min_assets: 2,
	max_assets: 20,
	default_assets: 10,
	default_max_weight: 0.25,
	default_risk_free_rate: 0.04
};

export const load: PageServerLoad = async (event) => {
	const api = backend(event);
	const [candidates, limits] = await Promise.all([
		tryCall(api.GET('/api/us/rankings/latest/candidates')),
		tryCall(api.GET('/api/us/portfolio/limits'))
	]);
	return { candidates, limits: limits.ok ? limits.data : DEFAULT_LIMITS };
};

export const actions: Actions = {
	optimizar: async (event) => {
		const form = await event.request.formData();
		const tickers = parseTickers(form);
		const res = await tryCall(
			backend(event).POST('/api/us/portfolio/optimize', {
				body: {
					tickers,
					n_assets: parseInt10(form.get('n_assets'), DEFAULT_LIMITS.default_assets),
					max_weight: parseFraction(form.get('max_weight'), DEFAULT_LIMITS.default_max_weight),
					views: parseViews(form, tickers) ?? null,
					risk_free_rate: null
				}
			})
		);
		if (!res.ok) return fail(res.status ?? 400, { error: res.message });
		return { optimizacion: res.data };
	}
};
