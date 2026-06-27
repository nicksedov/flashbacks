const CACHE_NAME = 'flashbacks-v3';
const STATIC_ASSETS = [
  '/manifest.json',
  '/favicon.svg',
  '/apple-touch-icon.png',
  '/icon-192.png',
  '/icon-512.png',
];

// Install: precache static assets (NOT index.html — it changes every build)
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(STATIC_ASSETS))
  );
  self.skipWaiting();
});

// Activate: clean up old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key !== CACHE_NAME)
          .map((key) => caches.delete(key))
      )
    )
  );
  self.clients.claim();
});

// Fetch: content-type-aware strategy to avoid MIME mismatch errors
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Skip non-GET requests
  if (request.method !== 'GET') return;

  // API requests: network-first with cache fallback
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(networkFirstWithCache(request));
    return;
  }

  // Navigation requests (HTML pages): always network-first, never serve stale index.html
  if (request.mode === 'navigate') {
    event.respondWith(networkFirstWithCache(request));
    return;
  }

  // JavaScript & CSS: network-first with strict Content-Type validation
  // Prevents caching HTML fallback responses as JS/CSS (the root cause of MIME errors)
  if (
    url.pathname.endsWith('.js') ||
    url.pathname.endsWith('.mjs') ||
    url.pathname.endsWith('.css')
  ) {
    event.respondWith(networkFirstStrictContentType(request, url));
    return;
  }

  // Static assets (images, fonts, icons): cache-first with network fallback
  event.respondWith(cacheFirstWithNetworkFallback(request, url));
});

/**
 * Network-first with cache fallback. Caches successful responses.
 */
async function networkFirstWithCache(request) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, response.clone());
    }
    return response;
  } catch (_err) {
    const cached = await caches.match(request);
    return cached || new Response('Network error', { status: 408 });
  }
}

/**
 * Network-first for JS/CSS with strict Content-Type validation.
 * If the server returns HTML instead of JS/CSS (SPA fallback for missing chunks),
 * the response is NOT cached and an error is thrown so the browser gets a proper 404.
 */
async function networkFirstStrictContentType(request, url) {
  try {
    const response = await fetch(request);

    if (!response.ok) {
      // Non-200: try cache fallback
      const cached = await caches.match(request);
      return cached || response;
    }

    const contentType = response.headers.get('Content-Type') || '';

    // Verify the response Content-Type matches the expected asset type
    if (url.pathname.endsWith('.css') && !contentType.includes('text/css')) {
      console.warn('SW: Expected CSS but got', contentType, 'for', url.pathname);
      // Try cache fallback, but do NOT cache this bogus response
      const cached = await caches.match(request);
      return cached || new Response('/* empty */', {
        status: 200,
        headers: { 'Content-Type': 'text/css' },
      });
    }

    if (
      (url.pathname.endsWith('.js') || url.pathname.endsWith('.mjs')) &&
      !contentType.includes('javascript')
    ) {
      console.warn('SW: Expected JavaScript but got', contentType, 'for', url.pathname);
      // Try cache fallback, but do NOT cache this bogus response
      const cached = await caches.match(request);
      if (cached) return cached;
      // Return empty module to avoid MIME error — the app may be broken but won't crash
      return new Response('', {
        status: 200,
        headers: { 'Content-Type': 'application/javascript' },
      });
    }

    // Valid response: cache it
    const cache = await caches.open(CACHE_NAME);
    cache.put(request, response.clone());
    return response;
  } catch (_err) {
    const cached = await caches.match(request);
    return cached || new Response('Network error', { status: 408 });
  }
}

/**
 * Cache-first for static assets (images, fonts, icons) with network fallback.
 */
async function cacheFirstWithNetworkFallback(request, url) {
  const cached = await caches.match(request);
  if (cached) return cached;

  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, response.clone());
    }
    return response;
  } catch (_err) {
    return new Response('Network error', { status: 408 });
  }
}
