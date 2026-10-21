/*
 * Exam Shield service worker: keeps the app shell (HTML, JS, CSS) available
 * so the exam page can reopen while the exam server is unreachable — after a
 * Wi-Fi drop, or while the server restarts after a power cut. Exam data and
 * answers are NOT cached here: the page keeps those in localStorage and syncs
 * them through the API when the connection returns.
 *
 * Strategy: navigations are network-first (always fresh when online) with the
 * cached shell as fallback; hashed /assets/ files are cache-first. /api and
 * /healthz are never touched.
 */
const CACHE = 'es-shell-v1'

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((c) => c.addAll(['/', '/exam']))
      .catch(() => undefined)
      .then(() => self.skipWaiting()),
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('fetch', (event) => {
  const req = event.request
  if (req.method !== 'GET') return
  const url = new URL(req.url)
  if (url.origin !== self.location.origin) return
  if (url.pathname.startsWith('/api/') || url.pathname === '/healthz' || url.pathname === '/sw.js') return

  if (req.mode === 'navigate') {
    event.respondWith(
      fetch(req)
        .then((res) => {
          if (res.ok) {
            const copy = res.clone()
            caches.open(CACHE).then((c) => c.put(url.pathname === '/' ? '/' : '/exam', copy))
          }
          return res
        })
        .catch(() =>
          caches
            .match(req)
            .then((hit) => hit || caches.match('/exam'))
            .then((hit) => hit || caches.match('/')),
        ),
    )
    return
  }

  if (url.pathname.startsWith('/assets/')) {
    event.respondWith(
      caches.match(req).then(
        (hit) =>
          hit ||
          fetch(req).then((res) => {
            if (res.ok) {
              const copy = res.clone()
              caches.open(CACHE).then((c) => c.put(req, copy))
            }
            return res
          }),
      ),
    )
  }
})
