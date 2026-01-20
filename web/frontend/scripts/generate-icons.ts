import { chromium } from 'playwright';
import { writeFileSync } from 'fs';
import { join } from 'path';

const sizes = [192, 512];
const staticDir = join(import.meta.dirname, '../static');

async function generateIcons() {
  const browser = await chromium.launch();
  const page = await browser.newPage();

  for (const size of sizes) {
    await page.setViewportSize({ width: size, height: size });
    await page.setContent(`
      <!DOCTYPE html>
      <html>
        <head>
          <style>
            * { margin: 0; padding: 0; }
            body { width: ${size}px; height: ${size}px; }
            svg { width: 100%; height: 100%; }
          </style>
        </head>
        <body>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
            <defs>
              <linearGradient id="emerald-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stop-color="#10b981"/>
                <stop offset="100%" stop-color="#059669"/>
              </linearGradient>
            </defs>
            <rect width="512" height="512" rx="96" fill="url(#emerald-gradient)"/>
            <path d="M192 144v224l176-112z" fill="#050505"/>
          </svg>
        </body>
      </html>
    `);

    const screenshot = await page.screenshot({ type: 'png' });
    writeFileSync(join(staticDir, `icon-${size}.png`), screenshot);
    console.log(`Generated icon-${size}.png`);
  }

  // Generate favicon (32x32)
  await page.setViewportSize({ width: 32, height: 32 });
  await page.setContent(`
    <!DOCTYPE html>
    <html>
      <head>
        <style>
          * { margin: 0; padding: 0; }
          body { width: 32px; height: 32px; }
          svg { width: 100%; height: 100%; }
        </style>
      </head>
      <body>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
          <defs>
            <linearGradient id="emerald-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stop-color="#10b981"/>
              <stop offset="100%" stop-color="#059669"/>
            </linearGradient>
          </defs>
          <rect width="512" height="512" rx="96" fill="url(#emerald-gradient)"/>
          <path d="M192 144v224l176-112z" fill="#050505"/>
        </svg>
      </body>
    </html>
  `);

  const favicon = await page.screenshot({ type: 'png' });
  writeFileSync(join(staticDir, 'favicon.png'), favicon);
  console.log('Generated favicon.png');

  await browser.close();
  console.log('Done!');
}

generateIcons();
