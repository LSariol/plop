// Service Worker for Plop PWA
// Registered at /sw.js (root scope) via a dedicated route in routes.go

const CACHE_NAME = 'plop-__ASSET_HASH__';

const PRECACHE = [
    '/static/style.css',
    '/static/app.js',
    '/static/auth.js',
    '/static/manifest.json',
    '/static/favicon.svg',
    '/static/icons/icon-192.png',
];

// Install — precache app shell
self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE))
    );
    self.skipWaiting();
});

// Activate — evict old caches
self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys().then((keys) =>
            Promise.all(
                keys
                    .filter((k) => k !== CACHE_NAME)
                    .map((k) => caches.delete(k))
            )
        )
    );
    self.clients.claim();
});

// Fetch strategy:
//   POST       → network only (uploads must reach the server)
//   navigate   → network first so server session checks and redirects work
//   GET static → cache-first, falling back to network; cache successful responses
self.addEventListener('fetch', (event) => {
    if (event.request.mode === 'navigate') {
        event.respondWith(
            fetch(event.request).catch(() =>
                caches.match('/static/app.js')
                    .then(() => caches.match(event.request))
                    .catch(() => new Response('Offline', { status: 503 }))
            )
        );
        return;
    }

    if (event.request.method !== 'GET') {
        event.respondWith(
            fetch(event.request).catch(() =>
                new Response(
                    JSON.stringify({ error: 'Offline — cannot reach server' }),
                    { status: 503, headers: { 'Content-Type': 'application/json' } }
                )
            )
        );
        return;
    }

    event.respondWith(
        caches.match(event.request).then((cached) => {
            if (cached) return cached;

            return fetch(event.request).then((response) => {
                if (!response || response.status !== 200 || response.type === 'opaque') {
                    return response;
                }
                const clone = response.clone();
                caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
                return response;
            }).catch(() => {
                // Offline fallback for HTML navigation requests
                if (event.request.headers.get('accept')?.includes('text/html')) {
                    return caches.match('/app');
                }
                return new Response('Offline', { status: 503 });
            });
        })
    );
});

// Background sync — retry pending sends
self.addEventListener('sync', (event) => {
    if (event.tag === 'sync-send') {
        event.waitUntil(syncPendingMessages());
    }
});

async function syncPendingMessages() {
    try {
        const db = await openDatabase();
        const pending = await getPendingMessages(db);
        for (const msg of pending) {
            const res = await fetch('/upload', { method: 'POST', body: msg.data });
            if (res.ok) await removePendingMessage(db, msg.id);
        }
    } catch (err) {
        console.error('Background sync failed:', err);
        throw err; // re-throw so the browser retries
    }
}

// Notification click — open or focus the app
self.addEventListener('notificationclick', (event) => {
    event.notification.close();
    event.waitUntil(
        clients.matchAll({ type: 'window' }).then((list) => {
            for (const client of list) {
                if ('focus' in client) return client.focus();
            }
            if (clients.openWindow) return clients.openWindow('/app');
        })
    );
});

// ─── IndexedDB helpers for offline queue ─────────────────────────────────

function openDatabase() {
    return new Promise((resolve, reject) => {
        const req = indexedDB.open('plop-db', 1);
        req.onerror = () => reject(req.error);
        req.onsuccess = () => resolve(req.result);
        req.onupgradeneeded = (e) => {
            const db = e.target.result;
            if (!db.objectStoreNames.contains('pending-messages')) {
                db.createObjectStore('pending-messages', { keyPath: 'id', autoIncrement: true });
            }
        };
    });
}

function getPendingMessages(db) {
    return new Promise((resolve, reject) => {
        const tx = db.transaction('pending-messages', 'readonly');
        const req = tx.objectStore('pending-messages').getAll();
        req.onerror = () => reject(req.error);
        req.onsuccess = () => resolve(req.result);
    });
}

function removePendingMessage(db, id) {
    return new Promise((resolve, reject) => {
        const tx = db.transaction('pending-messages', 'readwrite');
        const req = tx.objectStore('pending-messages').delete(id);
        req.onerror = () => reject(req.error);
        req.onsuccess = () => resolve();
    });
}
