import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => {
	const [resumen, top15] = await Promise.all([
		datos(fetch, '/api/etf/resumen'),
		datos(fetch, '/api/etf/top15')
	]);
	return { resumen, top15 };
};
