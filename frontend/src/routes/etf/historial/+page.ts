import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	historial: await datos(fetch, '/api/etf/historial')
});
