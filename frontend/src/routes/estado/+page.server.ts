import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

export const load: PageServerLoad = async (event) => {
	const api = backend(event);
	const [health, status] = await Promise.all([
		tryCall(api.GET('/api/health')),
		tryCall(api.GET('/api/status'))
	]);
	return { health, status };
};
