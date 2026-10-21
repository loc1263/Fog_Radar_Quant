import type { PageLoad } from './$types';
import type { Loaded, BusquedaResponse } from '$lib/api/types';
import { datos } from '$lib/datos';
import { buscarEnIndice, limpiarConsulta } from '$lib/transversal';

/** Buscador global: `?q=` en la URL (formulario GET). Busca en el índice exportado, en el navegador. */
export const load: PageLoad = async ({ fetch, url }) => {
	const q = limpiarConsulta(url.searchParams.get('q'));
	if (!q) return { q, resultados: null };
	const indice = await datos(fetch, '/api/buscar/indice');
	const resultados: Loaded<BusquedaResponse> = indice.ok
		? { ok: true, data: buscarEnIndice(indice.data.entradas, q, 50) }
		: indice;
	return { q, resultados };
};
