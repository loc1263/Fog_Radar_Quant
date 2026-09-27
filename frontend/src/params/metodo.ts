import type { ParamMatcher } from '@sveltejs/kit';

/** Métodos de portafolio del módulo EEUU (`/eeuu/portafolio/[metodo=metodo]`). */
export const match = ((param: string): param is 'markowitz' | 'black-litterman' | 'hrp' =>
	param === 'markowitz' || param === 'black-litterman' || param === 'hrp') satisfies ParamMatcher;
