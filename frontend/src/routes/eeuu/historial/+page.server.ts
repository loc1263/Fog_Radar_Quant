import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';
import { elegir, parDeCorridas } from '$lib/historial';

/** Corridas recientes que cuenta la tabla de permanencia en el top. */
const VENTANA_TOP = 20;

/**
 * Historial del screener EEUU (history.db): permanencia en el top, qué entró y
 * salió entre dos corridas y la evolución de un ticker. Filtros en la URL.
 */
export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const api = backend(event);

	const [runs, top] = await Promise.all([
		tryCall(api.GET('/api/us/history/runs')),
		tryCall(api.GET('/api/us/history/top', { params: { query: { ultimas: VENTANA_TOP } } }))
	]);

	const claves = runs.ok ? runs.data.map((r) => r.run_id) : [];
	// Opciones del selector: los que pasaron por el top en la ventana (más frecuentes primero).
	const tickers = top.ok ? top.data.map((t) => ({ ticker: t.ticker, nombre: t.nombre })) : [];
	const ticker = elegir(
		q.get('ticker')?.toUpperCase() ?? null,
		tickers.map((t) => t.ticker),
		tickers[0]?.ticker ?? null
	);
	const par = parDeCorridas(claves, q.get('desde'), q.get('hasta'));

	const [serie, diff] = await Promise.all([
		ticker
			? tryCall(api.GET('/api/us/history/ticker/{ticker}', { params: { path: { ticker } } }))
			: null,
		par ? tryCall(api.GET('/api/us/history/diff', { params: { query: par } })) : null
	]);

	return { runs, top, ventana: VENTANA_TOP, tickers, ticker, par, serie, diff };
};
