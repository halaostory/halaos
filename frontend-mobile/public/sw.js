// HalaPeople mobile service worker: caches the app shell so the app opens
// offline and installs like a native app. API calls always go to the
// network — payslips and balances are never served stale from cache.
const CACHE = 'halapeople-shell-v1'
const SHELL = ['/m/', '/m/manifest.webmanifest', '/m/icon-192.png', '/m/icon-512.png']

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()))
})

self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))).then(() => self.clients.claim()))
})

self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url)
  if (e.request.method !== 'GET' || url.origin !== location.origin || url.pathname.startsWith('/api/')) return
  // Navigations: network first, fall back to the cached shell offline.
  if (e.request.mode === 'navigate') {
    e.respondWith(fetch(e.request).catch(() => caches.match('/m/')))
    return
  }
  // Static assets (hashed by Vite): cache first.
  if (url.pathname.startsWith('/m/assets/') || SHELL.includes(url.pathname)) {
    e.respondWith(caches.match(e.request).then((hit) => hit || fetch(e.request).then((res) => {
      if (res.ok) { const copy = res.clone(); caches.open(CACHE).then((c) => c.put(e.request, copy)) }
      return res
    })))
  }
})
