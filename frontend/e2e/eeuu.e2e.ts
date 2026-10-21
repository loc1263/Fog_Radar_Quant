import { expect, test } from '@playwright/test';

/**
 * Módulo EEUU. Requiere una corrida en data/us_stocks/runs y el export estático
 * (`fogradar export --modulo us`, que deja frontend/static/data).
 */

test('top 15 con sparklines y datos de la corrida', async ({ page }) => {
	await page.goto('/eeuu');
	const table = page.getByRole('table', { name: 'Top 15 por momentum' });
	await expect(table.locator('tbody tr')).toHaveCount(15);
	await expect(page.getByText('Universo (S&P 500 ∪ Nasdaq-100)')).toBeVisible();
	await expect(table.locator('tbody tr').first().getByRole('link').first()).toHaveAttribute(
		'href',
		/stockanalysis\.com/
	);
});

test('candidatos: búsqueda', async ({ page }) => {
	await page.goto('/eeuu/candidatos');
	const table = page.getByRole('table', { name: 'Candidatos de la última corrida' });
	expect(await table.locator('tbody tr').count()).toBeGreaterThan(20);
	await page.getByRole('searchbox', { name: 'Buscar', exact: true }).fill('nvidia');
	await expect(table.locator('tbody tr')).toHaveCount(1);
	await expect(table.getByRole('link', { name: 'NVDA', exact: true })).toBeVisible();
});

test('evaluación ordenada por score y rendimiento creciente', async ({ page }) => {
	await page.goto('/eeuu/evaluacion');
	const table = page.getByRole('table', { name: 'Evaluación de salud de los candidatos' });
	await expect(table.getByRole('columnheader', { name: /Score/ })).toHaveAttribute(
		'aria-sort',
		'descending'
	);

	await page.goto('/eeuu/rendimiento');
	const perf = page.getByRole('table', { name: 'Rendimiento por plazo' });
	const total = await perf.locator('tbody tr').count();
	const toggle = page.getByRole('link', { name: /Solo rendimientos crecientes/ });
	const n = Number((await toggle.innerText()).match(/\((\d+)\)/)?.[1]);
	await toggle.click();
	await expect(page).toHaveURL(/crecientes=1/);
	await expect(toggle).toHaveAttribute('aria-current', 'true');
	if (n > 0) await expect(perf.locator('tbody tr')).toHaveCount(n);
	expect(n).toBeLessThanOrEqual(total);
});

test('Portafolio precalculado: pestañas por universo, selección y pesos', async ({ page }) => {
	await page.goto('/eeuu/portafolio');
	await expect(page.getByText('Portafolios precalculados')).toBeVisible();
	await expect(
		page.getByRole('table', { name: /Selección por cluster y retornos de Black-Litterman/ })
	).toBeVisible();
	await expect(page.getByRole('list', { name: 'Composición del portafolio' })).toBeVisible();

	await page.getByRole('link', { name: /Top 15 por score/ }).click();
	await expect(page).toHaveURL(/u=top15/);
	await expect(page.getByRole('link', { name: /Top 15 por score/ })).toHaveAttribute(
		'aria-current',
		'page'
	);
});
