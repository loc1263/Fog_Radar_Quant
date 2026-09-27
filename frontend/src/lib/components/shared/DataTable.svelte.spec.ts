import { page } from 'vitest/browser';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DataTable from './DataTable.svelte';
import type { Row } from '$lib/table';

const rows = [
	{ ticker: 'AAA', score: 10 },
	{ ticker: 'BBB', score: 30 },
	{ ticker: 'CCC', score: 20 }
];
const columns = [
	{ key: 'ticker', label: 'Ticker' },
	{ key: 'score', label: 'Score', numeric: true }
];

function tickers(): string[] {
	return Array.from(document.querySelectorAll('tbody tr td:first-child')).map(
		(td) => td.textContent?.trim() ?? ''
	);
}

describe('DataTable', () => {
	it('ordena al hacer clic en el encabezado (números primero desc)', async () => {
		render(DataTable, { rows, columns });
		expect(tickers()).toEqual(['AAA', 'BBB', 'CCC']);
		await page.getByRole('button', { name: /Score/ }).click();
		expect(tickers()).toEqual(['BBB', 'CCC', 'AAA']);
		await page.getByRole('button', { name: /Score/ }).click();
		expect(tickers()).toEqual(['AAA', 'CCC', 'BBB']);
	});

	it('filtra con la búsqueda y muestra el contador', async () => {
		render(DataTable, { rows, columns, searchKeys: ['ticker'] });
		await page.getByRole('searchbox').fill('bb');
		expect(tickers()).toEqual(['BBB']);
		await expect.element(page.getByText('1 de 3')).toBeInTheDocument();
	});

	it('muestra el texto de vacío', async () => {
		render(DataTable, { rows: [], columns, emptyText: 'Nada por aquí' });
		await expect.element(page.getByText('Nada por aquí')).toBeInTheDocument();
	});

	it('despliega el detalle de una fila (una a la vez)', async () => {
		const detail = createRawSnippet<[{ row: Row }]>((args) => ({
			render: () => `<p>Detalle de ${String(args().row.ticker)}</p>`
		}));
		render(DataTable, {
			rows,
			columns,
			rowKey: (r: Row) => String(r.ticker),
			detailLabel: (r: Row) => `Ver ${String(r.ticker)}`,
			detail
		});
		const btn = page.getByRole('button', { name: 'Ver BBB' });
		await expect.element(btn).toHaveAttribute('aria-expanded', 'false');
		await btn.click();
		await expect.element(page.getByText('Detalle de BBB')).toBeInTheDocument();
		await expect.element(btn).toHaveAttribute('aria-expanded', 'true');
		await page.getByRole('button', { name: 'Ver AAA' }).click();
		await expect.element(page.getByText('Detalle de AAA')).toBeInTheDocument();
		await expect.element(page.getByText('Detalle de BBB')).not.toBeInTheDocument();
	});
});
