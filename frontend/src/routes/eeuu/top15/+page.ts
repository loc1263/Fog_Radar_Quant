import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Top 15 con sparklines de tendencia (cierres de los últimos ~3 meses). */
export const load: PageLoad = async ({ fetch }) => {
	const [ranking, prices] = await Promise.all([
		datos(fetch, '/api/us/rankings/latest'),
		datos(fetch, '/api/us/rankings/latest/prices')
	]);
	return { ranking, prices: prices.ok ? prices.data.prices : {} };
};
