import type { LayoutServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

/** Última corrida (reemplaza el dashboard_data.json estático del original). */
export const load: LayoutServerLoad = async (event) => {
	return { latest: await tryCall(backend(event).GET('/api/tendencia/latest')) };
};
