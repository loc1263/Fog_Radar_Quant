import { expect, test } from '@playwright/test';

/**
 * Módulo Chile. Requiere datos en data/cl_stocks (en CI se copian los
 * fixtures de backend/tests/cl_stocks/fixtures).
 */

test('ranking: tabla, detalle desplegable y pesos personalizados', async ({ page }) => {
	await page.goto('/chile');
	await expect(page.getByRole('heading', { name: 'Ranking IPSA' })).toBeVisible();

	const table = page.getByRole('table', { name: 'Ranking multi-factor del IPSA' });
	const rows = table.locator('tbody tr');
	expect(await rows.count()).toBeGreaterThan(5);
	await expect(rows.first().locator('td').nth(1)).toHaveText('1');

	// Detalle de la primera acción
	await table
		.getByRole('button', { name: /^Ver métricas de/ })
		.first()
		.click();
	await expect(table.getByText('Momentum (retorno de precio)')).toBeVisible();

	// Pesos: solo Calidad → cambia el líder; Restablecer vuelve al original
	const lider = await rows.first().locator('td').nth(2).innerText();
	for (const p of ['Valor', 'Crecimiento', 'Salud Financiera', 'Dividendo', 'Momentum']) {
		await page.getByLabel(p, { exact: true }).fill('0');
	}
	await page.getByLabel('Calidad', { exact: true }).fill('3');
	await expect(page.getByText('Pesos personalizados')).toBeVisible();
	await expect(page.getByText('100,0 % del score')).toBeVisible();
	const liderValor = await rows.first().locator('td').nth(2).innerText();
	expect(liderValor).not.toBe(lider);

	await page.getByRole('button', { name: 'Restablecer' }).click();
	await expect(page.getByText('Pesos por defecto del modelo.')).toBeVisible();
	await expect(rows.first().locator('td').nth(2)).toHaveText(lider);
});

test('ranking: el filtro de sector no cambia el puesto', async ({ page }) => {
	await page.goto('/chile');
	const table = page.getByRole('table', { name: 'Ranking multi-factor del IPSA' });
	await page.getByLabel('Sector').selectOption('Financial Services');
	const rows = table.locator('tbody tr');
	const n = await rows.count();
	expect(n).toBeGreaterThan(0);
	for (let i = 0; i < n; i++) {
		await expect(rows.nth(i).locator('td').nth(4)).toHaveText('Financial Services');
	}
});

test('indicadores: cambia de grupo de columnas', async ({ page }) => {
	await page.goto('/chile/indicadores');
	const table = page.getByRole('table', { name: 'Indicadores financieros del IPSA' });
	await expect(table.getByRole('button', { name: 'EV/EBITDA' })).toBeVisible();
	await page.getByRole('link', { name: 'Rentabilidad', exact: true }).click();
	await expect(page).toHaveURL(/grupo=Rentabilidad/);
	await expect(table.getByRole('button', { name: 'ROE' })).toBeVisible();
	await expect(table.getByRole('button', { name: 'EV/EBITDA' })).toHaveCount(0);
	const nav = page.getByRole('navigation', { name: 'Navegación principal' });
	await expect(nav.getByRole('link', { name: 'Chile', exact: true })).toHaveAttribute(
		'aria-current',
		'page'
	);
});

test('historial desde history.db', async ({ page }) => {
	await page.goto('/chile/historial');
	await expect(page.getByRole('table', { name: 'Corridas' }).locator('tbody tr')).not.toHaveCount(
		0
	);
	await expect(
		page.getByRole('table', { name: 'Trayectoria de la acción' }).locator('tbody tr')
	).not.toHaveCount(0);
});
