import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	cap: await datos(fetch, '/api/etf/russell3000')
});
