import type { PageServerLoad } from './$types';
import { backend } from '$lib/server/api';
import { tryCall } from '$lib/server/safe';

/** Campos que usa la tabla: el universo completo (~700 ETFs) no viaja entero al navegador. */
const CAMPOS = [
	'ticker',
	'nombre',
	'categoria',
	'proveedor',
	'precio',
	'max_52s',
	'min_52s',
	'perf_5d',
	'perf_1m',
	'perf_6m',
	'perf_1y',
	'sharpe',
	'beta',
	'score'
] as const;

export const load: PageServerLoad = async (event) => {
	const res = await tryCall(backend(event).GET('/api/etf/ranking-completo'));
	if (!res.ok) return { tabla: res };
	const filas = res.data.filas.map((f) => Object.fromEntries(CAMPOS.map((k) => [k, f[k]])));
	return {
		tabla: {
			ok: true as const,
			data: {
				fecha: res.data.fecha,
				filas: filas as Pick<(typeof res.data.filas)[number], (typeof CAMPOS)[number]>[]
			}
		}
	};
};
