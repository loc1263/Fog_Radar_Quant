import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	cats: await datos(fetch, '/api/etf/categorias/ranking')
});
