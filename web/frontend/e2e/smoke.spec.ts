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

	test('API health check - channel pagination', async ({ request }) => {
		// Get feeds first to find a channel
		const feedsResponse = await request.get('/api/feeds');
		const feeds = await feedsResponse.json();

		if (feeds.length > 0) {
			// Get first feed's channels
			const feedResponse = await request.get(`/api/feeds/${feeds[0].id}`);
			const feedData = await feedResponse.json();

			if (feedData.channels && feedData.channels.length > 0) {
				const channelId = feedData.channels[0].id;

				// Test pagination params
				const response = await request.get(`/api/channels/${channelId}?limit=5&offset=0`);
				expect(response.ok()).toBeTruthy();

				const data = await response.json();
				expect(data).toHaveProperty('channel');
				expect(data).toHaveProperty('videos');
				expect(data).toHaveProperty('hasMore');
				expect(Array.isArray(data.videos)).toBeTruthy();
				expect(data.videos.length).toBeLessThanOrEqual(5);
			}
		}
	});

	test('channel page loads with load more button', async ({ page }) => {
		// Get a channel ID from the API
		const feedsResponse = await page.request.get('/api/feeds');
		const feeds = await feedsResponse.json();

		if (feeds.length > 0) {
			const feedResponse = await page.request.get(`/api/feeds/${feeds[0].id}`);
			const feedData = await feedResponse.json();

			if (feedData.channels && feedData.channels.length > 0) {
				const channelId = feedData.channels[0].id;

				await page.goto(`/channels/${channelId}`);

				// Should show channel name
				await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

				// Should show video count (may include '+' if hasMore)
				await expect(page.getByText(/\d+\+? videos/)).toBeVisible();
			}
		}
	});

	test('channel page buttons visible on mobile', async ({ page }) => {
		// Get a channel ID from the API
		const feedsResponse = await page.request.get('/api/feeds');
		const feeds = await feedsResponse.json();

		if (feeds.length > 0) {
			const feedResponse = await page.request.get(`/api/feeds/${feeds[0].id}`);
			const feedData = await feedResponse.json();

			if (feedData.channels && feedData.channels.length > 0) {
				const channelId = feedData.channels[0].id;

				await page.goto(`/channels/${channelId}`);

				// Wait for channel name to be visible (confirms page is loaded)
				await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

				// Should show both Fetch More and Refresh buttons
				await expect(page.getByRole('button', { name: /Fetch More/i })).toBeVisible();
				await expect(page.getByRole('button', { name: /Refresh/i })).toBeVisible();

				// Check buttons are within viewport bounds (not pushed off screen)
				const fetchMoreButton = page.getByRole('button', { name: /Fetch More/i });
				const refreshButton = page.getByRole('button', { name: /Refresh/i });

				const fetchMoreBox = await fetchMoreButton.boundingBox();
				const refreshBox = await refreshButton.boundingBox();

				expect(fetchMoreBox).toBeTruthy();
				expect(refreshBox).toBeTruthy();

				// Check buttons are within reasonable viewport bounds (not pushed off right edge)
				const viewport = page.viewportSize();
				if (viewport && fetchMoreBox && refreshBox) {
					expect(fetchMoreBox.x + fetchMoreBox.width).toBeLessThanOrEqual(viewport.width + 10);
					expect(refreshBox.x + refreshBox.width).toBeLessThanOrEqual(viewport.width + 10);
				}
			}
		}
	});
});
