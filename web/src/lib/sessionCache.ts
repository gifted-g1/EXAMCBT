import type { SessionState } from '../api/types'

/**
 * A local copy of the exam in progress (questions, saved answers, end time,
 * settings) so the exam can reopen after a page reload, browser crash or
 * power cut even while the exam server is unreachable. Answers typed while
 * offline live in the AnswerQueue and are sent when the connection returns.
 *
 * The copy never contains correct answers (the server never sends them) and
 * is deleted as soon as the exam is submitted.
 */
const KEY = 'es_exam_cache'

export interface CachedExam {
  state: SessionState
  offsetMs: number
  savedAt: number
}

function store(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function saveExamCache(state: SessionState, offsetMs: number): void {
  try {
    store()?.setItem(KEY, JSON.stringify({ state, offsetMs, savedAt: Date.now() } satisfies CachedExam))
  } catch {
    /* quota: the server copy is still authoritative */
  }
}

/** Returns the cached exam if it is still running (with a little grace). */
export function loadExamCache(): CachedExam | null {
  try {
    const raw = store()?.getItem(KEY)
    if (!raw) return null
    const c = JSON.parse(raw) as CachedExam
    if (!c?.state?.questions || c.state.status !== 'ACTIVE') return null
    const ends = new Date(c.state.ends_at).getTime()
    if (Date.now() + c.offsetMs > ends + 10 * 60_000) {
      clearExamCache()
      return null
    }
    return c
  } catch {
    return null
  }
}

export function clearExamCache(): void {
  try {
    store()?.removeItem(KEY)
  } catch {
    /* ignore */
  }
}

export const pendingKey = (sessionId: number) => `es_pending_answers:${sessionId}`
