import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Top 15 con sparklines y, para las tarjetas del resumen, los datos de las otras secciones del menú. */
export const load: PageLoad = async ({ fetch }) => {
	const [ranking, prices, candidates, precalc] = await Promise.all([
		datos(fetch, '/api/us/rankings/latest'),
		datos(fetch, '/api/us/rankings/latest/prices'),
		datos(fetch, '/api/us/rankings/latest/candidates'),
		datos(fetch, '/api/us/portfolio/precalculado')
	]);
	return { ranking, prices: prices.ok ? prices.data.prices : {}, candidates, precalc };
};
