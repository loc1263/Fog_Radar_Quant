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

test('buscador global: encuentra un ticker y lleva a su módulo', async ({ page }) => {
	await page.goto('/');
	const buscador = page
		.getByRole('navigation', { name: 'Navegación principal' })
		.getByRole('searchbox');
	await buscador.fill('vapores');
	await buscador.press('Enter');
	await expect(page).toHaveURL(/\/buscar\?q=vapores/);
	const primera = page.locator('tbody tr').first();
	await expect(primera).toContainText('Acciones Chile');
	await expect(primera).toContainText('exacto');
	await primera.getByRole('link', { name: 'VAPORES.SN' }).click();
	await expect(page).toHaveURL(/\/chile\?q=VAPORES\.SN/);
	// La tabla de Chile llega filtrada por el ticker.
	await expect(page.getByRole('searchbox', { name: 'Buscar', exact: true })).toHaveValue(
		'VAPORES.SN'
	);
	await expect(
		page.getByRole('table', { name: 'Ranking multi-factor del IPSA' }).locator('tbody tr')
	).toHaveCount(1);
});

test('buscador global sin coincidencias', async ({ page }) => {
	await page.goto('/buscar?q=zzzzzz');
	await expect(page.getByText('Ningún módulo tiene ese ticker o nombre')).toBeVisible();
});
