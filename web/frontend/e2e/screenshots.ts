/**
 * Mobile Screenshot Capture Script
 *
 * Captures screenshots of all main pages in mobile viewport for UI review.
 *
 * Usage:
 *   bun run e2e:screenshots
 *
 * Requires the dev server to be running on localhost:5173
 */

import { chromium, type Browser, type BrowserContext, type Page } from 'playwright';
import { mkdir } from 'fs/promises';
import { join } from 'path';

const BASE_URL = process.env.BASE_URL || 'http://localhost:5173';
const OUTPUT_DIR = join(import.meta.dir, 'screenshots');

// iPhone 14 Pro viewport
const MOBILE_VIEWPORT = {
	width: 390,
	height: 844,
	deviceScaleFactor: 2,
	isMobile: true,
	hasTouch: true
};

const PAGES = [
	{ path: '/', name: 'home' },
	{ path: '/all', name: 'all-videos' },
	{ path: '/history', name: 'history' },
	{ path: '/import', name: 'import' }
];

async function captureScreenshots() {
	console.log('📱 Starting mobile screenshot capture...\n');

	// Create output directory
	await mkdir(OUTPUT_DIR, { recursive: true });

	const browser: Browser = await chromium.launch({
		headless: true,
		args: ['--no-sandbox', '--disable-setuid-sandbox']
	});

	const context: BrowserContext = await browser.newContext({
		viewport: { width: MOBILE_VIEWPORT.width, height: MOBILE_VIEWPORT.height },
		deviceScaleFactor: MOBILE_VIEWPORT.deviceScaleFactor,
		isMobile: MOBILE_VIEWPORT.isMobile,
		hasTouch: MOBILE_VIEWPORT.hasTouch,
		userAgent:
			'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1'
	});

	const page: Page = await context.newPage();

	// Capture static pages
	for (const { path, name } of PAGES) {
		const url = `${BASE_URL}${path}`;
		console.log(`📸 Capturing ${name} (${url})`);

		try {
			await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 });
			await page.waitForTimeout(500); // Allow animations to settle

			const filename = join(OUTPUT_DIR, `mobile-${name}.png`);
			await page.screenshot({ path: filename, fullPage: false });
			console.log(`   ✓ Saved: ${filename}`);
		} catch (err) {
			console.error(`   ✗ Failed: ${err}`);
		}
	}

	// Try to capture a feed page if feeds exist
	try {
		await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
		const feedLink = await page.$('a[href^="/feeds/"]');
		if (feedLink) {
			const href = await feedLink.getAttribute('href');
			if (href) {
				console.log(`📸 Capturing feed page (${href})`);
				await page.goto(`${BASE_URL}${href}`, { waitUntil: 'networkidle' });
				await page.waitForTimeout(500);
				const filename = join(OUTPUT_DIR, 'mobile-feed-detail.png');
				await page.screenshot({ path: filename, fullPage: false });
				console.log(`   ✓ Saved: ${filename}`);
			}
		}
	} catch (err) {
		console.log(`   ⚠ Could not capture feed page: ${err}`);
	}

	// Try to capture a channel page
	try {
		await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
		const channelLink = await page.$('a[href^="/channels/"]');
		if (channelLink) {
			const href = await channelLink.getAttribute('href');
			if (href) {
				console.log(`📸 Capturing channel page (${href})`);
				await page.goto(`${BASE_URL}${href}`, { waitUntil: 'networkidle' });
				await page.waitForTimeout(500);
				const filename = join(OUTPUT_DIR, 'mobile-channel.png');
				await page.screenshot({ path: filename, fullPage: false });
				console.log(`   ✓ Saved: ${filename}`);
			}
		}
	} catch (err) {
		console.log(`   ⚠ Could not capture channel page: ${err}`);
	}

	// Try to capture watch page
	try {
		await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
		const videoLink = await page.$('a[href^="/watch/"]');
		if (videoLink) {
			const href = await videoLink.getAttribute('href');
			if (href) {
				console.log(`📸 Capturing watch page (${href})`);
				await page.goto(`${BASE_URL}${href}`, { waitUntil: 'networkidle' });
				await page.waitForTimeout(1000); // Video player may need more time
				const filename = join(OUTPUT_DIR, 'mobile-watch.png');
				await page.screenshot({ path: filename, fullPage: false });
				console.log(`   ✓ Saved: ${filename}`);
			}
		}
	} catch (err) {
		console.log(`   ⚠ Could not capture watch page: ${err}`);
	}

	await browser.close();

	console.log('\n✅ Screenshot capture complete!');
	console.log(`   Output directory: ${OUTPUT_DIR}`);
}

captureScreenshots().catch(console.error);
