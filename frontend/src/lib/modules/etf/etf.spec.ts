/**
 * Lógica del módulo ETF. Portado de los tests del original
 * (`csv.test.ts`, `Top15Table.test.tsx`, `MomentumTable.test.tsx`,
 * `ResumenPanel.test.tsx`) a la lógica pura que ahora vive en `format.ts`.
 */
import { describe, expect, it } from 'vitest';
import { toCsv } from '$lib/table';
import {
	csvColumnas,
	csvName,
	fmtDelta,
	fmtFechaCorrida,
	fmtRend,
	historialConDeltas,
	pesoPorBucket,
	rango52
} from './format';

const FILA = {
	ticker: 'PWV',
	nombre: 'Invesco Large Cap Value',
	categoria: 'Large Value',
	perf_5d: 1.23,
	perf_1m: 4.56,
	perf_6m: 7.89,
	perf_1y: 10.11,
	sharpe: 1.5,
	beta: 0.9,
	score: 85.4,
	score_comp: 70.2
};

describe('formato', () => {
	it('rendimientos en puntos, con signo', () => {
		expect(fmtRend(12.3)).toBe('+12,30 %');
		expect(fmtRend(-1.5)).toBe('-1,50 %');
		expect(fmtRend(null)).toBe('—');
	});
	it('deltas con signo y sin ruido de redondeo', () => {
		expect(fmtDelta(3.44, 1)).toBe('+3,4');
		expect(fmtDelta(-2, 0)).toBe('-2');
		expect(fmtDelta(0.0000001, 3)).toBe('0,000');
		expect(fmtDelta(undefined)).toBe('—');
	});
	it('fecha de la corrida sin conversión de zona', () => {
		expect(fmtFechaCorrida('2026-09-24')).toBe('24/09/2026');
		expect(fmtFechaCorrida(null)).toBe('—');
	});
});

describe('CSV (etfExportColumns del original)', () => {
	it('agrega la fecha de la corrida al nombre base', () => {
		expect(csvName('etf_top15_recomendados', '2026-07-23')).toBe('etf_top15_recomendados_20260723');
		expect(csvName('etf_top15_recomendados', null)).toBe('etf_top15_recomendados');
	});
	it('usa la columna Score por defecto', () => {
		const csv = toCsv([FILA], csvColumnas());
		const [header, fila] = csv.replace(/^﻿/, '').split(/\r?\n/);
		expect(header.split(';').at(-1)).toBe('Score');
		expect(fila).toContain('85.4');
	});
	it('usa Score_comp cuando se pide (tabla de Momentum)', () => {
		const csv = toCsv([FILA], csvColumnas('score_comp', 'Score_comp'));
		const fila = csv.replace(/^﻿/, '').split(/\r?\n/)[1];
		expect(fila).toContain('70.2');
		expect(fila).not.toContain('85.4');
	});
});

describe('rango de 52 semanas', () => {
	it('clasifica cerca del máximo / mínimo / medio', () => {
		expect(rango52({ precio: 95, max_52s: 100, min_52s: 50 })?.etiqueta).toBe('Cerca del máximo');
		expect(rango52({ precio: 55, max_52s: 100, min_52s: 50 })?.etiqueta).toBe('Cerca del mínimo');
		expect(rango52({ precio: 75, max_52s: 100, min_52s: 50 })).toMatchObject({
			pos: 50,
			etiqueta: 'Rango medio'
		});
	});
	it('acota fuera de rango y sin datos devuelve null', () => {
		expect(rango52({ precio: 120, max_52s: 100, min_52s: 50 })?.pos).toBe(100);
		expect(rango52({ precio: 10, max_52s: 50, min_52s: 50 })).toBeNull();
		expect(rango52({ precio: null, max_52s: 100, min_52s: 50 })).toBeNull();
	});
});

describe('historial', () => {
	const base = { universo_depurado: 700, top1_ticker: 'X', top1_score: 80 };
	it('más reciente primero y Δ contra la corrida inmediatamente anterior', () => {
		const h = historialConDeltas([
			{
				...base,
				fecha: '2026-07-10',
				universo_bruto: 1000,
				n_outliers: 10,
				sharpe_promedio: 1.0,
				beta_promedio: 0.5
			},
			{
				...base,
				fecha: '2026-07-17',
				universo_bruto: 1000,
				n_outliers: 20,
				sharpe_promedio: 1.2,
				beta_promedio: 0.4
			}
		]);
		expect(h.map((r) => r.fecha)).toEqual(['2026-07-17', '2026-07-10']);
		expect(h[0].delta_sharpe).toBeCloseTo(0.2);
		expect(h[0].delta_beta).toBeCloseTo(-0.1);
		expect(h[0].pct_outliers).toBe(2);
		expect(h[1].delta_sharpe).toBeNull();
	});
});

describe('Top 15 Cap.', () => {
	it('suma el peso por bucket (fracción 0-1), mayor primero', () => {
		expect(
			pesoPorBucket([
				{ cap_bucket: 'Mid-cap', peso_portfolio: 20 },
				{ cap_bucket: 'Large-cap', peso_portfolio: 30 },
				{ cap_bucket: 'Large-cap', peso_portfolio: 25 },
				{ cap_bucket: null, peso_portfolio: 25 }
			])
		).toEqual([
			{ label: 'Large-cap', value: 0.55 },
			{ label: 'Otros', value: 0.25 },
			{ label: 'Mid-cap', value: 0.2 }
		]);
	});
});
