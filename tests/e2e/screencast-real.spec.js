// Pusk README screencast — real mode, pre-filled data via webhook API
// Shows: login screen → login → channels with formatted alerts → ACK → settings
const { test } = require('@playwright/test');

const BASE = 'http://localhost:18888';
const SLUG = 'acme-ops';
const USER = 'admin';
const PIN = 'admin1234';
const BOT_TOKEN = 'prometheus-bot-12345';
const BOT2_TOKEN = 'zabbix-bot-67890';

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

  // ── Setup everything via API before UI starts ──
  const reg = await api('POST', '/api/org/register', {
    slug: SLUG, name: 'Acme Ops', username: USER, pin: PIN
  });
  const token = reg.data.token;

  // Create bots
  await api('POST', '/admin/bots', { token: BOT_TOKEN, name: 'Prometheus' }, token);
  await api('POST', '/admin/bots', { token: BOT2_TOKEN, name: 'Zabbix' }, token);

  // Send alerts via webhook — channels auto-created by webhook handler
  // Alertmanager format → #alerts (via Prometheus bot)
  await api('POST', `/hook/${BOT_TOKEN}?format=alertmanager&channel=alerts`, {
    status: 'firing',
    alerts: [{
      status: 'firing',
      labels: { alertname: 'HighCPU', severity: 'critical', instance: 'prod-web-01:9090' },
      annotations: { summary: 'CPU usage above 95%', description: 'Production web server CPU is critically high for 5 minutes' }
    }]
  });
  await new Promise(r => setTimeout(r, 200));

  await api('POST', `/hook/${BOT_TOKEN}?format=alertmanager&channel=alerts`, {
    status: 'firing',
    alerts: [{
      status: 'firing',
      labels: { alertname: 'PodCrashLoop', severity: 'warning', instance: 'k8s-node-03' },
      annotations: { summary: 'Pod api-gateway restarting', description: 'CrashLoopBackOff: 5 restarts in 10 minutes' }
    }]
  });
  await new Promise(r => setTimeout(r, 200));

  // Zabbix format → #infra (via Zabbix bot)
  await api('POST', `/hook/${BOT2_TOKEN}?format=zabbix&channel=infra`, {
    subject: 'Disk full on db-master-01',
    message: 'Disk /data usage is 94%, threshold 90%',
    severity: 'High',
    status: 'PROBLEM',
    host: 'db-master-01'
  });
  await new Promise(r => setTimeout(r, 200));

  // Plain text → #deploy via sendChannel (Prometheus bot)
  await api('POST', `/bot/${BOT_TOKEN}/sendChannel`, {
    channel: 'deploy',
    text: 'Deploy api-gateway v2.14.1 to production — 3/3 pods ready'
  });
  await new Promise(r => setTimeout(r, 300));

  // Subscribe to all auto-created channels
  const chList = await api('GET', '/api/channels', null, token);
  for (const ch of (chList.data || [])) {
    await api('POST', `/api/channels/${ch.id}/subscribe`, {}, token);
  }

  // ── UI flow: skip landing, go straight to login ──

  await page.goto(BASE);
  await page.waitForTimeout(800);

  // Click Login to skip landing page
  await page.click('#land-login');
  await page.waitForSelector('#auth', { state: 'visible', timeout: 5000 });
  await page.waitForTimeout(800);

  // Fill credentials
  await page.type('#a-org', SLUG, { delay: 45 });
  await page.waitForTimeout(200);
  await page.type('#a-user', USER, { delay: 45 });
  await page.waitForTimeout(200);
  await page.type('#a-pin', PIN, { delay: 45 });
  await page.waitForTimeout(400);

  // Login
  await page.click('#btn-login');
  await page.waitForSelector('#app', { state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // Channels should have unread badges
  await page.waitForSelector('.ch-row', { timeout: 5000 });
  await page.waitForTimeout(1500);

  // Open #alerts — formatted alertmanager alerts with ACK buttons
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
    await page.waitForTimeout(800);
  }

  // Open #infra — Zabbix formatted alert
  const infraRow = page.locator('.ch-row', { hasText: 'infra' }).first();
  if (await infraRow.isVisible({ timeout: 3000 }).catch(() => false)) {
    await infraRow.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(2000);
    await page.click('#hdr-back');
    await page.waitForTimeout(800);
  }

  // Open #deploy
  const deployRow = page.locator('.ch-row', { hasText: 'deploy' }).first();
  if (await deployRow.isVisible({ timeout: 3000 }).catch(() => false)) {
    await deployRow.click();
    await page.waitForSelector('#chat-view', { state: 'visible', timeout: 5000 });
    await page.waitForTimeout(1500);
    await page.click('#hdr-back');
    await page.waitForTimeout(800);
  }

  // Settings
  await page.click('#hdr-ava');
  await page.waitForTimeout(2000);
  await page.click('#settings-bg');
  await page.waitForTimeout(1000);
});
