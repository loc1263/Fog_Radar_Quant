import { describe, expect, it } from 'vitest';
import { elegir, fmtRunId, fmtRunKey, fmtTs, parDeCorridas, resumenDiff } from './historial';

describe('historial', () => {
	it('resumenDiff cuenta entradas, salidas y movimientos', () => {
		expect(
			resumenDiff([
				{ estado: 'nuevo', delta_rank: null },
				{ estado: 'salio', delta_rank: null },
				{ estado: 'sigue', delta_rank: 3 },
				{ estado: 'sigue', delta_rank: -1 },
				{ estado: 'sigue', delta_rank: 0 },
				{ estado: 'fuera', delta_rank: 2 }
			])
		).toEqual({ nuevos: 1, salidas: 1, siguen: 3, suben: 1, bajan: 1 });
	});

	it('elegir solo acepta opciones válidas', () => {
		expect(elegir('b', ['a', 'b'], 'a')).toBe('b');
		expect(elegir('zzz', ['a', 'b'], 'a')).toBe('a');
		expect(elegir(null, ['a'], null)).toBeNull();
	});

	it('parDeCorridas usa la URL o las dos últimas', () => {
		const k = ['1', '2', '3'];
		expect(parDeCorridas(k, null, null)).toEqual({ desde: '2', hasta: '3' });
		expect(parDeCorridas(k, '1', '3')).toEqual({ desde: '1', hasta: '3' });
		expect(parDeCorridas(k, '3', '3')).toEqual({ desde: '2', hasta: '3' });
		expect(parDeCorridas(k, 'x', '3')).toEqual({ desde: '2', hasta: '3' });
		expect(parDeCorridas(['1'], null, null)).toBeNull();
	});

	it('formatea las claves de corrida de cada módulo', () => {
		expect(fmtTs('20260929_193618')).toBe('29-09-2026 19:36');
		expect(fmtRunId('2026-09-29T22-39-31Z')).toBe('29-09-2026 22:39 UTC');
		expect(fmtRunKey('20260917_2')).toBe('17-09-2026 (2)');
		expect(fmtRunKey('20260917')).toBe('17-09-2026');
	});
});
