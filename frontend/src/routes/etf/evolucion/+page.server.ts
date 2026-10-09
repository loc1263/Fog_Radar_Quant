import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';
import { parDeCorridas } from '$lib/historial';

/** Corridas recientes que cuenta la tabla de permanencia en el Top 15. */
const VENTANA_TOP = 12;
const TICKER = /^[A-Za-z0-9.-]{1,12}$/;

/**
 * Evolución de ETFs (history.db): rotación del Top 15 entre dos corridas
 * cualesquiera, permanencia, trayectoria de un ETF y categorías en el tiempo.
 */
export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const api = backend(event);

	const [runs, top, categorias] = await Promise.all([
		tryCall(api.GET('/api/etf/history/runs')),
		tryCall(api.GET('/api/etf/history/top', { params: { query: { ultimas: VENTANA_TOP } } })),
		tryCall(api.GET('/api/etf/history/categorias'))
	]);

	const claves = runs.ok ? runs.data.map((r) => r.run_key) : [];
	const tickers = top.ok ? top.data.map((t) => ({ ticker: t.ticker, nombre: t.nombre })) : [];
	// Cualquier ticker válido (p. ej. un enlace desde la rotación), no solo los del selector.
	const pedido = q.get('ticker')?.toUpperCase() ?? '';
	const ticker = TICKER.test(pedido) ? pedido : (tickers[0]?.ticker ?? null);
	const par = parDeCorridas(claves, q.get('desde'), q.get('hasta'));

	const [serie, diff] = await Promise.all([
		ticker
			? tryCall(api.GET('/api/etf/history/ticker/{ticker}', { params: { path: { ticker } } }))
			: null,
		par ? tryCall(api.GET('/api/etf/history/diff', { params: { query: par } })) : null
	]);

	return {
		runs,
		top,
		categorias: categorias.ok ? categorias.data : [],
		ventana: VENTANA_TOP,
		tickers,
		ticker,
		par,
		serie,
		diff
	};
};
