/**
 * Datos del sitio estático: `fogradar export` deja la respuesta de cada endpoint GET de la API
 * en `static/data/<ruta>.json`. Las páginas la leen desde el navegador (o el prerender) con `datos`,
 * sin backend ni API key. Las llamadas llevan el mismo tipo que la API (`paths`).
 */
import { asset } from '$app/paths';
import type { Loaded, paths } from '$lib/api/types';

type RutaGet = {
	[P in keyof paths]: paths[P] extends { get: { responses: { 200: unknown } } } ? P : never;
}[keyof paths];

type Respuesta<P extends RutaGet> = paths[P] extends {
	get: { responses: { 200: { content: { 'application/json': infer T } } } };
}
	? T
	: never;

/** `/api/etf/history/ticker/{ticker}` + `{ ticker: 'SPY' }` → `/api/etf/history/ticker/SPY`. */
export function rutaConcreta(ruta: string, params: Record<string, string> = {}): string {
	return ruta.replace(/\{(\w+)\}/g, (_, k: string) => encodeURIComponent(params[k] ?? ''));
}

/** Lee `static/data<archivo>.json`; `archivo` es la ruta de la API, concreta o con un sufijo propio del export. */
async function leer<T>(fetch: typeof globalThis.fetch, archivo: string): Promise<Loaded<T>> {
	const url = asset(`/data${archivo}.json` as Parameters<typeof asset>[0]);
	try {
		const res = await fetch(url);
		if (res.ok) return { ok: true, data: (await res.json()) as T };
		const sinDatos = res.status === 404;
		return {
			ok: false,
			status: res.status,
			code: sinDatos ? 'sin_datos' : `http_${res.status}`,
			message: sinDatos
				? 'Aún no hay datos publicados para esta vista.'
				: `No se pudieron cargar los datos (HTTP ${res.status}).`
		};
	} catch {
		return {
			ok: false,
			code: 'sin_conexion',
			message: 'No se pudieron cargar los datos del sitio.'
		};
	}
}

export function datos<P extends RutaGet>(
	fetch: typeof globalThis.fetch,
	ruta: P,
	params?: Record<string, string>
): Promise<Loaded<Respuesta<P>>> {
	return leer(fetch, rutaConcreta(ruta, params));
}

/** Diff de una corrida contra la anterior (`fogradar export` no guarda pares arbitrarios). */
export function datosDiffEtf(
	fetch: typeof globalThis.fetch,
	desde: string,
	hasta: string
): Promise<Loaded<Respuesta<'/api/etf/history/diff'>>> {
	return leer(
		fetch,
		`/api/etf/history/diff/${encodeURIComponent(desde)}__${encodeURIComponent(hasta)}`
	);
}
