/**
 * Smoke Tests
 *
 * Quick verification that core functionality works after major changes.
 * These tests are intentionally simple and fast.
 *
 * Usage:
 *   cd web/frontend && bun run test:e2e
 */

import { test, expect } from '@playwright/test';

test.describe('Smoke Tests', () => {
	test('home page loads and shows feeds', async ({ page }) => {
		await page.goto('/');

		// Page title should be set
		await expect(page).toHaveTitle(/Feeds/);

		// Should show Inbox (system feed)
		await expect(page.getByRole('link', { name: /Inbox/ })).toBeVisible();

		// Should show Everything link
		await expect(page.getByRole('link', { name: /Everything/ })).toBeVisible();
	});

	test('can navigate to a feed', async ({ page }) => {
		await page.goto('/');

		// Click on Inbox
		await page.getByRole('link', { name: /Inbox/ }).click();

		// Should navigate to feed page
		await expect(page).toHaveURL(/\/feeds\/\d+/);

		// Should show feed title
		await expect(page.getByRole('heading', { name: /Inbox/ })).toBeVisible();
	});

	test('all videos page loads', async ({ page }) => {
		await page.goto('/all');

		// Should show page title
		await expect(page).toHaveTitle(/All Videos|Everything/);
	});

	test('history page loads', async ({ page }) => {
		await page.goto('/history');

		// Should show history heading or empty state
		await expect(page).toHaveTitle(/History/);
	});

	test('import page loads without AI option', async ({ page }) => {
		await page.goto('/import');

		// Should show import page
		await expect(page).toHaveTitle(/Import/);

		// Should show Watch History heading
		await expect(page.getByRole('heading', { name: 'Watch History', exact: true })).toBeVisible();

		// Should NOT show "Organize with AI" button (we removed it)
		await expect(page.getByRole('button', { name: /Organize with AI/i })).not.toBeVisible();
	});

	test('feed page shows videos tab', async ({ page }) => {
		await page.goto('/');

		// Navigate to first feed
		const feedLink = page.locator('a[href^="/feeds/"]').first();
		await feedLink.click();

		// Wait for navigation
		await expect(page).toHaveURL(/\/feeds\/\d+/);

		// Should show Videos tab (default)
		await expect(page.getByRole('button', { name: /Videos/i })).toBeVisible();
	});

	test('API health check - feeds endpoint', async ({ request }) => {
		const response = await request.get('/api/feeds');
		expect(response.ok()).toBeTruthy();

		const feeds = await response.json();
		expect(Array.isArray(feeds)).toBeTruthy();

		// Should have at least Inbox
		expect(feeds.length).toBeGreaterThan(0);
	});

	test('API health check - config endpoint', async ({ request }) => {
		const response = await request.get('/api/config');
		expect(response.ok()).toBeTruthy();

		const config = await response.json();
		// Should have ytdlpCookiesConfigured but NOT aiEnabled
		expect(config).toHaveProperty('ytdlpCookiesConfigured');
		expect(config).not.toHaveProperty('aiEnabled');
	});

	test('API health check - recent videos endpoint', async ({ request }) => {
		const response = await request.get('/api/videos/recent');
		expect(response.ok()).toBeTruthy();

		const data = await response.json();
		expect(data).toHaveProperty('videos');
		expect(data).toHaveProperty('total');
	});
});
