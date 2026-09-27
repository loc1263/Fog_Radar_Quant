import { describe, expect, it } from 'vitest';
import { isActive, MODULES } from './nav';

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
