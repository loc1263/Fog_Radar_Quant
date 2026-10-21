import type { PageLoad } from './$types';
import { datos, datosDiff } from '$lib/datos';
import { elegir, parDeCorridas } from '$lib/historial';

/**
 * Historial del ranking IPSA (history.db). Ticker y corridas a comparar en la URL
 * (?ticker=…&desde=…&hasta=…): formulario GET.
 */
export const load: PageLoad = async ({ fetch, url }) => {
	const q = url.searchParams;

	const [runs, ranking] = await Promise.all([
		datos(fetch, '/api/cl/history/runs'),
		datos(fetch, '/api/cl/ranking')
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
	const elegido = parDeCorridas(claves, q.get('desde'), q.get('hasta'));
	// El export guarda cada par en orden cronológico (desde < hasta).
	const par =
		elegido && elegido.desde > elegido.hasta
			? { desde: elegido.hasta, hasta: elegido.desde }
			: elegido;

	const [serie, diff] = await Promise.all([
		ticker && claves.length ? datos(fetch, '/api/cl/history/ticker/{ticker}', { ticker }) : null,
		par ? datosDiff(fetch, '/api/cl/history/diff', par.desde, par.hasta) : null
	]);

	return { runs, tickers, ticker, par, serie, diff };
};
