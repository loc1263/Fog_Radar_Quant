/** Datos ficticios para la vitrina de componentes (no son datos de mercado reales). */

export type SampleRow = {
	ticker: string;
	nombre: string;
	sector: string;
	score: number | null;
	ret1y: number | null;
	peso: number;
	precios: number[];
};

function walk(seed: number, n: number, drift: number): number[] {
	let x = 100;
	let s = seed;
	return Array.from({ length: n }, () => {
		s = (s * 9301 + 49297) % 233280;
		x *= 1 + drift + (s / 233280 - 0.5) * 0.04;
		return Math.round(x * 100) / 100;
	});
}

export const rows: SampleRow[] = [
	{
		ticker: 'AAA',
		nombre: 'Alfa Energía',
		sector: 'Energía',
		score: 82,
		ret1y: 0.214,
		peso: 0.119,
		precios: walk(1, 30, 0.004)
	},
	{
		ticker: 'BBB',
		nombre: 'Beta Bancos',
		sector: 'Financiero',
		score: 67,
		ret1y: 0.081,
		peso: 0.094,
		precios: walk(2, 30, 0.001)
	},
	{
		ticker: 'CCC',
		nombre: 'Gamma Minería',
		sector: 'Materiales',
		score: 38,
		ret1y: -0.126,
		peso: 0.052,
		precios: walk(3, 30, -0.004)
	},
	{
		ticker: 'DDD',
		nombre: 'Delta Retail',
		sector: 'Consumo',
		score: null,
		ret1y: null,
		peso: 0.031,
		precios: []
	},
	{
		ticker: 'EEE',
		nombre: 'Épsilon Tecnología',
		sector: 'Tecnología',
		score: 91,
		ret1y: 0.452,
		peso: 0.143,
		precios: walk(5, 30, 0.006)
	}
];

export const series = (() => {
	const a = walk(11, 24, 0.008);
	const b = walk(12, 24, 0.003);
	return a.map((v, i) => ({
		fecha: new Date(Date.UTC(2024, i, 1)),
		portafolio: v,
		benchmark: b[i]
	}));
})();

export const sectores = [
	{ sector: 'Tecnología', peso: 0.32 },
	{ sector: 'Financiero', peso: 0.21 },
	{ sector: 'Energía', peso: 0.18 },
	{ sector: 'Materiales', peso: 0.15 },
	{ sector: 'Consumo', peso: 0.14 }
];

export const frontera = Array.from({ length: 20 }, (_, i) => {
	const vol = 0.1 + i * 0.012;
	return { vol, ret: 0.04 + Math.sqrt(i) * 0.035 };
});

export const carteras = [
	{ vol: 0.16, ret: 0.13 },
	{ vol: 0.22, ret: 0.15 },
	{ vol: 0.28, ret: 0.12 }
];
