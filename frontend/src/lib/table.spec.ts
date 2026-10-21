import { describe, expect, it } from 'vitest';
import { nextSort, sortRows, toCsv, type Column } from './table';

const rows = [
	{ ticker: 'SQM-B', nombre: 'Soquimich', score: 1.2 },
	{ ticker: 'CHILE', nombre: 'Banco de Chile', score: null },
	{ ticker: 'COPEC', nombre: 'Empresas Copec', score: -0.4 },
	{ ticker: 'BSANTANDER', nombre: 'Santander Chile', score: 2.5 }
];

describe('sortRows', () => {
	it('ordena números desc y deja vacíos al final', () => {
		const out = sortRows(rows, { key: 'score', dir: 'desc' }).map((r) => r.ticker);
		expect(out).toEqual(['BSANTANDER', 'SQM-B', 'COPEC', 'CHILE']);
	});
	it('vacíos al final también en asc', () => {
		const out = sortRows(rows, { key: 'score', dir: 'asc' }).map((r) => r.ticker);
		expect(out).toEqual(['COPEC', 'SQM-B', 'BSANTANDER', 'CHILE']);
	});
	it('texto con locale es', () => {
		const out = sortRows(rows, { key: 'nombre', dir: 'asc' }).map((r) => r.ticker);
		expect(out[0]).toBe('CHILE');
	});
	it('sin orden devuelve copia en el orden original', () => {
		const out = sortRows(rows, null);
		expect(out).toEqual(rows);
		expect(out).not.toBe(rows);
	});
});

describe('nextSort', () => {
	it('números empiezan desc, texto asc, tercer clic quita el orden', () => {
		let s = nextSort(null, 'score', true);
		expect(s).toEqual({ key: 'score', dir: 'desc' });
		s = nextSort(s, 'score', true);
		expect(s).toEqual({ key: 'score', dir: 'asc' });
		expect(nextSort(s, 'score', true)).toBeNull();
		expect(nextSort(s, 'nombre', false)).toEqual({ key: 'nombre', dir: 'asc' });
	});
});

describe('toCsv', () => {
	it('usa ; con BOM y escapa comillas', () => {
		const cols: Column[] = [
			{ key: 'ticker', label: 'Ticker' },
			{ key: 'nombre', label: 'Nombre' },
			{ key: 'score', label: 'Score', csv: (v) => (typeof v === 'number' ? v.toFixed(2) : '') }
		];
		const csv = toCsv([{ ticker: 'X', nombre: 'Uno; "dos"', score: 1 }], cols);
		expect(csv.startsWith('﻿Ticker;Nombre;Score\r\n')).toBe(true);
		expect(csv).toContain('X;"Uno; ""dos""";1.00');
	});
});

describe('csvCell (inyección CSV)', () => {
	it('neutraliza textos que Excel ejecutaría como fórmula', async () => {
		const { csvCell } = await import('./table');
		expect(csvCell('=HYPERLINK("http://x","ok")')).toBe(`"'=HYPERLINK(""http://x"",""ok"")"`);
		expect(csvCell('+1')).toBe("'+1");
		expect(csvCell('@SUM(A1)')).toBe("'@SUM(A1)");
		expect(csvCell('-3 %')).toBe("'-3 %");
		expect(csvCell(-3.5)).toBe('-3.5'); // número: se exporta como número
		expect(csvCell('AAPL')).toBe('AAPL');
	});
});
