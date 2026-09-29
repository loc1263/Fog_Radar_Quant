import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

export const load: PageServerLoad = async (event) => {
	return { div: await tryCall(backend(event).GET('/api/etf/diversificacion')) };
};
