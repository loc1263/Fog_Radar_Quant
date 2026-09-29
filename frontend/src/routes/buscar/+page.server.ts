import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';
import { limpiarConsulta } from '$lib/transversal';

/** Buscador global: `?q=` en la URL (formulario GET, funciona sin JavaScript). */
export const load: PageServerLoad = async (event) => {
	const q = limpiarConsulta(event.url.searchParams.get('q'));
	if (!q) return { q, resultados: null };
	const api = backend(event);
	const resultados = await tryCall(
		api.GET('/api/buscar', { params: { query: { q, limite: 50 } } })
	);
	return { q, resultados };
};
