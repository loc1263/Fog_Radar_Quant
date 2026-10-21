import { dev } from '$app/environment';
import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/** Vitrina de componentes: solo existe en desarrollo. */
export const load: PageLoad = () => {
	if (!dev) error(404, { message: 'Página no encontrada.' });
	return {};
};
