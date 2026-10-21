import { request } from '../api/client'
import type { SessionState } from '../api/types'
import { AnswerQueue } from './offlineQueue'
import { clearExamCache, loadExamCache, saveExamCache } from './sessionCache'

const state = (endsInMs: number): SessionState => ({
  session_id: 7,
  status: 'ACTIVE',
  exam_id: 1,
  exam_title: 'Networks',
  ends_at: new Date(Date.now() + endsInMs).toISOString(),
  server_time: new Date().toISOString(),
  student: { matric_no: 'IFT/22/9203', full_name: 'A' },
  questions: [{ id: 1, position: 1, kind: 'TEXT', prompt: 'Q', marks: 1 }],
  answers: {},
  proctoring: {
    face_verification: false,
    camera_monitoring: false,
    audio_monitoring: false,
    fullscreen_required: false,
    tab_switch_detection: false,
    clipboard_blocking: false,
  },
})

describe('power cut / browser crash', () => {
  it('unsent answers survive a restart: a new queue on the same key still has them', async () => {
    const offline = async () => {
      throw new TypeError('offline')
    }
    const q1 = new AnswerQueue(offline, undefined, 'es_pending_answers:7')
    q1.stash(1, 'half-typed essay')
    q1.enqueue(2, '1')
    await q1.flush()
    q1.dispose() // the tab dies here

    const sent: [number, string][] = []
    const q2 = new AnswerQueue(async (id, r) => {
      sent.push([id, r])
    }, undefined, 'es_pending_answers:7')
    expect(q2.entries()).toEqual([
      [1, 'half-typed essay'],
      [2, '1'],
    ])
    await q2.flush()
    expect(sent).toHaveLength(2)
    expect(localStorage.getItem('es_pending_answers:7')).toBeNull()
  })

  it('stash writes to disk without sending; clear() forgets everything', () => {
    const save = vi.fn()
    const q = new AnswerQueue(save, undefined, 'k')
    q.stash(3, 'draft')
    expect(save).not.toHaveBeenCalled()
    expect(JSON.parse(localStorage.getItem('k')!)).toEqual({ '3': 'draft' })
    q.clear()
    expect(localStorage.getItem('k')).toBeNull()
  })

  it('keeps a local copy of the running exam and drops it once over', () => {
    saveExamCache(state(60_000), 0)
    expect(loadExamCache()?.state.exam_title).toBe('Networks')
    clearExamCache()
    expect(loadExamCache()).toBeNull()

    saveExamCache(state(-20 * 60_000), 0) // ended 20 minutes ago
    expect(loadExamCache()).toBeNull()
  })
})

describe('weak network', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('times out a hanging request as a network error instead of waiting forever', async () => {
    vi.stubGlobal(
      'fetch',
      (_: string, init: RequestInit) =>
        new Promise((_res, rej) => init.signal?.addEventListener('abort', () => rej(new DOMException('aborted', 'AbortError')))),
    )
    await expect(request('/api/x', { timeoutMs: 50 })).rejects.toBeInstanceOf(TypeError)
  })
})
