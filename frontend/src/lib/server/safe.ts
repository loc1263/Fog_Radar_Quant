/**
 * Variante "tolerante" de `call`: en vez de lanzar, devuelve el error para
 * que la página se renderice igual y muestre un ErrorState en su lugar
 * (útil en el hub, donde una falla del backend no debe tumbar el sitio).
 */
import { isHttpError } from '@sveltejs/kit';
import { call } from './api';

import type { Loaded } from '$lib/api/types';

export type { Loaded };

type ApiResult<T> = Parameters<typeof call<T>>[0];

export async function tryCall<T>(promise: ApiResult<T>): Promise<Loaded<T>> {
	try {
		return { ok: true, data: await call(promise) };
	} catch (e) {
		if (isHttpError(e)) {
			return {
				ok: false,
				message: e.body.message,
				code: e.body.code ?? `http_${e.status}`,
				status: e.status
			};
		}
		throw e;
	}
}
