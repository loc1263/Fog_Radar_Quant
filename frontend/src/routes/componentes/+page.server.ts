import { dev } from '$app/environment';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

/** Vitrina de componentes: solo existe en desarrollo. */
export const load: PageServerLoad = () => {
	if (!dev) error(404, { message: 'Página no encontrada.' });
	return {};
};
