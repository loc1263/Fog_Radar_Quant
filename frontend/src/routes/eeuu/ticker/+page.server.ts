import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const TICKER_RE = /^[A-Z0-9.\-^=]{1,15}$/;

/** `GET /eeuu/ticker?t=aapl` (buscador) → `/eeuu/ticker/AAPL`. */
export const load: PageServerLoad = ({ url }) => {
	const t = (url.searchParams.get('t') ?? '').trim().toUpperCase();
	if (TICKER_RE.test(t)) redirect(303, `/eeuu/ticker/${encodeURIComponent(t)}`);
	return { invalid: url.searchParams.has('t') ? t : null };
};
