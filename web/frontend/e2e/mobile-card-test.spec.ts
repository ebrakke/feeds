import { test, expect } from '@playwright/test';

test.describe('Mobile Card Layout', () => {
	test('video cards fit within mobile viewport on all pages', async ({ page }) => {
		await page.setViewportSize({ width: 320, height: 568 });

		// Test /all page
		await page.goto('/all');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('article.card', { timeout: 10000 });

		const viewportWidth = page.viewportSize()?.width || 320;

		// Check cards don't overflow
		const cards = await page.locator('article.card').all();
		expect(cards.length).toBeGreaterThan(0);

		for (let i = 0; i < Math.min(cards.length, 5); i++) {
			const card = cards[i];
			const box = await card.boundingBox();
			if (box) {
				expect(box.x + box.width).toBeLessThanOrEqual(viewportWidth + 1);
			}
		}

		// Check no horizontal scrollbar
		const scrollInfo = await page.evaluate(() => ({
			bodyScrollWidth: document.body.scrollWidth,
			bodyClientWidth: document.body.clientWidth
		}));
		expect(scrollInfo.bodyScrollWidth).toBeLessThanOrEqual(scrollInfo.bodyClientWidth);

		// Take screenshot
		await page.screenshot({ path: './e2e/results/mobile-all-320w-final.png' });
	});

	test('video cards fit on feed pages', async ({ page }) => {
		await page.setViewportSize({ width: 320, height: 568 });
		await page.goto('/feeds/2');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('article.card', { timeout: 10000 });

		const viewportWidth = page.viewportSize()?.width || 320;

		const cards = await page.locator('article.card').all();
		for (let i = 0; i < Math.min(cards.length, 5); i++) {
			const card = cards[i];
			const box = await card.boundingBox();
			if (box) {
				expect(box.x + box.width).toBeLessThanOrEqual(viewportWidth + 1);
			}
		}

		// Take screenshot
		await page.screenshot({ path: './e2e/results/mobile-feed-320w-final.png' });
	});

	test('card content (time, menu button) is visible', async ({ page }) => {
		await page.setViewportSize({ width: 320, height: 568 });
		await page.goto('/all');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('article.card', { timeout: 10000 });

		const viewportWidth = page.viewportSize()?.width || 320;

		// Check menu buttons are visible
		const menuButtons = await page.locator('article.card button[title="More options"]').all();
		expect(menuButtons.length).toBeGreaterThan(0);

		for (let i = 0; i < Math.min(menuButtons.length, 3); i++) {
			const button = menuButtons[i];
			const box = await button.boundingBox();
			if (box) {
				expect(box.x + box.width).toBeLessThanOrEqual(viewportWidth);
			}
		}

		// Check time elements are visible
		const timeElements = await page.locator('article.card span.text-text-muted.whitespace-nowrap').all();
		for (let i = 0; i < Math.min(timeElements.length, 3); i++) {
			const timeEl = timeElements[i];
			const box = await timeEl.boundingBox();
			if (box) {
				expect(box.x + box.width).toBeLessThanOrEqual(viewportWidth);
			}
		}
	});

	test('cards work at standard mobile width (375px)', async ({ page }) => {
		await page.setViewportSize({ width: 375, height: 812 });
		await page.goto('/all');
		await page.waitForLoadState('networkidle');
		await page.waitForSelector('article.card', { timeout: 10000 });

		const viewportWidth = page.viewportSize()?.width || 375;

		const cards = await page.locator('article.card').all();
		for (let i = 0; i < Math.min(cards.length, 3); i++) {
			const card = cards[i];
			const box = await card.boundingBox();
			if (box) {
				expect(box.x + box.width).toBeLessThanOrEqual(viewportWidth + 1);
			}
		}

		// Take screenshot
		await page.screenshot({ path: './e2e/results/mobile-all-375w-final.png' });
	});
});
