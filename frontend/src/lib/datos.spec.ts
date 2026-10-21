import { describe, expect, it } from 'vitest';
import { rutaConcreta } from './datos';

describe('rutaConcreta', () => {
	it('reemplaza y codifica los parámetros de ruta', () => {
		expect(rutaConcreta('/api/etf/history/ticker/{ticker}', { ticker: 'BRK.B' })).toBe(
			'/api/etf/history/ticker/BRK.B'
		);
		expect(rutaConcreta('/api/x/{a}', { a: 'a b/c' })).toBe('/api/x/a%20b%2Fc');
		expect(rutaConcreta('/api/etf/top15')).toBe('/api/etf/top15');
	});
});
