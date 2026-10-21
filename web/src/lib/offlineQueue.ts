/**
 * Answer save queue that survives network drops, page reloads, browser
 * crashes and power cuts.
 *
 * Every answer is written to localStorage (on disk) before any network
 * request is made, and removed only after the server confirms it. Only the
 * latest response per question matters, so the queue is a map keyed by
 * question id. flush() retries with exponential back-off; callers learn the
 * pending count through subscribe().
 *
 * The storage key must be unique per exam session, so that on a shared lab PC
 * a later student never inherits (and submits) a previous student's answers.
 */
type Saver = (questionId: number, response: string) => Promise<unknown>

function storage(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null // storage blocked: the queue still works in memory
  }
}

export class AnswerQueue {
  private pending = new Map<number, string>()
  private flushing = false
  private timer: ReturnType<typeof setTimeout> | null = null
  private delay = 1000
  private listeners = new Set<(n: number) => void>()

  constructor(
    private readonly save: Saver,
    private readonly isFatal: (e: unknown) => boolean = () => false,
    private readonly storageKey = 'es_pending_answers',
  ) {
    try {
      const raw = storage()?.getItem(this.storageKey)
      if (raw) for (const [k, v] of Object.entries(JSON.parse(raw) as Record<string, string>)) this.pending.set(Number(k), v)
    } catch {
      /* ignore corrupt storage */
    }
  }

  subscribe(fn: (n: number) => void): () => void {
    this.listeners.add(fn)
    fn(this.pending.size)
    return () => {
      this.listeners.delete(fn)
    }
  }

  get size(): number {
    return this.pending.size
  }

  /** Answers not yet confirmed by the server (e.g. to restore after a reload). */
  entries(): [number, string][] {
    return Array.from(this.pending)
  }

  /** Save to disk now and send to the server. */
  enqueue(questionId: number, response: string): void {
    this.stash(questionId, response)
    void this.flush()
  }

  /** Save to disk only (every keystroke); a later enqueue/flush sends it. */
  stash(questionId: number, response: string): void {
    this.pending.set(questionId, response)
    this.persist()
  }

  async flush(): Promise<void> {
    if (this.flushing) return
    this.flushing = true
    try {
      for (const [qid, resp] of Array.from(this.pending)) {
        try {
          await this.save(qid, resp)
          // Only drop it if it wasn't changed while saving.
          if (this.pending.get(qid) === resp) this.pending.delete(qid)
          this.delay = 1000
        } catch (e) {
          if (this.isFatal(e)) {
            this.pending.delete(qid)
            continue
          }
          this.schedule()
          break
        }
      }
    } finally {
      this.flushing = false
      this.persist()
    }
  }

  /** Forget everything (after a confirmed submit). */
  clear(): void {
    this.pending.clear()
    if (this.timer) clearTimeout(this.timer)
    this.timer = null
    this.persist()
  }

  dispose(): void {
    if (this.timer) clearTimeout(this.timer)
    this.timer = null
    this.listeners.clear()
  }

  private schedule() {
    if (this.timer) return
    this.timer = setTimeout(() => {
      this.timer = null
      void this.flush()
    }, this.delay)
    this.delay = Math.min(this.delay * 2, 15000)
  }

  private persist() {
    try {
      const s = storage()
      if (this.pending.size === 0) s?.removeItem(this.storageKey)
      else s?.setItem(this.storageKey, JSON.stringify(Object.fromEntries(this.pending)))
    } catch {
      /* quota or blocked storage: keep going in memory */
    }
    this.listeners.forEach((fn) => fn(this.pending.size))
  }
}
