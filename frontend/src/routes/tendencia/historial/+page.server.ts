import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

const FECHA = /^\d{4}-\d{2}-\d{2}$/;
const TICKER = /^[A-Za-z0-9.\-^=]{1,15}$/;

/**
 * Vista Historial: todas las corridas guardadas en history.db. Los filtros viven
 * en la URL (?analisis=us&desde=…&hasta=…&ticker=…): formulario GET, sin JavaScript.
 */
export const load: PageServerLoad = async (event) => {
	const q = event.url.searchParams;
	const analysis = q.get('analisis') === 'us' ? '_us' : '';
	const desde = FECHA.test(q.get('desde') ?? '') ? q.get('desde') : null;
	const hasta = FECHA.test(q.get('hasta') ?? '') ? q.get('hasta') : null;
	const api = backend(event);

	const [analyses, runs, sectors, leaders, tickers] = await Promise.all([
		tryCall(api.GET('/api/tendencia/analyses')),
		tryCall(
			api.GET('/api/tendencia/runs', {
				params: {
					query: { analysis, from: desde ?? undefined, to: hasta ?? undefined, limit: 5000 }
				}
			})
		),
		tryCall(api.GET('/api/tendencia/history/sectors', { params: { query: { analysis } } })),
		tryCall(
			api.GET('/api/tendencia/history/momentum-leaders', {
				params: { query: { analysis, top: 3 } }
			})
		),
		tryCall(api.GET('/api/tendencia/history/tickers', { params: { query: { analysis } } }))
	]);

	// history.db devuelve (ticker, nombre) distintos: un ticker cuyo nombre cambió entre
	// corridas aparece dos veces. Se deja una entrada por ticker (el nombre más reciente).
	const lista = [...new Map((tickers.ok ? tickers.data : []).map((t) => [t.ticker, t])).values()];
	const pedido = q.get('ticker');
	const ticker =
		pedido && TICKER.test(pedido) && lista.some((t) => t.ticker === pedido)
			? pedido
			: (lista[0]?.ticker ?? null);
	const [trayectoria, universo] = ticker
		? await Promise.all([
				tryCall(
					api.GET('/api/tendencia/history/ticker/{ticker}', {
						params: { path: { ticker }, query: { analysis } }
					})
				),
				// Pesos de Markowitz del universo completo (history.db v2): cuán cerca del
				// top estuvo el ticker en las corridas en que no quedó seleccionado.
				tryCall(
					api.GET('/api/tendencia/history/universe/{ticker}', {
						params: { path: { ticker }, query: { analysis } }
					})
				)
			])
		: [null, null];

	return {
		filtros: { analisis: analysis ? 'us' : '', desde, hasta, ticker },
		analyses,
		runs,
		sectors: sectors.ok ? sectors.data : [],
		leaders: leaders.ok ? leaders.data : [],
		tickers: lista,
		trayectoria,
		universo: universo?.ok ? universo.data : []
	};
};
