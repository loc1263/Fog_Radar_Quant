import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

export const load: PageLoad = async ({ fetch }) => ({
	tabla: await datos(fetch, '/api/etf/top15-vanguard')
});
