/** Tabla común de ETFs (port de Top15Table.test.tsx / MomentumTable.test.tsx). */
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import EtfTable from './EtfTable.svelte';
import { col, conPosicion } from './format';

const filas = conPosicion([
	{
		ticker: 'SOXX',
		nombre: 'iShares Semiconductor ETF',
		categoria: 'Technology',
		perf_5d: 5.64,
		perf_1y: 129.26,
		precio: 95,
		max_52s: 100,
		min_52s: 50,
		score: 91.3
	}
]);
type Fila = (typeof filas)[number];
const columns = [
	col.pos<Fila>(),
	col.ticker<Fila>(),
	col.nombre<Fila>(),
	col.rango<Fila>(),
	col.score<Fila>()
];

describe('EtfTable', () => {
	it('renderiza ticker, nombre, categoría, rango 52S y score', async () => {
		render(EtfTable<Fila>, { rows: filas, columns });
		await expect.element(page.getByRole('link', { name: 'SOXX' })).toBeInTheDocument();
		await expect.element(page.getByText('iShares Semiconductor ETF')).toBeInTheDocument();
		await expect.element(page.getByText('Technology')).toBeInTheDocument();
		await expect.element(page.getByText('Cerca del máximo')).toBeInTheDocument();
		await expect.element(page.getByText('91,3')).toBeInTheDocument();
	});

	it('muestra el mensaje de vacío', async () => {
		render(EtfTable<Fila>, {
			rows: [],
			columns,
			emptyText: 'El análisis de Momentum/Crecimiento no se corrió en esta corrida.'
		});
		await expect.element(page.getByText(/no se corrió en esta corrida/)).toBeInTheDocument();
	});
});
