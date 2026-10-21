import type { PageLoad } from './$types';
import { datos, datosDiff } from '$lib/datos';
import { elegir, parDeCorridas } from '$lib/historial';

/** Corridas recientes que cuenta la tabla de permanencia en el top (el export usa el default del backend). */
const VENTANA_TOP = 12;

/**
 * Historial del screener EEUU (history.db): permanencia en el top, qué entró y
 * salió entre dos corridas y la evolución de un ticker. Filtros en la URL.
 */
export const load: PageLoad = async ({ fetch, url }) => {
	const q = url.searchParams;

	const [runs, top] = await Promise.all([
		datos(fetch, '/api/us/history/runs'),
		datos(fetch, '/api/us/history/top')
	]);

	const claves = runs.ok ? runs.data.map((r) => r.run_id) : [];
	// Opciones del selector: los que pasaron por el top en la ventana (más frecuentes primero).
	const tickers = top.ok ? top.data.map((t) => ({ ticker: t.ticker, nombre: t.nombre })) : [];
	const ticker = elegir(
		q.get('ticker')?.toUpperCase() ?? null,
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
		ticker ? datos(fetch, '/api/us/history/ticker/{ticker}', { ticker }) : null,
		par ? datosDiff(fetch, '/api/us/history/diff', par.desde, par.hasta) : null
	]);

	return { runs, top, ventana: VENTANA_TOP, tickers, ticker, par, serie, diff };
};
