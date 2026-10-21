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
	/** Parámetros de la URL que se conservan al cambiar de sección (p. ej. `analisis`). */
	keepParams?: string[];
}

export const MODULES: ModuleNav[] = [
	{
		key: 'cl_stocks',
		label: 'Chile',
		href: '/chile',
		description: 'Ranking multi-factor de acciones del IPSA.',
		sections: [
			{ label: 'Ranking', href: '/chile' },
			{ label: 'Indicadores', href: '/chile/indicadores' }
		]
	},
	{
		key: 'us_stocks',
		label: 'EEUU',
		href: '/eeuu',
		description: 'Screener de momentum y salud financiera de large caps de EEUU.',
		sections: [
			{ label: 'Top 15', href: '/eeuu' },
			{ label: 'Candidatos', href: '/eeuu/candidatos' },
			{ label: 'Evaluación', href: '/eeuu/evaluacion' },
			{ label: 'Rendimiento', href: '/eeuu/rendimiento' },
			{ label: 'Portafolio', href: '/eeuu/portafolio' }
		]
	},
	{
		key: 'etf',
		label: 'ETFs',
		href: '/etf',
		description: 'Ranking de ETFs: Top 15, momentum, categorías y rotación.',
		sections: [
			{ label: 'Resumen', href: '/etf' },
			{ label: 'Top 15', href: '/etf/top15' },
			{ label: 'Universo completo', href: '/etf/universo' },
			{ label: 'Excluidos', href: '/etf/excluidos' },
			{ label: 'Rotación', href: '/etf/rotacion' },
			{ label: 'Diversificación', href: '/etf/diversificacion' },
			{ label: 'Categorías', href: '/etf/categorias' },
			{ label: 'Top 15 Perf. + Beta', href: '/etf/top15-simple' },
			{ label: 'Top 15 Vanguard', href: '/etf/top15-vanguard' },
			{ label: 'Top 15 Momentum', href: '/etf/momentum' },
			{ label: 'Top 15 Cap.', href: '/etf/top15-cap' }
		]
	},
	{
		key: 'tendencia',
		label: 'Tendencia',
		href: '/tendencia',
		description: 'Portafolio Markowitz, tendencia a 6 meses y momentum Asia + EEUU.',
		sections: [
			{ label: 'Resumen', href: '/tendencia' },
			{ label: 'Portafolio', href: '/tendencia/portafolio' },
			{ label: 'Tendencia 6m', href: '/tendencia/proyeccion' },
			{ label: 'Sectores', href: '/tendencia/sectores' },
			{ label: 'Momentum', href: '/tendencia/momentum' },
			{ label: 'Limitaciones', href: '/tendencia/limitaciones' }
		],
		keepParams: ['analisis']
	}
];

export const EXTRA_NAV: NavItem[] = [{ label: 'Estado', href: '/estado' }];

/** ¿`pathname` está dentro de `href`? (`/` solo coincide exacto). */
export function isActive(pathname: string, href: string): boolean {
	if (href === '/') return pathname === '/';
	return pathname === href || pathname.startsWith(`${href}/`);
}

/** `?a=1` con los parámetros de `url` que la sección conserva (o "" si ninguno). */
export function keptQuery(url: URL, keep: string[] | undefined): string {
	if (!keep?.length) return '';
	const params = new URLSearchParams();
	for (const k of keep) {
		const v = url.searchParams.get(k);
		if (v) params.set(k, v);
	}
	const q = params.toString();
	return q ? `?${q}` : '';
}
