import { describe, expect, it } from 'vitest';
import { esNumerica, fmtValor, fmtZ } from './format';

describe('fmtValor', () => {
	it('formatea según la unidad', () => {
		expect(fmtValor('pct', 0.1234)).toBe('12,3 %');
		expect(fmtValor('pct_pts', 5.16)).toBe('5,16 %');
		expect(fmtValor('multiplo', 14)).toBe('14,00x');
		expect(fmtValor('numero', 1.5)).toBe('1,50');
		expect(fmtValor('texto', 'CLP')).toBe('CLP');
		expect(fmtValor('monto', 1_983_462_506_496)).toMatch(/^1,98\sB/);
		expect(fmtValor('fecha', 1777248000)).toMatch(/2026/);
	});

	it('vacíos como guion', () => {
		for (const u of ['pct', 'texto', 'fecha', 'multiplo'] as const) {
			expect(fmtValor(u, null)).toBe('—');
		}
	});

	it('clasifica unidades numéricas', () => {
		expect(esNumerica('pct')).toBe(true);
		expect(esNumerica('texto')).toBe(false);
		expect(esNumerica('fecha')).toBe(false);
	});
});

describe('fmtZ', () => {
	it('muestra el signo', () => {
		expect(fmtZ(0.61)).toBe('+0,61');
		expect(fmtZ(-1.2)).toBe('-1,20');
		expect(fmtZ(null)).toBe('—');
	});
});
