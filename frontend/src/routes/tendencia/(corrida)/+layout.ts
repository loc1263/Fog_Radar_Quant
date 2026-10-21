import type { LayoutLoad } from './$types';
import { datos } from '$lib/datos';

/** Última corrida (reemplaza el dashboard_data.json estático del original). */
export const load: LayoutLoad = async ({ fetch }) => ({
	latest: await datos(fetch, '/api/tendencia/latest')
});
