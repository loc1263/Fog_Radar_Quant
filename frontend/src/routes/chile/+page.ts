import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Ranking IPSA y, para las tarjetas del resumen, los indicadores de la sección Indicadores. */
export const load: PageLoad = async ({ fetch }) => {
	const [ranking, indicadores] = await Promise.all([
		datos(fetch, '/api/cl/ranking'),
		datos(fetch, '/api/cl/indicadores')
	]);
	return { ranking, indicadores };
};
