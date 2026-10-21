import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	rotacion: await datos(fetch, '/api/etf/rotacion')
});
