import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	ranking: await datos(fetch, '/api/cl/ranking')
});
