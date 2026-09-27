import { defineConfig } from '@playwright/test';

/**
 * E2E de humo. Requiere el backend en PRIVATE_API_URL (por defecto :8000);
 * sin backend las páginas deben mostrar su estado de error, no romperse.
 */
export default defineConfig({
	webServer: {
		command: 'npm run build && node build',
		port: 4173,
		env: { PORT: '4173', ORIGIN: 'http://localhost:4173' },
		reuseExistingServer: !process.env.CI
	},
	use: { baseURL: 'http://localhost:4173' },
	testDir: 'e2e',
	testMatch: '**/*.e2e.{ts,js}'
});
