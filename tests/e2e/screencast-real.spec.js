// Pusk README screencast — real mode (no demo)
// Shows: landing → create org → login → receive alert → ACK → settings
const { test } = require('@playwright/test');

const BASE = 'http://localhost:18888';
const SLUG = 'acme-ops';
const USER = 'admin';
const PIN = 'admin1234';
const BOT_TOKEN = 'prometheus-bot-token-12345';

async function api(method, path, body, token) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (token) opts.headers.Authorization = token;
  if (body) opts.body = JSON.stringify(body);
  const r = await fetch(`${BASE}${path}`, opts);
  return { status: r.status, data: await r.json().catch(() => null) };
}

test('Pusk screencast — real workflow', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.video().path();

  // ── 1. Landing page — show off the product ──
  await page.goto(BASE);
  await page.waitForTimeout(3000);

  // ── 2. Create organization via UI modal ──
  await page.click('#land-create-org');
  await page.waitForSelector('#org-modal-bg.open', { timeout: 5000 });
  await page.waitForTimeout(800);

  // Type org details with visible typing
  await page.type('#org-slug', SLUG, { delay: 60 });
  await page.waitForTimeout(400);
  await page.type('#org-name', 'Acme Ops', { delay: 60 });
  await page.waitForTimeout(400);
  await page.type('#org-user', USER, { delay: 60 });
  await page.waitForTimeout(400);
  await page.type('#org-pin', PIN, { delay: 60 });
  await page.waitForTimeout(600);

  // Submit
  await page.click('#org-ok');
  await page.waitForSelector('#app', { state: 'visible', timeout: 15000 });
  await page.waitForTimeout(2000);

  // ── 3. Now we're in the app. Create a bot and channels via API ──
  // Get token by logging in via API
  const auth = await api('POST', '/api/auth', {
    org: SLUG, username: USER, pin: PIN
  });
  const token = auth.data?.token;

  if (token) {
    // Create bot
    await api('POST', '/admin/bots', { token: BOT_TOKEN, name: 'Prometheus' }, token);

    // Create channels
    await api('POST', '/admin/channel', { name: 'alerts' }, token);
    await api('POST', '/admin/channel', { name: 'infra' }, token);

    // Get channel IDs
    const chList = await api('GET', '/api/channels', null, token);
    const alertsCh = chList.data?.find(c => c.name === 'alerts');
    const infraCh = chList.data?.find(c => c.name === 'infra');

    if (alertsCh) await api('POST', `/api/channels/${alertsCh.id}/subscribe`, {}, token);
    if (infraCh) await api('POST', `/api/channels/${infraCh.id}/subscribe`, {}, token);

    // Send alerts via bot API
    await api('POST', `/bot${BOT_TOKEN}/sendMessage`, {
      chat_id: '#alerts',
      text: JSON.stringify({
        status: 'firing',
        alerts: [{
          status: 'firing',
          labels: { alertname: 'HighCPU', severity: 'critical', instance: 'prod-web-01:9090' },
          annotations: { summary: 'CPU usage above 95%', description: 'Production web server CPU is critically high' }
        }]
      }),
      parse_mode: 'json'
    });

    await new Promise(r => setTimeout(r, 300));

    await api('POST', `/bot${BOT_TOKEN}/sendMessage`, {
      chat_id: '#infra',
      text: JSON.stringify({
        status: 'firing',
        alerts: [{
          status: 'firing',
          labels: { alertname: 'DiskFull', severity: 'warning', instance: 'db-master-01:9100' },
          annotations: { summary: 'Disk usage at 92%' }
        }]
      }),
      parse_mode: 'json'
    });

    await new Promise(r => setTimeout(r, 500));
  }

  // ── 4. Reload to see channels/bots ──
  await page.reload();
  await page.waitForSelector('#app', { state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // ── 5. Open #alerts channel ──
  const alertsRow = page.locator('.ch-row', { hasText: 'alerts' }).first();
  if (await alertsRow.isVisible({ timeout: 3000 }).catch(() => false)) {
    await alertsRow.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(2500);

    // ACK the alert
    const ackBtn = page.locator('.m-kb-btn', { hasText: 'ACK' }).first();
    if (await ackBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await ackBtn.click();
      await page.waitForTimeout(2000);
    }

    await page.click('#hdr-back');
    await page.waitForTimeout(1000);
  }

  // ── 6. Open #infra channel ──
  const infraRow = page.locator('.ch-row', { hasText: 'infra' }).first();
  if (await infraRow.isVisible({ timeout: 3000 }).catch(() => false)) {
    await infraRow.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(2000);
    await page.click('#hdr-back');
    await page.waitForTimeout(1000);
  }

  // ── 7. Open Prometheus bot ──
  const promBot = page.locator('.bot-row', { hasText: 'Prometheus' }).first();
  if (await promBot.isVisible({ timeout: 3000 }).catch(() => false)) {
    await promBot.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(2000);
    await page.click('#hdr-back');
    await page.waitForTimeout(1000);
  }

  // ── 8. Settings ──
  await page.click('#hdr-ava');
  await page.waitForTimeout(2500);
  await page.click('#settings-bg');
  await page.waitForTimeout(1500);
});
