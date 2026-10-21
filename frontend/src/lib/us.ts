/** Cargas comunes del módulo EEUU (lee el export estático). */
import { datos } from '$lib/datos';

/** Candidatos de la última corrida + cierres recientes para las sparklines (el export ya trae ~3 meses). */
export async function cargarCandidatos(fetch: typeof globalThis.fetch) {
	const [candidates, prices] = await Promise.all([
		datos(fetch, '/api/us/rankings/latest/candidates'),
		datos(fetch, '/api/us/rankings/latest/prices')
	]);
	return { candidates, prices: prices.ok ? prices.data.prices : {} };
}
