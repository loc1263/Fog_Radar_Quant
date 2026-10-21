import { expect, test } from '@playwright/test';

const MODULES = [
	{ label: 'Chile', path: '/chile' },
	{ label: 'EEUU', path: '/eeuu' },
	{ label: 'ETFs', path: '/etf' },
	{ label: 'Tendencia', path: '/tendencia' }
];

test('el hub muestra los 4 módulos', async ({ page }) => {
	await page.goto('/');
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Fog Radar Quant');
	for (const m of MODULES) {
		await expect(page.getByRole('link', { name: new RegExp(`^${m.label}`) }).first()).toBeVisible();
	}
});

test('la navegación marca la página activa', async ({ page }) => {
	for (const m of MODULES) {
		await page.goto(m.path);
		const nav = page.getByRole('navigation', { name: 'Navegación principal' });
		await expect(nav.getByRole('link', { name: m.label, exact: true })).toHaveAttribute(
			'aria-current',
			'page'
		);
	}
});

test('el tema oscuro se aplica y persiste', async ({ page }) => {
	await page.goto('/');
	const html = page.locator('html');
	const wasDark = await html.evaluate((el) => el.classList.contains('dark'));
	await page.getByRole('button', { name: 'Cambiar tema' }).click();
	await expect(html).toHaveClass(wasDark ? /^(?!.*dark)/ : /dark/);
	await page.reload();
	await expect(html).toHaveClass(wasDark ? /^(?!.*dark)/ : /dark/);
});

test('ruta inexistente muestra la página de error', async ({ page }) => {
	const res = await page.goto('/no-existe');
	expect(res?.status()).toBe(404);
	await expect(page.getByText('Página no encontrada').first()).toBeVisible();
});

test('el hub muestra el top 3 y la frescura de cada módulo', async ({ page }) => {
	await page.goto('/');
	const tarjetas = page.getByRole('link').filter({ hasText: 'Top 3 ·' });
	await expect(tarjetas).toHaveCount(4);
	await expect(tarjetas.first().locator('ol li')).toHaveCount(3);
	await expect(
		tarjetas.first().getByText(/Datos al día|Datos atrasados/, { exact: true })
	).toBeVisible();
});
