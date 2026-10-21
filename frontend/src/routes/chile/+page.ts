import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Resumen de Chile: el ranking y los indicadores (una tarjeta por sección del menú). */
export const load: PageLoad = async ({ fetch }) => {
	const [ranking, indicadores] = await Promise.all([
		datos(fetch, '/api/cl/ranking'),
		datos(fetch, '/api/cl/indicadores')
	]);
	return { ranking, indicadores };
};
