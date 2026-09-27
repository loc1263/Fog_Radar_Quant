/** Menú principal y sub-navegación de cada módulo (una ruta por ítem, §4). */
import type { Pathname } from '$app/types';

export interface NavItem {
	label: string;
	href: Pathname;
}

export interface ModuleNav extends NavItem {
	/** Clave del módulo en el backend (`/api/status`). */
	key: 'cl_stocks' | 'us_stocks' | 'etf' | 'tendencia';
	description: string;
	/** Secciones del módulo (se completan al migrar cada uno). */
	sections: NavItem[];
}

export const MODULES: ModuleNav[] = [
	{
		key: 'cl_stocks',
		label: 'Chile',
		href: '/chile',
		description: 'Ranking multi-factor de acciones del IPSA.',
		sections: []
	},
	{
		key: 'us_stocks',
		label: 'EEUU',
		href: '/eeuu',
		description: 'Screener de momentum y salud financiera de large caps de EEUU.',
		sections: []
	},
	{
		key: 'etf',
		label: 'ETFs',
		href: '/etf',
		description: 'Ranking de ETFs: Top 15, momentum, categorías y rotación.',
		sections: []
	},
	{
		key: 'tendencia',
		label: 'Tendencia',
		href: '/tendencia',
		description: 'Portafolio Markowitz, tendencia a 6 meses y momentum Asia + EEUU.',
		sections: []
	}
];

export const EXTRA_NAV: NavItem[] = [{ label: 'Estado', href: '/estado' }];

/** ¿`pathname` está dentro de `href`? (`/` solo coincide exacto). */
export function isActive(pathname: string, href: string): boolean {
	if (href === '/') return pathname === '/';
	return pathname === href || pathname.startsWith(`${href}/`);
}
