/**
 * Pubkey Quest service worker — the minimum needed for installability + a light
 * offline shell. Deliberately conservative about what it caches:
 *   - navigations   → network-first, fall back to the cached landing when offline
 *   - images/fonts  → cache-first (they rarely change; big win on repeat loads)
 *   - JS / CSS      → straight to network (versioned via ?v=, never serve stale code)
 *   - /api/*        → never intercepted (game state must always be live + authoritative)
 * Bump CACHE when this file changes to evict the old shell.
 */
const CACHE = 'pubkey-quest-v1';
const CORE = ['/', '/res/img/icon-192.png', '/res/img/favicon.ico'];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(CORE)).catch(() => {}));
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  const req = event.request;
  if (req.method !== 'GET') return;

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return; // leave cross-origin (CDNs, relays) alone

  if (req.mode === 'navigate') {
    event.respondWith(fetch(req).catch(() => caches.match('/')));
    return;
  }

  // Cache-first for static media only — never for JS/CSS (code) or /api (state).
  if (/\.(png|ico|gif|jpe?g|svg|webp|ttf|woff2?)$/.test(url.pathname)) {
    event.respondWith(
      caches.match(req).then((hit) =>
        hit ||
        fetch(req).then((res) => {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
          return res;
        }).catch(() => hit)
      )
    );
  }
  // Everything else falls through to the network (default handling).
});
