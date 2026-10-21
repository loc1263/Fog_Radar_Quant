import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	indicadores: await datos(fetch, '/api/cl/indicadores')
});
