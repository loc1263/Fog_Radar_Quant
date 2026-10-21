import { expect, test } from '@playwright/test';

/**
 * Módulo Tendencia. Requiere la corrida 20260926_01 en data/tendencia/output y
 * history.db (con datos reales y `fogradar job tendencia --backfill`).
 */

test('resumen y selector de análisis en la URL', async ({ page }) => {
	await page.goto('/tendencia');
	await expect(page.getByText('corrida 20260926_01')).toBeVisible();
	await expect(page.getByText('Sharpe Ratio (portafolio)')).toBeVisible();
	await page.getByRole('link', { name: 'solo EEUU' }).click();
	await expect(page).toHaveURL(/analisis=us/);
	await expect(
		page.getByRole('heading', { name: /Resumen ejecutivo \(solo EEUU\)/ })
	).toBeVisible();
	// La sección conserva el análisis al navegar.
	await page.getByRole('link', { name: 'Portafolio', exact: true }).click();
	await expect(page).toHaveURL(/\/tendencia\/portafolio\?analisis=us/);
});

test('portafolio: selector 5 / 10 / 15', async ({ page }) => {
	await page.goto('/tendencia/portafolio');
	const table = page.getByRole('table', { name: 'Portafolio seleccionado' });
	await expect(table.locator('tbody tr')).toHaveCount(10);
	await page.getByRole('link', { name: '5 acciones', exact: true }).click();
	await expect(table.locator('tbody tr')).toHaveCount(5);
	await page.getByRole('link', { name: '15 acciones' }).click();
	await expect(table.locator('tbody tr')).toHaveCount(15);
});

test('momentum mensual y semanal', async ({ page }) => {
	await page.goto('/tendencia/momentum');
	await expect(
		page.getByRole('table', { name: 'Momentum mensual' }).locator('tbody tr')
	).toHaveCount(10);
	await expect(
		page.getByRole('table', { name: 'Momentum semanal' }).locator('tbody tr')
	).toHaveCount(10);
});
