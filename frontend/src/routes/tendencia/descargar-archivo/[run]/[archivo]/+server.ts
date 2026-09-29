/**
 * Descarga de un archivo de una corrida de Tendencia (PDF, CSV…) por el BFF:
 * reemplaza el servidor estático del original sobre backend/output.
 */
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { backend } from '$lib/server/api';

// Mismas reglas que el backend (run_files.RUN_DIR_PATTERN / NOMBRE_ARCHIVO): se validan
// antes de armar la URL (fetch normalizaría `..`) y para no reflejar texto arbitrario.
const RUN = /^\d{8}_\d{2}$/;
const ARCHIVO = /^[A-Za-z0-9][A-Za-z0-9_-]{0,126}\.[A-Za-z0-9]{1,5}$/;

export const GET: RequestHandler = async (event) => {
	const { run, archivo } = event.params;
	if (!RUN.test(run) || !ARCHIVO.test(archivo)) {
		error(404, { message: 'Archivo no disponible.', code: 'not_found' });
	}
	let res: Response;
	try {
		const r = await backend(event, { timeoutMs: 60_000 }).GET(
			'/api/tendencia/runs/{run_dir}/files/{nombre}',
			{ params: { path: { run_dir: run, nombre: archivo } }, parseAs: 'stream' }
		);
		res = r.response;
	} catch {
		error(503, { message: 'No se pudo conectar con el backend.', code: 'backend_unreachable' });
	}
	if (!res.ok || !res.body) {
		const body = (await res.json().catch(() => ({}))) as { detail?: string; code?: string };
		error(res.status === 401 ? 502 : res.status, {
			message: body.detail ?? 'No se pudo descargar el archivo.',
			code: body.code ?? 'backend_error'
		});
	}
	return new Response(res.body, {
		headers: {
			'content-type': res.headers.get('content-type') ?? 'application/octet-stream',
			'content-disposition': `attachment; filename="${archivo}"`,
			'cache-control': 'no-store'
		}
	});
};
