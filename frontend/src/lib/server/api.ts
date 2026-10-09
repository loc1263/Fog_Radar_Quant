/**
 * Cliente del backend — SOLO servidor (patrón BFF).
 *
 * El navegador nunca llama al backend: las páginas cargan datos en
 * `+page.server.ts` / `+layout.server.ts` con este cliente, que agrega la
 * API key y la IP del usuario final. Al estar en `$lib/server`, SvelteKit
 * impide importarlo desde código del cliente.
 */
import { error, type RequestEvent } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import createClient from 'openapi-fetch';
import type { paths } from '$lib/api/types';

const DEFAULT_TIMEOUT_MS = 15_000;

export function backendUrl(): string {
	return (env.PRIVATE_API_URL ?? 'http://localhost:8000').replace(/\/+$/, '');
}

type ClientEvent = Pick<RequestEvent, 'fetch' | 'locals'>;

/** Cliente tipado por ruta (`api.GET('/api/status')`) para un request concreto. */
export function backend(event: ClientEvent, { timeoutMs = DEFAULT_TIMEOUT_MS } = {}) {
	const headers: Record<string, string> = { 'X-Forwarded-For': event.locals.clientIp };
	if (env.PRIVATE_API_KEY) headers['X-API-Key'] = env.PRIVATE_API_KEY;

	return createClient<paths>({
		baseUrl: backendUrl(),
		headers,
		fetch: (req: Request) => event.fetch(req, { signal: AbortSignal.timeout(timeoutMs) })
	});
}

type ApiResult<T> = { data?: T; error?: unknown; response: Response };

/**
 * Devuelve `data` o lanza el error de SvelteKit correspondiente, conservando
 * el `code` del backend para que la página de error pueda explicarlo.
 */
export function unwrap<T>(result: ApiResult<T>): T {
	if (result.data !== undefined && result.response.ok) return result.data;
	// Un 2xx sin cuerpo no es un error HTTP válido para `error()` (exige 400-599).
	const status = result.response.ok ? 502 : result.response.status || 502;
	const body = (result.error ?? {}) as { detail?: unknown; code?: unknown };
	const message =
		typeof body.detail === 'string'
			? body.detail
			: result.response.ok
				? 'Respuesta vacía del backend.'
				: `Error ${status} del backend`;
	const code = typeof body.code === 'string' ? body.code : 'backend_error';
	// 401 del backend = la API key del servidor está mal configurada: no es culpa del usuario.
	if (status === 401)
		error(502, { message: 'El sitio no pudo autenticarse con el backend.', code });
	error(status, { message, code });
}

/** Ejecuta la llamada y convierte fallas de red/timeout en un 503 legible. */
export async function call<T>(promise: Promise<ApiResult<T>>): Promise<T> {
	let result: ApiResult<T>;
	try {
		result = await promise;
	} catch (e) {
		const timeout = e instanceof DOMException && e.name === 'TimeoutError';
		error(503, {
			message: timeout
				? 'El backend tardó demasiado en responder.'
				: 'No se pudo conectar con el backend.',
			code: timeout ? 'backend_timeout' : 'backend_unreachable'
		});
	}
	return unwrap(result);
}
