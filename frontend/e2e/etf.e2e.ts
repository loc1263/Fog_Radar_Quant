import { expect, test } from '@playwright/test';

/**
 * Módulo ETF. Requiere la corrida del 24-09-2026 en data/etf (en CI se copia
 * de backend/tests/etf/fixtures).
 */

test('resumen: KPIs y gráfico del Top 5', async ({ page }) => {
	await page.goto('/etf');
	await expect(page.getByText('Universo ETF bruto')).toBeVisible();
	await expect(page.getByText('1.189')).toBeVisible();
	await expect(page.locator('svg').filter({ hasText: '6M' }).first()).toBeVisible();
});

test('top 15 y exportación CSV con la fecha de la corrida', async ({ page }) => {
	await page.goto('/etf/top15');
	const table = page.getByRole('table', { name: 'Top 15 ETFs recomendados' });
	await expect(table.locator('tbody tr')).toHaveCount(15);
	await expect(table.getByRole('link', { name: 'CRAK' })).toBeVisible();
	const download = page.waitForEvent('download');
	await page.getByRole('button', { name: 'Descargar CSV' }).click();
	expect((await download).suggestedFilename()).toBe('etf_top15_recomendados_20260924.csv');
});

test('universo completo: paginado y búsqueda', async ({ page }) => {
	await page.goto('/etf/universo');
	const table = page.getByRole('table', { name: 'Universo depurado' });
	await expect(table.locator('tbody tr')).toHaveCount(50);
	await expect(page.getByText('Página 1 de 14')).toBeVisible();
	await page.getByRole('searchbox').fill('semiconductor');
	await expect(page.getByText('Página 1 de 14')).toBeHidden();
	expect(await table.locator('tbody tr').count()).toBeGreaterThan(0);
});

test('rotación y categorías', async ({ page }) => {
	await page.goto('/etf/rotacion');
	await expect(page.getByText('comparada con la del 22/09/2026')).toBeVisible();
	await expect(
		page.getByRole('table', { name: 'Salidas del Top 15' }).locator('tbody tr')
	).toHaveCount(8);

	await page.goto('/etf/categorias');
	await page.getByRole('button', { name: /Ver los 7 ETFs de Commodities Broad Basket/ }).click();
	await expect(page.getByText('7 ETFs en la categoría')).toBeVisible();
});

test('descarga del PDF por el BFF', async ({ request }) => {
	const res = await request.get('/etf/descargar/informe.pdf');
	expect(res.status()).toBe(200);
	expect(res.headers()['content-type']).toBe('application/pdf');
	expect((await res.body()).subarray(0, 4).toString()).toBe('%PDF');
});
