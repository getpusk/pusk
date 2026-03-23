// Pusk Service Worker — App Shell cache + Push notifications
const CACHE = 'pusk-v6';
const SHELL = [
  '/',
  '/css/pusk.css',
  '/js/state.js',
  '/js/storage.js',
  '/js/util.js',
  '/js/views.js',
  '/js/ws.js',
  '/js/actions.js',
  '/js/settings.js',
  '/js/landing.js',
  '/js/onboard.js',
  '/js/app.js',
  '/manifest.json',
  '/icon-192.png',
];

// 1. Install: precache app shell
self.addEventListener('install', e => {
  e.waitUntil(
    caches.open(CACHE)
      .then(c => c.addAll(SHELL))
      .then(() => self.skipWaiting())
  );
});

// 2. Activate: clean old caches
self.addEventListener('activate', e => {
  e.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

// 3. Fetch: cache-first for shell, network-only for API/WS
self.addEventListener('fetch', e => {
  const url = new URL(e.request.url);
  // Network-only for API, WebSocket, and bot/hook/file endpoints
  if (url.pathname.startsWith('/api/') ||
      url.pathname.startsWith('/bot/') ||
      url.pathname.startsWith('/hook/') ||
      url.pathname.startsWith('/file/') ||
      url.pathname === '/metrics') {
    return;
  }
  // Stale-while-revalidate for static assets
  e.respondWith(
    caches.match(e.request).then(cached => {
      const fetchPromise = fetch(e.request).then(resp => {
        if (resp.ok && e.request.method === 'GET') {
          caches.open(CACHE).then(c => c.put(e.request, resp.clone()));
        }
        return resp;
      }).catch(() => cached);
      return cached || fetchPromise;
    }).catch(() => {
      if (e.request.mode === 'navigate') return caches.match('/');
    })
  );
});

// 4. Push notifications
self.addEventListener('push', e => {
  const data = e.data ? e.data.json() : {};
  e.waitUntil(self.registration.showNotification(data.title || 'Pusk', {
    body: data.body || 'New message',
    icon: data.icon || '/icon-192.png',
    badge: '/icon-192.png',
    tag: data.tag || 'pusk-msg',
    data: { url: data.url || '/' },
  }));
});

// 5. Notification click → open/focus app
self.addEventListener('notificationclick', e => {
  e.notification.close();
  e.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then(list => {
      for (const c of list) {
        if (c.url.includes(self.location.origin) && 'focus' in c) return c.focus();
      }
      return clients.openWindow(e.notification.data.url || '/');
    })
  );
});
