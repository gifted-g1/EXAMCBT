import { AnswerQueue } from './offlineQueue'
import { deviceId, fnv1a } from './device'
import { floatToPCM16, resampleTo16k } from './media'

describe('deviceId', () => {
  it('is stable and matches the server format', () => {
    const id = deviceId()
    expect(id).toMatch(/^[A-Za-z0-9_-]{16,128}$/)
    expect(deviceId()).toBe(id)
    expect(fnv1a('abc')).toBe('1a47e90b')
  })
})

describe('AnswerQueue', () => {
  it('keeps only the latest answer per question and retries after failure', async () => {
    vi.useFakeTimers()
    let fail = true
    const saved: [number, string][] = []
    const q = new AnswerQueue(async (id, r) => {
      if (fail) throw new TypeError('offline')
      saved.push([id, r])
    })
    q.enqueue(1, 'a')
    q.enqueue(1, 'b')
    await vi.advanceTimersByTimeAsync(0)
    expect(q.size).toBe(1)
    expect(JSON.parse(localStorage.getItem('es_pending_answers')!)).toEqual({ '1': 'b' })

    fail = false
    await vi.advanceTimersByTimeAsync(5000)
    expect(saved).toEqual([[1, 'b']])
    expect(q.size).toBe(0)
    q.dispose()
    vi.useRealTimers()
  })

  it('drops answers the server rejects permanently', async () => {
    const q = new AnswerQueue(
      async () => {
        throw new Error('gone')
      },
      () => true,
    )
    q.enqueue(5, 'x')
    await q.flush()
    expect(q.size).toBe(0)
  })
})

describe('audio helpers', () => {
  it('converts float samples to little-endian PCM16', () => {
    const buf = floatToPCM16(new Float32Array([0, 1, -1, 2]))
    const v = new DataView(buf)
    expect(v.getInt16(0, true)).toBe(0)
    expect(v.getInt16(2, true)).toBe(32767)
    expect(v.getInt16(4, true)).toBe(-32768)
    expect(v.getInt16(6, true)).toBe(32767)
  })
  it('resamples 48 kHz to 16 kHz', () => {
    expect(resampleTo16k(new Float32Array(48000), 48000)).toHaveLength(16000)
  })
})

describe('AnswerQueue isolation', () => {
  it('does not hand one session\'s pending answers to the next session', async () => {
    localStorage.setItem('es_pending_answers:1', JSON.stringify({ '7': 'A-answer' }))
    const saved: string[] = []
    const q = new AnswerQueue(async (_id, r) => {
      saved.push(r)
    }, undefined, 'es_pending_answers:2')
    await q.flush()
    expect(saved).toEqual([])
    expect(q.size).toBe(0)
  })
})
