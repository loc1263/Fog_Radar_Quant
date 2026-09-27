import { describe, expect, it } from 'vitest';
import { fmtDuration, fmtNumber, fmtPct, signedClass } from './format';

describe('format', () => {
	it('fmtNumber usa coma decimal y guion para vacíos', () => {
		expect(fmtNumber(1234.5)).toBe('1.234,50');
		expect(fmtNumber(null)).toBe('—');
		expect(fmtNumber(Number.NaN)).toBe('—');
	});

	it('fmtPct convierte fracciones y agrega signo', () => {
		expect(fmtPct(0.123)).toBe('12,3 %');
		expect(fmtPct(5.16, { isPercent: true, digits: 2 })).toBe('5,16 %');
		expect(fmtPct(0.05, { signed: true })).toBe('+5,0 %');
		expect(fmtPct(-0.05, { signed: true })).toBe('-5,0 %');
	});

	it('fmtDuration', () => {
		expect(fmtDuration(12.34)).toBe('12.3 s');
		expect(fmtDuration(125)).toBe('2 min 5 s');
		expect(fmtDuration(3725)).toBe('1 h 2 min');
	});

	it('signedClass', () => {
		expect(signedClass(0)).toContain('text-brand-green');
		expect(signedClass(-1)).toContain('text-brand-red');
		expect(signedClass(undefined)).toBe('');
	});
});
