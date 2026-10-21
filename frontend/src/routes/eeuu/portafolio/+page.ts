import type { PageLoad } from './$types';
import { datos } from '$lib/datos';
import { elegir } from '$lib/historial';

/** Portafolios precalculados por el batch (el sitio estático no optimiza a pedido): `?u=top30`. */
export const load: PageLoad = async ({ fetch, url }) => {
	const precalc = await datos(fetch, '/api/us/portfolio/precalculado');
	const claves = precalc.ok ? precalc.data.portafolios.map((p) => p.clave) : [];
	const clave = elegir(url.searchParams.get('u'), claves, claves[0] ?? null);
	return { precalc, clave };
};
