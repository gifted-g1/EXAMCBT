/** Error thrown for any non-2xx API response. `code` mirrors the server's
 * machine-readable error (LAN_REQUIRED, EXAM_NOT_OPEN, PIN_REQUIRED, ...). */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details: unknown

  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
  }
}

/** True when the request never reached the server (offline, DNS, reset). */
export const isNetworkError = (e: unknown): boolean => e instanceof TypeError

export interface RequestOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
  signal?: AbortSignal
  /** Abort after this many ms (default 20 s) so a dead network never hangs a save. */
  timeoutMs?: number
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json', ...opts.headers }
  let body: BodyInit | undefined
  if (opts.body instanceof FormData || opts.body instanceof Blob || opts.body instanceof ArrayBuffer) {
    body = opts.body as BodyInit
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), opts.timeoutMs ?? 20_000)
  opts.signal?.addEventListener('abort', () => ctrl.abort())
  let res: Response
  try {
    res = await fetch(path, {
      method: opts.method ?? (body ? 'POST' : 'GET'),
      headers,
      body,
      credentials: 'same-origin',
      signal: ctrl.signal,
    })
  } catch (e) {
    // Timeouts and aborts are network failures for every caller.
    throw e instanceof TypeError ? e : new TypeError(`network error: ${(e as Error)?.name ?? 'unknown'}`)
  } finally {
    clearTimeout(timer)
  }
  if (res.status === 204 || res.status === 202) return undefined as T
  const text = await res.text()
  let data: unknown = undefined
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }
  if (!res.ok) {
    const d = (data ?? {}) as { error?: string; message?: string; details?: unknown }
    throw new ApiError(res.status, d.error ?? `HTTP_${res.status}`, d.message ?? res.statusText, d.details)
  }
  return data as T
}
