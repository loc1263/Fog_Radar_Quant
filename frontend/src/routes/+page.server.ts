import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

export const load: PageServerLoad = async (event) => {
	const api = backend(event);
	return { status: await tryCall(api.GET('/api/status')) };
};
