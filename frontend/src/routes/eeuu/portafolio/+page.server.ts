import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = () => {
	redirect(307, resolve('/eeuu/portafolio/[metodo=metodo]', { metodo: 'markowitz' }));
};
