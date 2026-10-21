import { describe, expect, it } from 'vitest';
import { stockAnalysisPath } from './stockanalysis';

describe('stockAnalysisPath', () => {
	it('acciones y ETFs de EEUU', () => {
		expect(stockAnalysisPath('AAPL')).toBe('/stocks/aapl/');
		expect(stockAnalysisPath('SPY', true)).toBe('/etf/spy/');
		expect(stockAnalysisPath('BRK-B')).toBe('/stocks/brk.b/');
	});
	it('otros mercados por sufijo de Yahoo', () => {
		expect(stockAnalysisPath('SQM-B.SN')).toBe('/quote/bcs/SQM-B/');
		expect(stockAnalysisPath('005930.KS')).toBe('/quote/krx/005930/');
		expect(stockAnalysisPath('7203.T')).toBe('/quote/tyo/7203/');
		expect(stockAnalysisPath('2330.TW')).toBe('/quote/tpe/2330/');
	});
});
