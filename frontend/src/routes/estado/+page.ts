import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Estado del sitio: la instantánea (health, módulos y programas) de la última exportación. */
export const load: PageLoad = async ({ fetch }) => {
	const [health, status, programas] = await Promise.all([
		datos(fetch, '/api/health'),
		datos(fetch, '/api/status'),
		datos(fetch, '/api/jobs/programas')
	]);
	return { health, status, programas };
};
