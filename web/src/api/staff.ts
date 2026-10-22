import { ApiError, request, type RequestOptions } from './client'
import type {
  AccessCard,
  AISuggestResult,
  AnswerScript,
  MarkingOverview,
  MarkingPoint,
  MarkResult,
  QuestionScripts,
  QuestionStat,
  Proctoring,
  Exam,
  Question,
  Report,
  SessionRow,
  Student,
  User,
  Violation,
} from './types'

/**
 * Staff auth: the 15-minute access token lives only in memory (never in
 * localStorage, so XSS can't lift a long-lived credential); the 7-day refresh
 * token is an HTTP-only cookie the browser sends to /api/staff/auth only.
 */
let accessToken: string | null = null
let refreshing: Promise<boolean> | null = null
const listeners = new Set<(u: User | null) => void>()

export function onAuthChange(fn: (u: User | null) => void): () => void {
  listeners.add(fn)
  return () => {
    listeners.delete(fn)
  }
}

function emit(u: User | null) {
  listeners.forEach((fn) => fn(u))
}

interface LoginResponse {
  access_token: string
  expires_at: string
  user: User
}

/** Refresh once even if many requests 401 at the same moment. */
export function refresh(): Promise<boolean> {
  if (!refreshing) {
    refreshing = request<LoginResponse>('/api/staff/auth/refresh', { method: 'POST' })
      .then((r) => {
        accessToken = r.access_token
        emit(r.user)
        return true
      })
      .catch(() => {
        accessToken = null
        emit(null)
        return false
      })
      .finally(() => {
        refreshing = null
      })
  }
  return refreshing
}

async function authed<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const run = () =>
    request<T>(path, { ...opts, headers: { ...opts.headers, ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}) } })
  try {
    return await run()
  } catch (e) {
    if (e instanceof ApiError && e.status === 401 && (await refresh())) return run()
    throw e
  }
}

/** Fetch an authenticated image as an object URL (img tags can't send headers). */
export async function authedImage(path: string): Promise<string | null> {
  const get = () => fetch(path, { headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {} })
  let res = await get()
  if (res.status === 401 && (await refresh())) res = await get()
  if (!res.ok) return null
  return URL.createObjectURL(await res.blob())
}

export interface ExamInput {
  title: string
  description: string
  starts_at: string
  ends_at: string
  lan_cidr: string
  require_pin?: boolean
}

export interface QuestionInput {
  kind: 'MCQ' | 'TEXT'
  prompt: string
  options?: string[]
  correct_option?: number
  marks?: number
}

const s = '/api/staff'

