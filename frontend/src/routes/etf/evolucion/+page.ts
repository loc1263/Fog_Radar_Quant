import type { PageLoad } from './$types';
import { datos, datosDiff } from '$lib/datos';
import { elegir } from '$lib/historial';

/** Corridas recientes que cuenta la tabla de permanencia en el Top 15 (el export usa el default del backend). */
const VENTANA_TOP = 12;
const TICKER = /^[A-Za-z0-9.-]{1,12}$/;

/**
 * Evolución de ETFs (history.db): rotación del Top 15 de una corrida contra la anterior,
 * permanencia, trayectoria de un ETF y categorías en el tiempo.
 */
export const load: PageLoad = async ({ fetch, url }) => {
	const q = url.searchParams;

	const [runs, top, categorias] = await Promise.all([
		datos(fetch, '/api/etf/history/runs'),
		datos(fetch, '/api/etf/history/top'),
		datos(fetch, '/api/etf/history/categorias')
	]);

	const claves = runs.ok ? runs.data.map((r) => r.run_key) : [];
	const tickers = top.ok ? top.data.map((t) => ({ ticker: t.ticker, nombre: t.nombre })) : [];
	const pedido = q.get('ticker')?.toUpperCase() ?? '';
	const ticker = TICKER.test(pedido) ? pedido : (tickers[0]?.ticker ?? null);

	// Una corrida contra la anterior; por defecto, la última.
	const hasta = elegir(q.get('hasta'), claves.slice(1), claves.at(-1) ?? null);
	const idx = hasta ? claves.indexOf(hasta) : -1;
	const par = idx > 0 ? { desde: claves[idx - 1], hasta: claves[idx] } : null;

	const [serie, diff] = await Promise.all([
		ticker ? datos(fetch, '/api/etf/history/ticker/{ticker}', { ticker }) : null,
		par ? datosDiff(fetch, '/api/etf/history/diff', par.desde, par.hasta) : null
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
