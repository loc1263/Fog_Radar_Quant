import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

const TICKER_RE = /^[A-Z0-9.\-^=]{1,15}$/;

export const load: PageServerLoad = async (event) => {
	const ticker = event.params.ticker.trim().toUpperCase();
	if (!TICKER_RE.test(ticker)) error(404, { message: 'Ticker inválido.', code: 'not_found' });
	const evaluation = await tryCall(
		backend(event, { timeoutMs: 30_000 }).GET('/api/us/tickers/{ticker}/evaluate', {
			params: { path: { ticker } }
		})
	);
	return { ticker, evaluation };
};
