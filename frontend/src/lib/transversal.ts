/** Lógica del hub (sin Svelte: se testea con Vitest). */
import type { Frescura, Unidad } from '$lib/api/types';
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
