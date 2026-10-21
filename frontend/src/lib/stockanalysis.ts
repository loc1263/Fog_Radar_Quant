// Ruta (sobre https://stockanalysis.com) de un ticker en formato Yahoo.
// EEUU: /stocks/<t>/ o /etf/<t>/; resto de mercados: /quote/<bolsa>/<código>/ según el sufijo de Yahoo.

const BOLSAS: Record<string, string> = {
	SN: 'bcs',
	KS: 'krx',
	KQ: 'kosdaq',
	T: 'tyo',
	TW: 'tpe',
	TWO: 'tpex',
	HK: 'hkg',
	SS: 'sha',
	SZ: 'szse',
	SI: 'sgx',
	L: 'lon',
	TO: 'tsx',
	AX: 'asx',
	NS: 'nse',
	BO: 'bom',
	DE: 'etr',
	PA: 'epa'
};

export function stockAnalysisPath(ticker: string, etf = false): string {
	const t = ticker.trim().toUpperCase();
	const i = t.lastIndexOf('.');
	const bolsa = i > 0 ? BOLSAS[t.slice(i + 1)] : undefined;
	if (bolsa) return `/quote/${bolsa}/${encodeURIComponent(t.slice(0, i))}/`;
	// EEUU: la clase de acción va con punto (BRK-B → brk.b)
	const us = encodeURIComponent(t.replace(/-/g, '.').toLowerCase());
	return `/${etf ? 'etf' : 'stocks'}/${us}/`;
}
