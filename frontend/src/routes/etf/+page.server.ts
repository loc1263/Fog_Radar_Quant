import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

export const load: PageServerLoad = async (event) => {
	const api = backend(event);
	const [resumen, top15] = await Promise.all([
		tryCall(api.GET('/api/etf/resumen')),
		tryCall(api.GET('/api/etf/top15'))
	]);
	return { resumen, top15 };
};
