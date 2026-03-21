const { test } = require('@playwright/test');
const BASE = 'https://getpusk.ru';

test('Record Pusk demo screencast', async ({ page }) => {
  // Set viewport for nice recording
  await page.setViewportSize({ width: 1280, height: 720 });

  // Start recording
  await page.video().path();

  // 1. Landing page
  await page.goto(BASE);
  await page.waitForTimeout(2000);

  // 2. Click "Try Demo"
  await page.click('#land-demo');
  await page.waitForSelector('#app', { state: 'visible', timeout: 10000 });
  await page.waitForTimeout(1500);

  // 3. Show channels with unread badges
  await page.waitForSelector('.ch-row', { timeout: 5000 });
  await page.waitForTimeout(1500);

  // 4. Open #alerts channel
  const alerts = page.locator('.ch-row', { hasText: 'alerts' }).first();
  if (await alerts.isVisible()) {
    await alerts.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(2000);

    // 5. Click ACK button if visible
    const ackBtn = page.locator('.m-kb-btn', { hasText: 'ACK' }).first();
    if (await ackBtn.isVisible()) {
      await ackBtn.click();
      await page.waitForTimeout(1500);
    }

    // 6. Go back
    await page.click('#hdr-back');
    await page.waitForTimeout(1000);
  }

  // 7. Open DemoBot
  const demoBot = page.locator('.bot-row', { hasText: 'DemoBot' }).first();
  if (await demoBot.isVisible()) {
    await demoBot.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(2000);

    // 8. Click an inline button
    const featBtn = page.locator('.m-kb-btn').first();
    if (await featBtn.isVisible()) {
      await featBtn.click();
      await page.waitForTimeout(2000);
    }

    // 9. Go back
    await page.click('#hdr-back');
    await page.waitForTimeout(1000);
  }

  // 10. Open Settings
  await page.click('#hdr-ava');
  await page.waitForTimeout(2000);

  // 11. Close Settings
  await page.click('#settings-bg');
  await page.waitForTimeout(1000);

  // Final pause
  await page.waitForTimeout(1000);
});
