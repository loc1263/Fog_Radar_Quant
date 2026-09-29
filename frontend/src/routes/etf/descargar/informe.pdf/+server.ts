/**
 * Descarga del PDF ejecutivo (BFF): el navegador pide el PDF a SvelteKit y
 * SvelteKit lo trae del backend con la API key. Se reenvía el cuerpo tal cual.
 */
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { backend } from '$lib/server/api';

export const GET: RequestHandler = async (event) => {
	let res: Response;
	try {
		const r = await backend(event, { timeoutMs: 60_000 }).GET('/api/etf/pdf', {
			parseAs: 'stream'
		});
		res = r.response;
	} catch {
		error(503, { message: 'No se pudo conectar con el backend.', code: 'backend_unreachable' });
	}
	if (!res.ok || !res.body) {
		const body = (await res.json().catch(() => ({}))) as { detail?: string; code?: string };
		error(res.status === 401 ? 502 : res.status, {
			message: body.detail ?? 'No se pudo generar el PDF.',
			code: body.code ?? 'backend_error'
		});
	}
	return new Response(res.body, {
		headers: {
			'content-type': 'application/pdf',
			'content-disposition':
				res.headers.get('content-disposition') ?? 'attachment; filename="etf_informe.pdf"',
			'cache-control': 'no-store'
		}
	});
};
