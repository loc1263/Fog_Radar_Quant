import type { PageLoad } from './$types';
import { datos } from '$lib/datos';

/** Campos que usa la tabla: se descartan el resto para no retener el universo completo (~700 ETFs) en memoria. */
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

export const load: PageLoad = async ({ fetch }) => {
	const res = await datos(fetch, '/api/etf/ranking-completo');
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
