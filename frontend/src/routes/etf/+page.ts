import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Resumen de ETF: la corrida y un indicador de cada sección del menú (las que fallen quedan en blanco). */
export const load: PageLoad = async ({ fetch }) => {
	const [resumen, top15, rotacion, diversificacion, categorias, simple, vanguard, momentum, cap] =
		await Promise.all([
			datos(fetch, '/api/etf/resumen'),
			datos(fetch, '/api/etf/top15'),
			datos(fetch, '/api/etf/rotacion'),
			datos(fetch, '/api/etf/diversificacion'),
			datos(fetch, '/api/etf/categorias/ranking'),
			datos(fetch, '/api/etf/top15-simple'),
			datos(fetch, '/api/etf/top15-vanguard'),
			datos(fetch, '/api/etf/momentum/top15'),
			datos(fetch, '/api/etf/russell3000')
		]);
	return { resumen, top15, rotacion, diversificacion, categorias, simple, vanguard, momentum, cap };
};
