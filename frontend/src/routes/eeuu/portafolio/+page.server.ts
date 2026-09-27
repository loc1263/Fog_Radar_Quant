import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = () => {
	redirect(307, '/eeuu/portafolio/markowitz');
};
