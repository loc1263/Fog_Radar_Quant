import { describe, expect, it } from 'vitest';
import { destinoResultado, fmtEdad, fmtPosicion, fmtValor, limpiarConsulta } from './transversal';

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

	it('lleva cada resultado a la vista de su módulo', () => {
		expect(destinoResultado({ modulo: 'cl_stocks', ticker: 'SQM-B.SN' })).toEqual({
			path: '/chile',
			query: '?q=SQM-B.SN'
		});
		expect(destinoResultado({ modulo: 'tendencia', ticker: '000660.KS' })?.query).toBe(
			'?ticker=000660.KS'
		);
		expect(destinoResultado({ modulo: 'etf', ticker: 'A&B' })?.query).toBe('?q=A%26B');
		expect(destinoResultado({ modulo: 'otro', ticker: 'X' })).toBeNull();
	});

	it('posición y consulta', () => {
		expect(fmtPosicion(3, 60)).toBe('3 de 60');
		expect(fmtPosicion(2, null)).toBe('#2');
		expect(fmtPosicion(null, 5)).toBe('—');
		expect(limpiarConsulta('  mu  ')).toBe('mu');
		expect(limpiarConsulta('x'.repeat(50))).toHaveLength(40);
	});
});
