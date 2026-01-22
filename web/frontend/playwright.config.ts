import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	outputDir: './e2e/results',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: 'html',
	use: {
		baseURL: process.env.BASE_URL || 'http://localhost:5173',
		trace: 'on-first-retry',
		screenshot: 'only-on-failure',
		// Required when running as root
		launchOptions: {
			args: ['--no-sandbox', '--disable-setuid-sandbox']
		}
	},
	projects: [
		{
			name: 'desktop-chrome',
			use: { ...devices['Desktop Chrome'] }
		},
		{
			name: 'mobile-chrome',
			use: { ...devices['Pixel 5'] }
		},
		{
			name: 'mobile-safari',
			use: { ...devices['iPhone 14 Pro'] }
		}
	],
	webServer: process.env.CI || process.env.NO_WEBSERVER
		? undefined
		: {
				command: 'npm run dev',
				url: 'http://localhost:5173',
				reuseExistingServer: true
			}
});
