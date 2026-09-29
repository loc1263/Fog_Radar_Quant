/** Lógica del hub y del buscador global (sin Svelte: se testea con Vitest). */
import type { Frescura, ResultadoBusqueda, Unidad } from '$lib/api/types';
import { fmtNumber, fmtPct } from '$lib/format';

type Signal = 'positive' | 'negative' | 'mixed' | 'neutral';

export const FRESCURA: Record<Frescura, { signal: Signal; label: string }> = {
	al_dia: { signal: 'positive', label: 'Datos al día' },
	atrasado: { signal: 'mixed', label: 'Datos atrasados' },
	sin_datos: { signal: 'neutral', label: 'Sin datos' },
	error: { signal: 'negative', label: 'Datos ilegibles' }
};

/** Valor de una métrica según su unidad (`fraccion` → porcentaje). */
export function fmtValor(valor: number | null | undefined, unidad: Unidad): string {
	return unidad === 'fraccion' ? fmtPct(valor, { digits: 1 }) : fmtNumber(valor, 2);
}

/** Antigüedad legible: "hace 3 h", "hace 2 días". */
export function fmtEdad(horas: number | null | undefined): string {
	if (horas === null || horas === undefined || !Number.isFinite(horas)) return '—';
	if (horas < 1) return 'hace menos de 1 h';
	if (horas < 48) return `hace ${Math.round(horas)} h`;
	return `hace ${Math.round(horas / 24)} días`;
}

/**
 * Página del sitio donde se ve un resultado del buscador (ruta + query), o `null`
 * si el módulo no tiene una vista para ese instrumento.
 */
export function destinoResultado(r: Pick<ResultadoBusqueda, 'modulo' | 'ticker'>): {
	path: '/chile' | '/eeuu/candidatos' | '/etf/universo' | '/tendencia/historial';
	query: string;
} | null {
	const q = encodeURIComponent(r.ticker);
	switch (r.modulo) {
		case 'cl_stocks':
			return { path: '/chile', query: `?q=${q}` };
		case 'us_stocks':
			return { path: '/eeuu/candidatos', query: `?q=${q}` };
		case 'etf':
			return { path: '/etf/universo', query: `?q=${q}` };
		case 'tendencia':
			return { path: '/tendencia/historial', query: `?ticker=${q}` };
		default:
			return null;
	}
}

/** Posición "3 de 60" (o solo "#3" si no hay total). */
export function fmtPosicion(posicion: number | null | undefined, total: number | null | undefined) {
	if (!posicion) return '—';
	return total ? `${posicion} de ${total}` : `#${posicion}`;
}

/** Consulta normalizada del buscador (la API acepta 1 a 40 caracteres). */
export function limpiarConsulta(q: string | null | undefined): string {
	return (q ?? '').trim().slice(0, 40);
}
