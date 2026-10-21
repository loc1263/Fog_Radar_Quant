import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Resumen de EEUU: la corrida y un indicador de cada sección del menú. */
export const load: PageLoad = async ({ fetch }) => {
	const [ranking, candidates, precalc] = await Promise.all([
		datos(fetch, '/api/us/rankings/latest'),
		datos(fetch, '/api/us/rankings/latest/candidates'),
		datos(fetch, '/api/us/portfolio/precalculado')
	]);
	return { ranking, candidates, precalc };
};
