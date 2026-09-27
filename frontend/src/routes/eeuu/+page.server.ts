import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';
import { SPARK_DAYS } from '$lib/server/us';

export const load: PageServerLoad = async (event) => {
	const api = backend(event);
	const [ranking, prices] = await Promise.all([
		tryCall(api.GET('/api/us/rankings/latest')),
		tryCall(api.GET('/api/us/rankings/latest/prices', { params: { query: { days: SPARK_DAYS } } }))
	]);
	return { ranking, prices: prices.ok ? prices.data.prices : {} };
};