export const staffApi = {
  async login(username: string, password: string): Promise<User> {
    const r = await request<LoginResponse>(`${s}/auth/login`, { method: 'POST', body: { username, password } })
    accessToken = r.access_token
    emit(r.user)
    return r.user
  },
  async logout(): Promise<void> {
    try {
      await request<void>(`${s}/auth/logout`, { method: 'POST' })
    } finally {
      accessToken = null
      emit(null)
    }
  },
  me: () => authed<User>(`${s}/me`),
  system: () => authed<Record<string, unknown>>(`${s}/system`),

  exams: () => authed<{ exams: Exam[] }>(`${s}/exams`).then((r) => r.exams),
  exam: (id: number) => authed<{ exam: Exam; can_manage: boolean }>(`${s}/exams/${id}`),
  createExam: (in_: ExamInput) => authed<Exam>(`${s}/exams`, { method: 'POST', body: in_ }),
  updateExam: (id: number, in_: ExamInput) => authed<Exam>(`${s}/exams/${id}`, { method: 'PUT', body: in_ }),
  deleteExam: (id: number) => authed<void>(`${s}/exams/${id}`, { method: 'DELETE' }),
  setStatus: (id: number, action: 'schedule' | 'open' | 'close') =>
    authed<Exam>(`${s}/exams/${id}/status`, { method: 'POST', body: { action } }),
  setPin: (id: number, enabled: boolean) => authed<Exam>(`${s}/exams/${id}/pin`, { method: 'POST', body: { enabled } }),
  setProctoring: (id: number, patch: Partial<Proctoring>) =>
    authed<Exam>(`${s}/exams/${id}/proctoring`, { method: 'PUT', body: patch }),
  accessCard: (id: number) => authed<AccessCard>(`${s}/exams/${id}/access-card`),

  questions: (id: number) => authed<{ questions: Question[] }>(`${s}/exams/${id}/questions`).then((r) => r.questions),
  addQuestions: (id: number, questions: QuestionInput[]) =>
    authed<{ questions: Question[] }>(`${s}/exams/${id}/questions`, { method: 'POST', body: { questions } }),
  deleteQuestion: (id: number, qid: number) => authed<void>(`${s}/exams/${id}/questions/${qid}`, { method: 'DELETE' }),
  parseQuestionsPDF: (id: number, file: File) => {
    const fd = new FormData()
    fd.set('file', file)
    return authed<ParsedPDF>(`${s}/exams/${id}/questions/parse-pdf`, { method: 'POST', body: fd })
  },
  samplePDF: () => authedImage(`${s}/questions/sample.pdf`),

  sessions: (id: number) => authed<{ sessions: SessionRow[] | null; server_time: string }>(`${s}/exams/${id}/sessions`),
  violations: (id: number) =>
    authed<{ violations: Violation[] | null }>(`${s}/exams/${id}/violations?limit=200`).then((r) => r.violations ?? []),
  review: (examId: number, vid: number, status: 'CONFIRMED' | 'DISMISSED' | 'PENDING') =>
    authed<{ integrity_score: number }>(`${s}/exams/${examId}/violations/${vid}/review`, { method: 'POST', body: { status } }),
  report: (id: number) => authed<Report>(`${s}/exams/${id}/report`),
  answerScript: (sid: number) => authed<AnswerScript>(`${s}/sessions/${sid}/answers`),
  itemAnalysis: (id: number) =>
    authed<{ candidates: number; questions: QuestionStat[] | null }>(`${s}/exams/${id}/item-analysis`),
  marking: (id: number) => authed<MarkingOverview>(`${s}/exams/${id}/marking`),
  markingScripts: (id: number, qid: number) => authed<QuestionScripts>(`${s}/exams/${id}/marking/${qid}`),
  setMarkingGuide: (id: number, qid: number, points: MarkingPoint[], model_answer: string) =>
    authed<Question>(`${s}/exams/${id}/questions/${qid}/guide`, { method: 'PUT', body: { points, model_answer } }),
  aiSuggest: (id: number, qid: number, answer_ids?: number[]) =>
    authed<AISuggestResult>(`${s}/exams/${id}/marking/${qid}/ai-suggest`, { method: 'POST', body: { answer_ids } }),
  markAnswer: (id: number, aid: number, marks: number | null, comment: string, source: 'manual' | 'keyword' | 'ai') =>
    authed<MarkResult>(`${s}/exams/${id}/answers/${aid}/mark`, { method: 'PUT', body: { marks, comment, source } }),
  reportCSV: async (id: number) => {
    const url = await authedBlob(`${s}/exams/${id}/report?format=csv`)
    return url
  },

  session: (sid: number) =>
    authed<{ session: SessionRow; violations: Violation[] | null; has_reference: boolean; has_id_photo: boolean }>(
      `${s}/sessions/${sid}`,
    ),
  unlock: (sid: number) => authed<void>(`${s}/sessions/${sid}/unlock`, { method: 'POST' }),
  terminate: (sid: number) => authed<void>(`${s}/sessions/${sid}/terminate`, { method: 'POST' }),
  flag: (sid: number, severity: string, note: string) =>
    authed<Violation>(`${s}/sessions/${sid}/flag`, { method: 'POST', body: { event_type: 'PROCTOR_NOTE', severity, note } }),

  wsTicket: (examId: number) => authed<{ ticket: string }>(`${s}/ws-ticket`, { method: 'POST', body: { exam_id: examId } }),

  students: (q: string, page = 1) =>
    authed<{ students: Student[] | null; total: number; page: number; per_page: number }>(
      `${s}/students?q=${encodeURIComponent(q)}&page=${page}`,
    ),
  createStudent: (matric_no: string, full_name: string) =>
    authed<Student>(`${s}/students`, { method: 'POST', body: { matric_no, full_name } }),
  importRoster: (file: File) => {
    const fd = new FormData()
    fd.set('file', file)
    return authed<{ imported: number; rejected: { line: number; error: string }[] }>(`${s}/students/import`, {
      method: 'POST',
      body: fd,
    })
  },
  uploadPhotos: (files: FileList) => {
    const fd = new FormData()
    Array.from(files).forEach((f) => fd.append('photos', f))
    return authed<{ results: { file: string; status: string }[] }>(`${s}/students/photos`, { method: 'POST', body: fd })
  },
  deleteStudent: (id: number) => authed<void>(`${s}/students/${id}`, { method: 'DELETE' }),

  users: () => authed<{ users: User[] }>(`${s}/users`).then((r) => r.users),
  createUser: (u: { username: string; password: string; full_name: string; role: string }) =>
    authed<User>(`${s}/users`, { method: 'POST', body: u }),
  updateUser: (id: number, patch: Partial<{ is_active: boolean; role: string; password: string; full_name: string }>) =>
    authed<User>(`${s}/users/${id}`, { method: 'PATCH', body: patch }),
}

async function authedBlob(path: string): Promise<string | null> {
  return authedImage(path)
}

/** A question as parsed from a PDF (preview only — nothing saved yet). */
export interface ParsedQuestion {
  number: number
  kind: 'MCQ' | 'TEXT'
  prompt: string
  options?: string[]
  correct_option: number | null
  marks: number
  warnings?: string[]
}

export interface ParsedPDF {
  questions: ParsedQuestion[]
  warnings: string[]
  pages: number
  lines: number
}
