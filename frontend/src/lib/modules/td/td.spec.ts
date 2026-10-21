/** Lógica del módulo Tendencia contra la corrida real 20260926_01 (dashboard_data.json). */
import { describe, expect, it } from 'vitest';
import type { TdDashboard } from '$lib/api/types';
import sample from './fixtures/dashboard.sample.json';
import {
	analisisSlug,
	elegirAnalisis,
	elegirPortafolio,
	pctPts,
	pesoPorPais,
	senal,
	separarTickers
} from './format';

// El JSON archivado no trae los campos que agrega la API (archivos, parámetros).
const d = { ...sample, archivos: [], parametros: null } as unknown as TdDashboard;

describe('análisis', () => {
	it('slug en la URL y selección con respaldo al principal', () => {
		expect(analisisSlug('_us')).toBe('us');
		expect(analisisSlug('')).toBe('');
		expect(elegirAnalisis(d, 'us')?.label).toBe('solo EEUU');
		expect(elegirAnalisis(d, null)?.key).toBe('');
		expect(elegirAnalisis(d, 'asia')?.key).toBe('');
	});
});

describe('portafolio', () => {
	const a = elegirAnalisis(d, null)!;
	it('por defecto 10 acciones, re-optimizadas al 100 %', () => {
		const p = elegirPortafolio(a, null);
		expect(p.n).toBe(10);
		expect(p.completo).toBe(false);
		expect(p.holdings).toHaveLength(10);
		expect(p.holdings.reduce((s, h) => s + h.peso_final, 0)).toBeCloseTo(1, 3);
	});
	it('15 = portafolio completo con la performance del análisis', () => {
		const p = elegirPortafolio(a, 15);
		expect(p.completo).toBe(true);
		expect(p.sharpe).toBe(a.performance.portafolio_15_finales.sharpe_ratio);
	});
	it('corridas antiguas sin opciones muestran el completo', () => {
		expect(elegirPortafolio({ ...a, portfolio_options: [] }, 5).completo).toBe(true);
	});
	it('peso por país suma 1 y va de mayor a menor', () => {
		const paises = pesoPorPais(a.holdings);
		expect(paises.reduce((s, x) => s + x.value, 0)).toBeCloseTo(1, 3);
		expect(paises.map((x) => x.value)).toEqual(
			[...paises.map((x) => x.value)].sort((x, y) => y - x)
		);
	});
});

describe('formato', () => {
	it('señales técnicas → variante del badge', () => {
		expect(senal('Alcista')).toBe('positive');
		expect(senal('Bajista')).toBe('negative');
		expect(senal('Lateral / mixta')).toBe('mixed');
		expect(senal(null)).toBe('neutral');
	});
	it('porcentajes en puntos y tickers', () => {
		expect(pctPts(17.63, 1, true)).toBe('+17,6 %');
		expect(separarTickers('AAA, BBB,CCC')).toEqual(['AAA', 'BBB', 'CCC']);
	});
});
