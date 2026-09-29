import { expect, test } from '@playwright/test';

/**
 * Módulo EEUU. Requiere una corrida en data/us_stocks/runs (en CI se copia
 * la de backend/tests/us_stocks/fixtures).
 */

test('top 15 con sparklines y datos de la corrida', async ({ page }) => {
	await page.goto('/eeuu');
	const table = page.getByRole('table', { name: 'Top 15 por momentum' });
	await expect(table.locator('tbody tr')).toHaveCount(15);
	await expect(page.getByText('Universo (S&P 500 ∪ Nasdaq-100)')).toBeVisible();
	await expect(
		table.locator('tbody tr').first().getByRole('link', { name: 'evaluar' })
	).toBeVisible();
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

test('Markowitz: optimizar y buscar la mejor combinación', async ({ page }) => {
	await page.goto('/eeuu/portafolio/markowitz');
	await expect(page.getByText(/15 seleccionados/)).toBeVisible();
	await page.getByRole('button', { name: 'Optimizar' }).click();
	await expect(page.getByText('Sharpe', { exact: true })).toBeVisible({ timeout: 20_000 });
	await expect(page.getByRole('heading', { name: 'Frontera eficiente' })).toBeVisible();
	await expect(page.getByRole('list', { name: 'Composición del portafolio' })).toBeVisible();

	// La selección se conserva tras optimizar (use:enhance sin reset)
	await expect(page.getByText(/15 seleccionados/)).toBeVisible();

	await page.getByLabel('Tickers por portafolio (k)').fill('3');
	await expect(page.getByText('455 combinaciones posibles')).toBeVisible();
	await page.getByRole('button', { name: 'Buscar' }).click();
	await expect(page.getByText('combinaciones evaluadas')).toBeVisible({ timeout: 45_000 });
	await expect(
		page.getByRole('table', { name: 'Mejores combinaciones' }).locator('tbody tr')
	).toHaveCount(5);
	// El resultado de optimizar sigue visible
	await expect(page.getByRole('heading', { name: 'Frontera eficiente' })).toBeVisible();
});

test('Black-Litterman muestra equilibrio, views y posterior', async ({ page }) => {
	await page.goto('/eeuu/portafolio/black-litterman');
	await page.getByRole('button', { name: 'Desmarcar todos' }).click();
	await expect(page.getByRole('button', { name: 'Optimizar' })).toBeDisabled();
	await page.getByRole('button', { name: 'Top 15' }).click();
	await page.getByRole('button', { name: 'Optimizar' }).click();
	await expect(
		page.getByRole('table', { name: 'Retornos de Black-Litterman por ticker' })
	).toBeVisible({ timeout: 20_000 });
});

test('evaluación en vivo: ticker inválido y página de ticker', async ({ page }) => {
	await page.goto('/eeuu/ticker?t=no%20valido');
	await expect(page.getByText('no es un ticker válido')).toBeVisible();

	await page.goto('/eeuu/ticker?t=aapl');
	await expect(page).toHaveURL(/\/eeuu\/ticker\/AAPL$/);
	// Con o sin acceso a Yahoo, la página responde (resultado o error controlado)
	await expect(page.getByRole('heading', { name: /Evaluación en vivo · AAPL/ })).toBeVisible();
});
