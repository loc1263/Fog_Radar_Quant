import { describe, expect, it } from 'vitest';
import type { UsCandidateRow } from '$lib/api/types';
import {
	flattenCandidate,
	fmtPrice,
	fmtRet,
	fmtUsd,
	isGrowingPerformance,
	scoreSignal
} from './format';

describe('formato EEUU', () => {
	it('montos USD compactos', () => {
		expect(fmtUsd(1_147_237_548_999)).toBe('$1,15T');
		expect(fmtUsd(262_356_486_507)).toBe('$262,4B');
		expect(fmtUsd(15_200_000)).toBe('$15,2M');
		expect(fmtUsd(null)).toBe('—');
		expect(fmtPrice(1791.82)).toBe('$1.791,82');
		expect(fmtRet(0.1099)).toBe('+10,99 %');
	});

	it('cortes del score iguales a las etiquetas del backend', () => {
		expect(scoreSignal(80.3)).toBe('positive');
		expect(scoreSignal(60)).toBe('positive');
		expect(scoreSignal(45)).toBe('mixed');
		expect(scoreSignal(44.9)).toBe('negative');
		expect(scoreSignal(null)).toBe('neutral');
	});

	it('rendimiento creciente estricto', () => {
		const base = {
			return_1d: 0.01,
			return_5d: 0.02,
			return_3m: 0.05,
			return_6m: 0.1,
			return_1y: 0.3
		};
		expect(isGrowingPerformance(base)).toBe(true);
		expect(isGrowingPerformance({ ...base, return_6m: 0.05 })).toBe(false);
		expect(isGrowingPerformance({ ...base, return_1y: null })).toBe(false);
	});

	it('CSV con todos los campos', () => {
		const row = { ticker: 'AAA', price: 1, momentum_12_1: 0.5 } as UsCandidateRow;
		const flat = flattenCandidate(row);
		expect(Object.keys(flat)).toHaveLength(41);
		expect(flat.ticker).toBe('AAA');
		expect(flat.score_overall).toBeNull();
	});
});
