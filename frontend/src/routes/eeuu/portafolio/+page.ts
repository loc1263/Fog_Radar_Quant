import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** `pedido` si es una de las opciones; si no, `porDefecto`. */
function elegir<T extends string>(
	pedido: string | null,
	opciones: readonly T[],
	porDefecto: T | null
) {
	return opciones.find((o) => o === pedido) ?? porDefecto;
}

/** Portafolios precalculados por el batch (el sitio estático no optimiza a pedido): `?u=top30`. */
export const load: PageLoad = async ({ fetch, url }) => {
	const precalc = await datos(fetch, '/api/us/portfolio/precalculado');
	const claves = precalc.ok ? precalc.data.portafolios.map((p) => p.clave) : [];
	const clave = elegir(url.searchParams.get('u'), claves, claves[0] ?? null);
	return { precalc, clave };
};
