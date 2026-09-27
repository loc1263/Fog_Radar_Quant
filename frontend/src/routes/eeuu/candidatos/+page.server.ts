import type { PageServerLoad } from './$types';
import { loadCandidates } from '$lib/server/us';

export const load: PageServerLoad = (event) => loadCandidates(event);
