import { describe, expect, it } from 'vitest';
import { fmtEdad, fmtValor } from './transversal';

describe('transversal', () => {
	it('formatea el valor según la unidad', () => {
		expect(fmtValor(0.1234, 'fraccion')).toBe('12,3 %');
		expect(fmtValor(0.61, 'numero')).toBe('0,61');
		expect(fmtValor(null, 'numero')).toBe('—');
	});

	it('edad legible', () => {
		expect(fmtEdad(0.2)).toBe('hace menos de 1 h');
		expect(fmtEdad(15.9)).toBe('hace 16 h');
		expect(fmtEdad(91.6)).toBe('hace 4 días');
		expect(fmtEdad(null)).toBe('—');
	});
});
