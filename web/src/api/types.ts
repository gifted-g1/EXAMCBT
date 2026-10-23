export type Role = 'proctor' | 'admin'
export type ExamStatus = 'DRAFT' | 'SCHEDULED' | 'ACTIVE' | 'CLOSED'
export type SessionStatus = 'INIT' | 'ACTIVE' | 'SUBMITTED' | 'CLOSED'
export type Severity = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'
export type ReviewStatus = 'PENDING' | 'CONFIRMED' | 'DISMISSED'
export type QuestionKind = 'MCQ' | 'TEXT' | 'GAP'

export interface User {
  id: number
  username: string
  full_name: string
  role: Role
  is_active: boolean
  created_at: string
}

export interface Exam {
  id: number
  title: string
  description: string
  created_by: number
  starts_at: string
  ends_at: string
  status: ExamStatus
  lan_cidr: string
  pin_code?: string
  proctoring: Proctoring
  questions_count: number
  duration_minutes: number
  questions_per_student: number
}

export interface Question {
  id: number
  exam_id: number
  position: number
  kind: QuestionKind
  prompt: string
  options: string[] | null
  correct_option?: number | null
  blanks?: string[][] | null
  marks: number
  marking_guide?: MarkingPoint[] | null
  model_answer?: string
}

/** One key point an examiner expects in a written answer. */
export interface MarkingPoint {
  point: string
  marks: number
  keywords: string[]
}

export interface Student {
  id: number
  matric_no: string
  full_name: string
  department?: string
  photo_url?: string
}

export interface SessionRow {
  id: number
  exam_id: number
  student_id: number
  matric_no: string
  full_name: string
  status: SessionStatus
  client_ip: string
  started_at: string
  ended_at?: string
  last_seen_at?: string
  integrity_score: number
  face_match_score?: number
  score?: number
  auto_submitted: boolean
  violations_count: number
  high_count: number
  answered_count: number
}

export interface Violation {
  id: number
  session_id: number
  event_type: string
  severity: Severity
  source: 'CLIENT' | 'AI' | 'PROCTOR'
  occurred_at: string
  media_ref?: string
  ai_confidence?: number
  metadata?: Record<string, unknown>
  review_status: ReviewStatus
  matric_no?: string
  full_name?: string
}

export interface AccessCard {
  ip: string
  url: string
  pin?: string
  qr_url: string
  qr_png_base64: string
  lan_cidr: string
  status: ExamStatus
}

export interface GateInfo {
  exam_id: number
  title: string
  description: string
  starts_at: string
  ends_at: string
  server_time: string
  consent_version: string
  face_match_threshold: number
  proctoring: Proctoring
  duration_minutes: number
}

export interface QuestionView {
  id: number
  position: number
  kind: QuestionKind
  prompt: string
  options?: string[]
  marks: number
  /** GAP: number of blanks. */
  blanks?: number
}

export interface SessionState {
  session_id: number
  status: SessionStatus
  exam_id: number
  exam_title: string
  /** This student's deadline (start + time limit, capped at the window end). */
  ends_at: string
  duration_minutes: number
  server_time: string
  student: { matric_no: string; full_name: string }
  questions: QuestionView[]
  answers: Record<string, string>
  proctoring: Proctoring
}

export interface ReportRow {
  session_id: number
  matric_no: string
  full_name: string
  status: string
  score: number | null
  max_score: number
  integrity_score: number
  face_match_score: number | null
  violations: number
  high_violations: number
  auto_submitted: boolean
  needs_manual_marking: boolean
}

export interface Report {
  exam: Exam
  generated_at: string
  rows: ReportRow[] | null
  summary: { candidates: number; submitted: number; average_score: number; average_integrity: number; flagged_high: number }
}

export interface HubEvent {
  type: 'violation' | 'session' | 'exam' | 'review' | 'verify_failed'
  exam_id: number
  at: string
  data: Record<string, unknown>
}

/** Per-exam proctoring switches (all on by default). */
export interface Proctoring {
  face_verification: boolean
  camera_monitoring: boolean
  audio_monitoring: boolean
  fullscreen_required: boolean
  tab_switch_detection: boolean
  clipboard_blocking: boolean
}

export const proctoringNeedsMedia = (p?: Proctoring) =>
  !p || p.face_verification || p.camera_monitoring || p.audio_monitoring

export type AnswerStatus = 'correct' | 'incorrect' | 'unanswered' | 'needs_marking' | 'marked' | 'partial'

export interface ScriptItem {
  question_id: number
  number: number
  kind: QuestionKind
  prompt: string
  options?: string[]
  correct_option?: number | null
  response: string
  status: AnswerStatus
  marks: number
  awarded: number | null
  comment?: string
  blanks?: string[][]
  blank_answers?: string[]
  blank_correct?: boolean[]
}

/** A student's marked answer sheet (staff only). */
export interface AnswerScript {
  session_id: number
  exam_id: number
  exam_title: string
  matric_no: string
  full_name: string
  status: SessionStatus
  score: number
  max_score: number
  auto_marked_max: number
  counts: Record<AnswerStatus, number>
  items: ScriptItem[]
}

/** Class-wide result for one question. */
export interface QuestionStat {
  question_id: number
  number: number
  kind: QuestionKind
  prompt: string
  options?: string[]
  correct_option?: number | null
  correct: number
  incorrect: number
  partial?: number
  unanswered: number
  answered: number
  /** Students who were given this question (differs when questions are drawn at random). */
  seen?: number
  option_counts?: number[]
  common_wrong?: number | null
  percent_right: number | null
}

// --- Theory marking ------------------------------------------------------------

export interface MarkingQuestion {
  question_id: number
  number: number
  prompt: string
  marks: number
  marking_guide: MarkingPoint[]
  model_answer: string
  answered: number
  marked: number
}

export interface MarkingOverview {
  ai_enabled: boolean
  questions: MarkingQuestion[]
}

export interface PointMatch {
  index: number
  point: string
  marks: number
  met: boolean
  matched: string[]
}

export interface KeywordSuggestion {
  marks: number
  max: number
  points: PointMatch[]
  highlights: string[]
}

export interface AISuggestion {
  marks: number
  reason: string
  points_met: number[] | null
  model: string
  at: string
}

/** One anonymous candidate's answer to the question being marked. */
export interface MarkingScript {
  answer_id: number
  candidate: string
  response: string
  awarded: number | null
  comment: string
  marked_at: string | null
  keyword_suggestion: KeywordSuggestion | null
  ai_suggestion: AISuggestion | null
  submitted: boolean
}

export interface QuestionScripts {
  question: MarkingQuestion
  scripts: MarkingScript[]
}

export interface MarkResult {
  answer_id: number
  awarded: number | null
  session_id: number
  session_score: number | null
}

export interface AISuggestResult {
  requested: number
  suggested: number
  failed: number
  errors?: string[]
}
