const { test, expect } = require('@playwright/test');

const BASE = 'https://getpusk.ru';

test.describe('ARTAIS Deploy Bot — full button flow', () => {

  test('login → open bot → /start → click Status → verify edit', async ({ page }) => {
    // 1. Go to landing, click Login
    await page.goto(BASE);
    await page.click('#land-login');
    await page.waitForSelector('#auth', { state: 'visible' });

    // 2. Login as test1
    await page.fill('#a-user', 'test1');
    await page.fill('#a-pin', 'test1');
    await page.fill('#a-org', 'test1');
    await page.click('#btn-login');
    await page.waitForSelector('#app', { state: 'visible', timeout: 10000 });
    console.log('Logged in');

    // 3. Wait for bot list, find ARTAIS Deploy
    await page.waitForSelector('.bot-row', { timeout: 10000 });
    const bots = await page.locator('.bot-row').all();
    console.log('Bots found:', bots.length);

    // Find ARTAIS Deploy bot
    let artaisBot = null;
    for (const bot of bots) {
      const name = await bot.locator('.bot-name').textContent();
      console.log('Bot:', name);
      if (name.includes('ARTAIS')) {
        artaisBot = bot;
        break;
      }
    }
    expect(artaisBot).not.toBeNull();

    // 4. Click on ARTAIS Deploy
    await artaisBot.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 10000 });
    console.log('Chat opened');

    // 5. Type /start and send
    await page.fill('#msg-in', '/start');
    await page.click('#msg-send');
    console.log('/start sent');

    // 6. Wait for bot response with buttons
    await page.waitForFunction(() => {
      const msgs = document.querySelectorAll('.m-kb-btn');
      return msgs.length > 0;
    }, { timeout: 15000 });
    console.log('Buttons appeared!');

    // 7. Count buttons
    const buttons = await page.locator('.m-kb-btn').all();
    console.log('Button count:', buttons.length);
    for (const btn of buttons) {
      const text = await btn.textContent();
      console.log('  Button:', text);
    }

    // 8. Find and click Status button
    const statusBtn = page.locator('.m-kb-btn', { hasText: 'Status' }).first();
    await expect(statusBtn).toBeVisible();

    // Get message text BEFORE click
    const msgsBefore = await page.locator('.m-text').last();
    const textBefore = await msgsBefore.textContent();
    console.log('Text before click:', textBefore.substring(0, 50));

    // Click Status
    await statusBtn.click();
    console.log('Status clicked');

    // 9. Wait for message update (editMessageText via WS)
    try {
      await page.waitForFunction((before) => {
        const texts = document.querySelectorAll('.m-text');
        if (texts.length === 0) return false;
        const last = texts[texts.length - 1].textContent;
        return last !== before && last.includes('Status');
      }, textBefore, { timeout: 15000 });
      console.log('Message UPDATED via editMessageText!');
    } catch (e) {
      // Check what's in the chat now
      const allTexts = await page.locator('.m-text').allTextContents();
      console.log('Timeout - current messages:', allTexts.map(t => t.substring(0, 40)));

      // Check console for WS events
      const logs = [];
      page.on('console', msg => logs.push(msg.text()));
      await page.waitForTimeout(3000);
      console.log('Console logs:', logs.filter(l => l.includes('[ws]')));
      throw e;
    }

    // 10. Verify updated content
    const updatedText = await page.locator('.m-text').last().textContent();
    console.log('Updated text:', updatedText.substring(0, 80));
    expect(updatedText).toContain('Status');

    // 11. Check for Back button
    const backBtn = page.locator('.m-kb-btn', { hasText: 'Back' });
    await expect(backBtn.first()).toBeVisible({ timeout: 5000 });
    console.log('Back button visible');

    // 12. Click Back
    await backBtn.first().click();
    console.log('Back clicked');

    await page.waitForFunction(() => {
      const texts = document.querySelectorAll('.m-text');
      if (texts.length === 0) return false;
      const last = texts[texts.length - 1].textContent;
      return last.includes('my-portfolio') || last.includes('Welcome');
    }, null, { timeout: 15000 });
    console.log('Back to menu - PASS');

    const finalText = await page.locator('.m-text').last().textContent();
    console.log('Final text:', finalText.substring(0, 80));
  });
});
