import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';
import { elegir, parDeCorridas } from '$lib/historial';

/**
 * Historial del ranking IPSA (history.db). Ticker y corridas a comparar en la URL
 * (?ticker=…&desde=…&hasta=…): formulario GET, sin JavaScript.
 */
export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const api = backend(event);

	const [runs, ranking] = await Promise.all([
		tryCall(api.GET('/api/cl/history/runs')),
		tryCall(api.GET('/api/cl/ranking'))
	]);

	const claves = runs.ok ? runs.data.map((r) => r.ts) : [];
	const tickers = ranking.ok
		? ranking.data.filas.map((f) => ({ ticker: f.simbolo, nombre: f.nombre }))
		: [];
	const ticker = elegir(
		q.get('ticker'),
		tickers.map((t) => t.ticker),
		tickers[0]?.ticker ?? null
	);
	const par = parDeCorridas(claves, q.get('desde'), q.get('hasta'));

	const [serie, diff] = await Promise.all([
		ticker && claves.length
			? tryCall(api.GET('/api/cl/history/ticker/{ticker}', { params: { path: { ticker } } }))
			: null,
		par ? tryCall(api.GET('/api/cl/history/diff', { params: { query: par } })) : null
	]);

	return { runs, tickers, ticker, par, serie, diff };
};
