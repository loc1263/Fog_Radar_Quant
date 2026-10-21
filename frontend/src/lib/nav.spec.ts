import { describe, expect, it } from 'vitest';
import { isActive, MODULES, sinBase } from './nav';

describe('nav', () => {
	it('isActive respeta los límites de segmento', () => {
		expect(isActive('/etf/top15', '/etf')).toBe(true);
		expect(isActive('/etf', '/etf')).toBe(true);
		expect(isActive('/etfx', '/etf')).toBe(false);
		expect(isActive('/chile', '/')).toBe(false);
		expect(isActive('/', '/')).toBe(true);
	});
	it('los 4 módulos del backend están en el menú', () => {
		expect(MODULES.map((m) => m.key).sort()).toEqual([
			'cl_stocks',
			'etf',
			'tendencia',
			'us_stocks'
		]);
	});
});

describe('sinBase', () => {
	it('quita la subruta de despliegue y la barra final', () => {
		expect(sinBase('/Fog_Radar_Quant/chile/ranking', '/Fog_Radar_Quant')).toBe('/chile/ranking');
		expect(sinBase('/Fog_Radar_Quant/chile/', '/Fog_Radar_Quant')).toBe('/chile');
		expect(sinBase('/Fog_Radar_Quant', '/Fog_Radar_Quant')).toBe('/');
		expect(sinBase('/Fog_Radar_Quant/', '/Fog_Radar_Quant')).toBe('/');
		expect(sinBase('/etf', '')).toBe('/etf');
	});
});
