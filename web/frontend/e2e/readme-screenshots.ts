/**
 * README Screenshot Capture Script
 *
 * Captures polished screenshots for the README - both desktop and mobile.
 */

import { chromium, type Browser, type BrowserContext, type Page } from 'playwright';
import { mkdir } from 'fs/promises';
import { join } from 'path';

const BASE_URL = process.env.BASE_URL || 'http://localhost:5173';
const OUTPUT_DIR = join(import.meta.dir, '..', '..', '..', 'screenshots');

async function captureReadmeScreenshots() {
	console.log('📸 Capturing README screenshots...\n');

	await mkdir(OUTPUT_DIR, { recursive: true });

	const browser: Browser = await chromium.launch({
		headless: true,
		args: ['--no-sandbox', '--disable-setuid-sandbox']
	});

	// Desktop screenshots (1280x800)
	console.log('🖥️  Desktop screenshots...');
	const desktopContext: BrowserContext = await browser.newContext({
		viewport: { width: 1280, height: 800 },
		deviceScaleFactor: 2
	});
	const desktopPage: Page = await desktopContext.newPage();

	// Home page - desktop
	await desktopPage.goto(`${BASE_URL}/`, { waitUntil: 'networkidle', timeout: 30000 });
	await desktopPage.waitForTimeout(500);
	await desktopPage.screenshot({ path: join(OUTPUT_DIR, 'desktop-home.png') });
	console.log('   ✓ desktop-home.png');

	// Try to get a feed page
	const feedLink = await desktopPage.$('a[href^="/feeds/"]');
	if (feedLink) {
		const href = await feedLink.getAttribute('href');
		if (href) {
			await desktopPage.goto(`${BASE_URL}${href}`, { waitUntil: 'networkidle' });
			await desktopPage.waitForTimeout(500);
			await desktopPage.screenshot({ path: join(OUTPUT_DIR, 'desktop-feed.png') });
			console.log('   ✓ desktop-feed.png');
		}
	}

	// Try to get a watch page
	await desktopPage.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
	const videoLink = await desktopPage.$('a[href^="/watch/"]');
	if (videoLink) {
		const href = await videoLink.getAttribute('href');
		if (href) {
			await desktopPage.goto(`${BASE_URL}${href}`, { waitUntil: 'networkidle' });
			await desktopPage.waitForTimeout(1500); // Extra time for video player
			await desktopPage.screenshot({ path: join(OUTPUT_DIR, 'desktop-watch.png') });
			console.log('   ✓ desktop-watch.png');
		}
	}

	await desktopContext.close();

	// Mobile screenshots (iPhone 14 Pro - 390x844)
	console.log('\n📱 Mobile screenshots...');
	const mobileContext: BrowserContext = await browser.newContext({
		viewport: { width: 390, height: 844 },
		deviceScaleFactor: 2,
		isMobile: true,
		hasTouch: true,
		userAgent:
			'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1'
	});
	const mobilePage: Page = await mobileContext.newPage();

	// Home page - mobile
	await mobilePage.goto(`${BASE_URL}/`, { waitUntil: 'networkidle', timeout: 30000 });
	await mobilePage.waitForTimeout(500);
	await mobilePage.screenshot({ path: join(OUTPUT_DIR, 'mobile-home.png') });
	console.log('   ✓ mobile-home.png');

	// Feed page - mobile
	const mobileFeedLink = await mobilePage.$('a[href^="/feeds/"]');
	if (mobileFeedLink) {
		const href = await mobileFeedLink.getAttribute('href');
		if (href) {
			await mobilePage.goto(`${BASE_URL}${href}`, { waitUntil: 'networkidle' });
			await mobilePage.waitForTimeout(500);
			await mobilePage.screenshot({ path: join(OUTPUT_DIR, 'mobile-feed.png') });
			console.log('   ✓ mobile-feed.png');
		}
	}

	// Watch page - mobile
	await mobilePage.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
	const mobileVideoLink = await mobilePage.$('a[href^="/watch/"]');
	if (mobileVideoLink) {
		const href = await mobileVideoLink.getAttribute('href');
		if (href) {
			await mobilePage.goto(`${BASE_URL}${href}`, { waitUntil: 'networkidle' });
			await mobilePage.waitForTimeout(1500);
			await mobilePage.screenshot({ path: join(OUTPUT_DIR, 'mobile-watch.png') });
			console.log('   ✓ mobile-watch.png');
		}
	}

	await mobileContext.close();
	await browser.close();

	console.log(`\n✅ Screenshots saved to: ${OUTPUT_DIR}`);
}

captureReadmeScreenshots().catch(console.error);
