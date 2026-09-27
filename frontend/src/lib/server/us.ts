/** Cargas comunes del módulo EEUU (solo servidor). */
import type { RequestEvent } from '@sveltejs/kit';
import { backend } from './api';
import { tryCall } from './safe';

/** Días de la sparkline de tendencia (~3 meses hábiles, como el original). */
export const SPARK_DAYS = 60;

type Ev = Pick<RequestEvent, 'fetch' | 'locals'>;

/** Candidatos de la última corrida + cierres recientes para las sparklines. */
export async function loadCandidates(event: Ev) {
	const api = backend(event);
	const [candidates, prices] = await Promise.all([
		tryCall(api.GET('/api/us/rankings/latest/candidates')),
		tryCall(api.GET('/api/us/rankings/latest/prices', { params: { query: { days: SPARK_DAYS } } }))
	]);
	return { candidates, prices: prices.ok ? prices.data.prices : {} };
}
