// Pusk Service Worker — App Shell cache + Push notifications
const CACHE = 'pusk-v65';
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

// 1. Install: precache app shell, activate immediately
self.addEventListener('install', e => {
  e.waitUntil(
    caches.open(CACHE)
      .then(c => c.addAll(SHELL))
      .then(() => self.skipWaiting())
  );
});

// 2. Activate: clean old caches, claim clients
self.addEventListener('activate', e => {
  e.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

// 3. Fetch: network-first for JS/CSS, cache fallback for offline
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
  // JS and CSS: network-first (always fresh code, cache as offline fallback)
  if (url.pathname.endsWith('.js') || url.pathname.endsWith('.css')) {
    e.respondWith(
      fetch(e.request).then(resp => {
        if (resp.ok) {
          const clone = resp.clone();
          caches.open(CACHE).then(c => c.put(e.request, clone));
        }
        return resp;
      }).catch(() => caches.match(e.request))
    );
    return;
  }
  // Everything else: stale-while-revalidate
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
    vibrate: [200, 100, 200],
    renotify: true,
    requireInteraction: true,
    data: { url: data.url || '/' },
  }));
});

// 5. Notification click → open/focus app
self.addEventListener('notificationclick', e => {
  e.notification.close();
  const target = e.notification.data?.url || '/';
  e.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then(list => {
      for (const c of list) {
        if (c.url.includes(self.location.origin) && 'focus' in c) {
          c.postMessage({ type: 'push-navigate', url: target });
          return c.focus();
        }
      }
      return clients.openWindow(target);
    })
  );
});
