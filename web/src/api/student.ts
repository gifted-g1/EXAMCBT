import { request } from './client'
import { deviceId } from '../lib/device'
import type { GateInfo, Proctoring, SessionState } from './types'

const base = '/api/student'

function headers(pin?: string): Record<string, string> {
  const h: Record<string, string> = { 'X-Device-FP': deviceId() }
  if (pin) h['X-Exam-PIN'] = pin
  return h
}

export interface VerifyResponse {
  expires_at: string
  face_match_score: number | null
  assurance: 'high' | 'low'
  state: SessionState
}

export const studentApi = {
  gate: (pin?: string) => request<GateInfo>(`${base}/gate`, { headers: headers(pin) }),

  verify: (pin: string | undefined, matricNo: string, consent: boolean, selfie?: Blob, idPhoto?: Blob) => {
    const fd = new FormData()
    fd.set('matric_no', matricNo)
    fd.set('consent', String(consent))
    if (selfie) fd.set('selfie', selfie, 'selfie.jpg')
    if (idPhoto) fd.set('id_photo', idPhoto, 'id.jpg')
    return request<VerifyResponse>(`${base}/verify`, { method: 'POST', body: fd, headers: headers(pin) })
  },

  session: () => request<SessionState>(`${base}/session`, { headers: headers() }),

  saveAnswer: (questionId: number, response: string) =>
    request<{ saved_at: string }>(`${base}/answers`, {
      method: 'PUT',
      body: { question_id: questionId, response },
      headers: headers(),
    }),

  heartbeat: () => request<{ server_time: string; ends_at: string; proctoring: Proctoring }>(`${base}/heartbeat`, { method: 'POST', headers: headers() }),

  event: (type: string, metadata?: Record<string, string | number | boolean>) =>
    request<void>(`${base}/events`, { method: 'POST', body: { type, metadata }, headers: headers() }),

  frame: (jpeg: Blob) =>
    request<void>(`${base}/frames`, { method: 'POST', body: jpeg, headers: { ...headers(), 'Content-Type': 'image/jpeg' } }),

  audio: (pcm: ArrayBuffer) =>
    request<void>(`${base}/audio`, {
      method: 'POST',
      body: pcm,
      headers: { ...headers(), 'Content-Type': 'application/octet-stream' },
    }),

  submit: () => request<{ status: string; ended_at: string }>(`${base}/submit`, { method: 'POST', headers: headers() }),
}
