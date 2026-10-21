import type { PageLoad } from './$types';
import { cargarCandidatos } from '$lib/us';

export const load: PageLoad = ({ fetch }) => cargarCandidatos(fetch);
