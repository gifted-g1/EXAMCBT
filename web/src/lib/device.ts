/**
 * Device identity for the session lock.
 *
 * A random per-browser id (crypto.getRandomValues works on plain-HTTP LAN
 * origins, unlike crypto.subtle) plus a short hash of stable browser traits.
 * The random part is what distinguishes identically imaged lab PCs; the trait
 * hash makes a copied id less useful on different hardware. It is a lock
 * key, not an authenticator — the server binds it to the session token.
 */
const KEY = 'es_device_id'

function randomPart(): string {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

/** 32-bit FNV-1a, hex. */
export function fnv1a(s: string): string {
  let h = 0x811c9dc5
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return (h >>> 0).toString(16).padStart(8, '0')
}

function traits(): string {
  const nav = typeof navigator !== 'undefined' ? navigator : ({} as Navigator)
  const scr = typeof screen !== 'undefined' ? screen : ({} as Screen)
  return [
    nav.userAgent,
    nav.language,
    nav.hardwareConcurrency,
    scr.width,
    scr.height,
    scr.colorDepth,
    Intl.DateTimeFormat().resolvedOptions().timeZone,
  ].join('|')
}

let cached: string | null = null

export function deviceId(): string {
  if (cached) return cached
  let id: string | null = null
  try {
    id = localStorage.getItem(KEY)
  } catch {
    /* storage blocked: fall back to an in-memory id for this tab */
  }
  if (!id || !/^[a-f0-9]{32}$/.test(id)) {
    id = randomPart()
    try {
      localStorage.setItem(KEY, id)
    } catch {
      /* ignore */
    }
  }
  cached = `${id}-${fnv1a(traits())}`
  return cached
}
