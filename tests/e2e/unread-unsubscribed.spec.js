// Pusk E2E — Issue #101: Unread messages for unsubscribed channels
const { test, expect } = require('@playwright/test');

const BASE = process.env.PUSK_URL || process.env.BASE_URL || 'http://localhost:8443';

async function api(method, path, body, token) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (token) opts.headers.Authorization = token;
  if (body) opts.body = JSON.stringify(body);
  const r = await fetch(BASE + path, opts);
  return { status: r.status, data: await r.json().catch(() => null) };
}

test.describe('Unread badge for unsubscribed channels (#101)', () => {
  let adminToken, user2Token, orgSlug, generalId;

  test.beforeAll(async () => {
    orgSlug = 'unread-' + Date.now();

    // Create org with admin
    const reg = await api('POST', '/api/org/register', {
      slug: orgSlug, name: 'Unread Test', username: 'admin', pin: 'admin12345'
    });
    expect(reg.status).toBe(200);
    adminToken = reg.data.token;

    // Create invite for user2
    const inv = await api('POST', '/api/invite', null, adminToken);
    expect(inv.data.code).toBeTruthy();

    // User2 joins via invite
    const join = await api('POST', '/api/invite/accept?org=' + orgSlug, {
      code: inv.data.code, username: 'user2', pin: 'user212345', display_name: 'User Two'
    });
    expect(join.status).toBe(200);
    user2Token = join.data.token;

    // Find general channel
    const chs = await api('GET', '/api/channels', null, adminToken);
    generalId = chs.data.find(c => c.name === 'general')?.id;
    expect(generalId).toBeTruthy();

    // User2 subscribes to general
    await api('POST', `/api/channels/${generalId}/subscribe`, null, user2Token);

    // User2 opens channel (mark-read)
    await api('POST', `/api/channels/${generalId}/mark-read`, null, user2Token);

    // User2 unsubscribes
    await api('POST', `/api/channels/${generalId}/unsubscribe`, null, user2Token);
  });

  test('unsubscribed channel shows unread count via API', async () => {
    // Admin sends a message
    const send = await api('POST', `/api/channels/${generalId}/send`, {
      text: 'alert from admin'
    }, adminToken);
    expect(send.status).toBe(200);

    // User2 checks channels — should see unread > 0
    const chs = await api('GET', '/api/channels', null, user2Token);
    expect(chs.status).toBe(200);
    const ch = chs.data.find(c => c.id === generalId);
    expect(ch).toBeTruthy();
    expect(ch.subscribed).toBe(false);
    expect(ch.unread).toBeGreaterThan(0);
  });

  test('opening channel clears unread (mark-read)', async () => {
    // User2 opens channel and marks read
    await api('POST', `/api/channels/${generalId}/mark-read`, null, user2Token);

    // Check unread is now 0
    const chs = await api('GET', '/api/channels', null, user2Token);
    const ch = chs.data.find(c => c.id === generalId);
    expect(ch.unread).toBe(0);
  });

  test('unread persists across API calls (no session dependency)', async () => {
    // Admin sends 2 more messages
    await api('POST', `/api/channels/${generalId}/send`, { text: 'msg1' }, adminToken);
    await api('POST', `/api/channels/${generalId}/send`, { text: 'msg2' }, adminToken);

    // User2 checks — first call
    const chs1 = await api('GET', '/api/channels', null, user2Token);
    const unread1 = chs1.data.find(c => c.id === generalId)?.unread;

    // User2 checks — second call (simulates refresh)
    const chs2 = await api('GET', '/api/channels', null, user2Token);
    const unread2 = chs2.data.find(c => c.id === generalId)?.unread;

    expect(unread1).toBe(unread2);
    expect(unread1).toBeGreaterThanOrEqual(2);
  });
});
