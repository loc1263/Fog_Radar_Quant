/**
 * Recálculo del Score Compuesto con pesos personalizados, en el navegador.
 *
 * Port de `Stock_Choice_Nacional/.../lib/scoring.ts`, que a su vez replica
 * el cálculo de scores del backend (`internal/modules/cl/scoring.go`): un pilar
 * sin dato no aporta ni al numerador ni al denominador (no se trata como 0).
 * Los pesos por defecto vienen del backend (`modelo.pilares[].peso`).
 */
import type { ClPilarInfo, ClPilarKey, ClRankingRow } from '$lib/api/types';

export type Pesos = Record<ClPilarKey, number>;

export const PILAR_KEYS: readonly ClPilarKey[] = [
	'valor',
	'calidad',
	'crecimiento',
	'salud_financiera',
	'dividendo',
	'momentum'
];

export const PESO_MIN = 0;
export const PESO_MAX = 3;
export const PESO_PASO = 0.25;

function isNum(v: unknown): v is number {
	return typeof v === 'number' && Number.isFinite(v);
}

export function pesosDesdeModelo(pilares: readonly ClPilarInfo[]): Pesos {
	const pesos = Object.fromEntries(PILAR_KEYS.map((k) => [k, 1])) as Pesos;
	for (const p of pilares) pesos[p.key] = p.peso;
	return pesos;
}

export function pesosIguales(a: Pesos, b: Pesos): boolean {
	return PILAR_KEYS.every((k) => a[k] === b[k]);
}

/** Participación de cada pilar en el total de pesos (0-100). */
export function participacion(pesos: Pesos): Pesos {
	const total = PILAR_KEYS.reduce((acc, k) => acc + pesos[k], 0);
	return Object.fromEntries(
		PILAR_KEYS.map((k) => [k, total > 0 ? (pesos[k] / total) * 100 : 0])
	) as Pesos;
}

/** Promedio ponderado de los pilares disponibles de la fila; null si no hay ninguno. */
export function scorePersonalizado(
	fila: Pick<ClRankingRow, 'pilares'>,
	pesos: Pesos
): number | null {
	let suma = 0;
	let sumaPesos = 0;
	for (const k of PILAR_KEYS) {
		const valor = fila.pilares[k];
		if (!isNum(valor)) continue;
		suma += valor * pesos[k];
		sumaPesos += pesos[k];
	}
	return sumaPesos > 0 ? suma / sumaPesos : null;
}

/**
 * Recalcula `score_compuesto` y `ranking` de todas las filas, ordenadas de
 * mayor a menor. Empates comparten puesto (ranking con empates en el mínimo);
 * las filas sin score quedan al final con ranking null.
 */
export function recalcularRanking<T extends ClRankingRow>(filas: readonly T[], pesos: Pesos): T[] {
	const conScore = filas.map((f) => ({ ...f, score_compuesto: scorePersonalizado(f, pesos) }));
	conScore.sort((a, b) => {
		const sa = a.score_compuesto;
		const sb = b.score_compuesto;
		if (sa === null && sb === null) return 0;
		if (sa === null) return 1;
		if (sb === null) return -1;
		return sb - sa;
	});
	let puesto = 0;
	let anterior: number | null = null;
	return conScore.map((f, i) => {
		const s = f.score_compuesto;
		if (s === null) return { ...f, ranking: null };
		if (anterior === null || s < anterior) {
			puesto = i + 1;
			anterior = s;
		}
		return { ...f, ranking: puesto };
	});
}
