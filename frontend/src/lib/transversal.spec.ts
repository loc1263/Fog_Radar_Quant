import { describe, expect, it } from 'vitest';
import {
	buscarEnIndice,
	destinoResultado,
	fmtEdad,
	fmtPosicion,
	fmtValor,
	limpiarConsulta
} from './transversal';

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

describe('buscarEnIndice', () => {
	const e = (
		ticker: string,
		nombre: string | null,
		modulo: string,
		orden: number,
		posicion: number | null
	) => ({
		ticker,
		nombre,
		modulo,
		orden,
		posicion,
		titulo_modulo: modulo,
		total: null,
		valor: null,
		metrica: 'Score',
		unidad: 'numero' as const,
		contexto: null
	});
	const indice = [
		e('VAPORES.SN', 'Compañía Sud Americana de Vapores', 'cl_stocks', 0, 5),
		e('VAP', 'Vanguard Aperture', 'etf', 3, 2),
		e('SPY', 'SPDR S&P 500', 'etf', 3, 1),
		e('SPYG', 'SPDR Growth', 'etf', 3, 9)
	];

	it('ticker exacto o sin sufijo de bolsa va primero, luego prefijo, luego nombre', () => {
		const r = buscarEnIndice(indice, 'vapores');
		expect(r.resultados.map((x) => x.ticker)).toEqual(['VAPORES.SN']);
		expect(r.resultados[0].exacto).toBe(true);
		const s = buscarEnIndice(indice, 'spy').resultados;
		expect(s.map((x) => x.ticker)).toEqual(['SPY', 'SPYG']);
		expect(s.map((x) => x.exacto)).toEqual([true, false]);
	});
	it('busca por nombre sin tildes desde 3 letras y respeta el límite', () => {
		expect(buscarEnIndice(indice, 'compania').resultados).toHaveLength(1);
		expect(buscarEnIndice(indice, 'co').resultados).toHaveLength(0);
		expect(buscarEnIndice(indice, 'sp', 1).resultados).toHaveLength(1);
	});
	it('consulta vacía no devuelve nada', () => {
		expect(buscarEnIndice(indice, '  ').resultados).toEqual([]);
	});
});
