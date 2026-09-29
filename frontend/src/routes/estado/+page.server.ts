import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

export const load: PageServerLoad = async (event) => {
	const api = backend(event);
	const [health, status, programas] = await Promise.all([
		tryCall(api.GET('/api/health')),
		tryCall(api.GET('/api/status')),
		tryCall(api.GET('/api/jobs/programas'))
	]);
	return { health, status, programas };
};
