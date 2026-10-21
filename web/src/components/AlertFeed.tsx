import type { Violation } from '../api/types'
import { Button, SeverityBadge } from './ui'

export const eventLabels: Record<string, string> = {
  TAB_SWITCH: 'Switched tab / app',
  WINDOW_BLUR: 'Left the exam window',
  FULLSCREEN_EXIT: 'Exited fullscreen',
  COPY_PASTE: 'Copy / paste attempt',
  CONTEXT_MENU: 'Right-click',
  DEVTOOLS_SHORTCUT: 'Developer-tools shortcut',
  PRINT_SCREEN: 'Screenshot key',
  CAMERA_UNAVAILABLE: 'Camera unavailable',
  MIC_UNAVAILABLE: 'Microphone unavailable',
  MULTIPLE_DISPLAYS: 'Second display connected',
  NETWORK_OFFLINE: 'Went offline',
  NO_FACE: 'No face in view',
  MULTIPLE_FACES: 'More than one face',
  FACE_MISMATCH: 'Face does not match',
  LOOKING_AWAY: 'Head turned away',
  GAZE_OFF_SCREEN: 'Eyes off screen',
  PHONE_DETECTED: 'Phone detected',
  BOOK_DETECTED: 'Book detected',
  EXTRA_DEVICE: 'Extra device detected',
  EXTRA_PERSON: 'Another person in view',
  VOICE_DETECTED: 'Voice detected',
  NO_REFERENCE_PHOTO: 'Verified against ID card only',
  FACE_REVIEW_REQUIRED: 'Identity needs a manual check',
  PROCTOR_NOTE: 'Proctor note',
}

const time = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })

/** Newest-first alert list. AI alerts are prompts for a human: each can be
 * confirmed or dismissed, and dismissing restores the integrity score. */
export function AlertFeed({
  items,
  onReview,
  onSelectSession,
}: {
  items: Violation[]
  onReview: (v: Violation, status: 'CONFIRMED' | 'DISMISSED') => void
  onSelectSession?: (sessionId: number) => void
}) {
  if (items.length === 0) return <p className="p-4 text-sm text-slate-500">No alerts yet.</p>
  return (
    <ul aria-label="Alerts" className="divide-y divide-slate-100">
      {items.map((v) => (
        <li key={v.id} className={`p-3 ${v.review_status === 'DISMISSED' ? 'opacity-50' : ''}`}>
          <div className="flex items-center justify-between gap-2">
            <SeverityBadge severity={v.severity} />
            <time className="text-xs text-slate-500" dateTime={v.occurred_at}>
              {time(v.occurred_at)}
            </time>
          </div>
          <p className="mt-1 text-sm font-medium text-slate-900">{eventLabels[v.event_type] ?? v.event_type}</p>
          <button
            type="button"
            className="text-left text-xs text-brand-700 hover:underline"
            onClick={() => onSelectSession?.(v.session_id)}
          >
            {v.full_name ?? `Session ${v.session_id}`} {v.matric_no && <span className="font-mono">· {v.matric_no}</span>}
          </button>
          <p className="text-xs text-slate-500">
            {v.source}
            {v.ai_confidence != null && ` · ${Math.round(v.ai_confidence * 100)}% confidence`}
            {v.review_status !== 'PENDING' && ` · ${v.review_status.toLowerCase()}`}
          </p>
          {v.review_status === 'PENDING' && (
            <div className="mt-2 flex gap-2">
              <Button variant="secondary" className="px-2 py-1 text-xs" onClick={() => onReview(v, 'CONFIRMED')}>
                Confirm
              </Button>
              <Button variant="ghost" className="px-2 py-1 text-xs" onClick={() => onReview(v, 'DISMISSED')}>
                Dismiss
              </Button>
            </div>
          )}
        </li>
      ))}
    </ul>
  )
}
