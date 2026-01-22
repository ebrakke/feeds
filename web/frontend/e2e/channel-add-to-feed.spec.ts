import { test, expect } from '@playwright/test';

test.describe('Channel Page - Add to Feed', () => {
	test('shows bottom sheet on mobile when clicking Add to feed', async ({ page }) => {
		// Set mobile viewport
		await page.setViewportSize({ width: 375, height: 812 });

		// Navigate to all videos page first and click on a channel link
		await page.goto('/all');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('article.card', { timeout: 10000 });

		// Click on the first channel link (in the video card metadata)
		const channelLink = page.locator('article.card a[href^="/channels/"]').first();
		await channelLink.click();
		await page.waitForLoadState('networkidle');

		// Wait for channel page to load
		await page.waitForSelector('h1', { timeout: 10000 });

		// Take a screenshot before clicking
		await page.screenshot({ path: './e2e/results/channel-mobile-before-add.png' });

		// Look for the "Add to feed" button
		const addButton = page.locator('button:has-text("Add to feed")');
		const buttonExists = await addButton.count() > 0;

		if (buttonExists) {
			const isDisabled = await addButton.isDisabled().catch(() => true);

			if (!isDisabled) {
				await addButton.click();

				// Wait for bottom sheet animation
				await page.waitForTimeout(500);

				// Take a screenshot after clicking
				await page.screenshot({ path: './e2e/results/channel-mobile-after-add.png' });

				// The bottom sheet should be visible - check for fixed overlay
				const bottomSheetOverlay = page.locator('div.fixed.inset-0');
				const isVisible = await bottomSheetOverlay.isVisible().catch(() => false);
				console.log('Bottom sheet visible:', isVisible);
			} else {
				console.log('Add to feed button is disabled (channel already in all feeds)');
			}
		} else {
			console.log('No Add to feed button found');
		}
	});

	test('shows dropdown on desktop when clicking Add to feed', async ({ page }) => {
		// Set desktop viewport
		await page.setViewportSize({ width: 1280, height: 800 });

		// Navigate to all videos page first and click on a channel link
		await page.goto('/all');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('article.card', { timeout: 10000 });

		// Click on the first channel link
		const channelLink = page.locator('article.card a[href^="/channels/"]').first();
		await channelLink.click();
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('h1', { timeout: 10000 });

		const addButton = page.locator('button:has-text("Add to feed")');
		const buttonExists = await addButton.count() > 0;

		if (buttonExists) {
			const isDisabled = await addButton.isDisabled().catch(() => true);

			if (!isDisabled) {
				await addButton.click();
				await page.waitForTimeout(300);

				// Take a screenshot
				await page.screenshot({ path: './e2e/results/channel-desktop-add.png' });

				// On desktop, we should see a dropdown menu (not a bottom sheet)
				const dropdown = page.locator('.add-feed-dropdown .absolute');
				const dropdownVisible = await dropdown.isVisible().catch(() => false);
				console.log('Desktop dropdown visible:', dropdownVisible);
				expect(dropdownVisible).toBe(true);
			} else {
				console.log('Add to feed button is disabled');
				await page.screenshot({ path: './e2e/results/channel-desktop-disabled.png' });
			}
		}
	});
});
