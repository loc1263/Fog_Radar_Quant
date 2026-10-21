import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	resumen: await datos(fetch, '/api/etf/resumen')
});
