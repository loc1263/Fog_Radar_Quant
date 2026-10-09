import { describe, expect, it } from 'vitest';
import type { ClRankingResponse, ClRankingRow } from '$lib/api/types';
import sample from './fixtures/ranking.sample.json';
import {
	participacion,
	pesosDesdeModelo,
	pesosIguales,
	recalcularRanking,
	scorePersonalizado,
	type Pesos
} from './scoring';

const data = sample as unknown as ClRankingResponse;
const DEFAULT = pesosDesdeModelo(data.modelo.pilares);

function fila(simbolo: string, pilares: Partial<ClRankingRow['pilares']>): ClRankingRow {
	return {
		...data.filas[0],
		simbolo,
		pilares: {
			valor: null,
			calidad: null,
			crecimiento: null,
			salud_financiera: null,
			dividendo: null,
			momentum: null,
			...pilares
		}
	};
}

describe('pesos', () => {
	it('toma los pesos por defecto del modelo del backend', () => {
		expect(DEFAULT).toEqual({
			valor: 0.5,
			calidad: 1,
			crecimiento: 1,
			salud_financiera: 1,
			dividendo: 0.25,
			momentum: 2
		});
	});

	it('participación suma 100 y reacciona a los demás pilares', () => {
		const p = participacion(DEFAULT);
		expect(Object.values(p).reduce((a, b) => a + b, 0)).toBeCloseTo(100);
		expect(p.momentum).toBeCloseTo((2 / 5.75) * 100);
		const cero = participacion({
			...DEFAULT,
			valor: 0,
			calidad: 0,
			crecimiento: 0,
			salud_financiera: 0,
			dividendo: 0,
			momentum: 0
		});
		expect(cero.momentum).toBe(0);
	});

	it('compara pesos', () => {
		expect(pesosIguales(DEFAULT, { ...DEFAULT })).toBe(true);
		expect(pesosIguales(DEFAULT, { ...DEFAULT, valor: 1 })).toBe(false);
	});
});

describe('scorePersonalizado', () => {
	it('ignora pilares sin dato (no los trata como 0)', () => {
		const pesos: Pesos = { ...DEFAULT, momentum: 2, calidad: 1 };
		expect(scorePersonalizado(fila('A', { momentum: 1 }), pesos)).toBeCloseTo(1);
		expect(scorePersonalizado(fila('A', { momentum: 1, calidad: -1 }), pesos)).toBeCloseTo(1 / 3);
	});

	it('null si no hay pilares o todos los pesos disponibles son 0', () => {
		expect(scorePersonalizado(fila('A', {}), DEFAULT)).toBeNull();
		expect(scorePersonalizado(fila('A', { valor: 1 }), { ...DEFAULT, valor: 0 })).toBeNull();
	});
});

describe('recalcularRanking', () => {
	it('empates comparten puesto y sin score queda al final con ranking null', () => {
		const filas = [
			fila('C', { momentum: 0.1 }),
			fila('SIN', {}),
			fila('A', { momentum: 0.5 }),
			fila('B', { momentum: 0.5 })
		];
		const r = recalcularRanking(filas, DEFAULT);
		expect(r.map((f) => [f.simbolo, f.ranking])).toEqual([
			['A', 1],
			['B', 1],
			['C', 3],
			['SIN', null]
		]);
	});

	it('no modifica las filas originales', () => {
		const filas = [fila('A', { momentum: 1 })];
		recalcularRanking(filas, DEFAULT);
		expect(filas[0].score_compuesto).toBe(data.filas[0].score_compuesto);
	});

	it('con los pesos por defecto reproduce el ranking del backend', () => {
		const r = recalcularRanking(data.filas, DEFAULT);
		expect(r.map((f) => f.simbolo)).toEqual(data.filas.map((f) => f.simbolo));
		expect(r.map((f) => f.ranking)).toEqual(data.filas.map((f) => f.ranking));
		r.forEach((f, i) => {
			// los pilares vienen redondeados a 4 decimales en el CSV
			expect(f.score_compuesto).toBeCloseTo(data.filas[i].score_compuesto as number, 3);
		});
	});

	it('más peso a Valor cambia el primer lugar', () => {
		const soloValor: Pesos = {
			...DEFAULT,
			valor: 3,
			momentum: 0,
			calidad: 0,
			crecimiento: 0,
			salud_financiera: 0,
			dividendo: 0
		};
		const r = recalcularRanking(data.filas, soloValor);
		const mejorValor = [...data.filas]
			.filter((f) => f.pilares.valor !== null)
			.sort((a, b) => (b.pilares.valor as number) - (a.pilares.valor as number))[0];
		expect(r[0].simbolo).toBe(mejorValor.simbolo);
	});
});
