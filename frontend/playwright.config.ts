import { defineConfig } from '@playwright/test';

/**
 * E2E de humo sobre el sitio estático. Requiere los datos exportados
 * (`fogradar export --out ../frontend/static/data`, lo hace `iniciar.bat`).
 */
export default defineConfig({
	webServer: {
		command: 'npm run build && npm run preview -- --port 4173',
		port: 4173,
		reuseExistingServer: !process.env.CI
	},
	use: { baseURL: 'http://localhost:4173' },
	testDir: 'e2e',
	testMatch: '**/*.e2e.{ts,js}'
});
