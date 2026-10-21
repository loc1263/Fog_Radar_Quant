import { building } from '$app/environment';
import type { Handle, HandleServerError } from '@sveltejs/kit';

const SECURITY_HEADERS: Record<string, string> = {
	'X-Content-Type-Options': 'nosniff',
	'X-Frame-Options': 'DENY',
	'Referrer-Policy': 'strict-origin-when-cross-origin',
	'Permissions-Policy': 'camera=(), microphone=(), geolocation=()'
};

export const handle: Handle = async ({ event, resolve }) => {
	// Detrás de Caddy, getClientAddress() lee ADDRESS_HEADER / XFF_DEPTH (adapter-node).
	// Al construir el sitio estático (fallback SPA) no hay cliente.
	event.locals.clientIp = building ? '' : event.getClientAddress();

	const response = await resolve(event);
	for (const [name, value] of Object.entries(SECURITY_HEADERS)) {
		if (!response.headers.has(name)) response.headers.set(name, value);
	}
	return response;
};

export const handleError: HandleServerError = ({ error, event, status, message }) => {
	if (status !== 404) {
		console.error(`[${status}] ${event.request.method} ${event.url.pathname}`, error);
	}
	return { message: status === 404 ? 'Página no encontrada.' : message, code: 'unexpected' };
};
