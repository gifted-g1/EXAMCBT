import { useEffect, useState } from 'react'
import { authedImage, staffApi } from '../../api/staff'
import type { SessionRow, Violation } from '../../api/types'
import { eventLabels } from '../AlertFeed'
import { Button, ErrorText, SeverityBadge, Spinner, StatusBadge, errorMessage } from '../ui'

function AuthedImg({ src, alt }: { src: string; alt: string }) {
  const [url, setUrl] = useState<string | null>(null)
  const [missing, setMissing] = useState(false)
  useEffect(() => {
    let live = true
    let made: string | null = null
    void authedImage(src).then((u) => {
      if (!live) {
        if (u) URL.revokeObjectURL(u)
        return
      }
      made = u
      if (u) setUrl(u)
      else setMissing(true)
    })
    return () => {
      live = false
      if (made) URL.revokeObjectURL(made)
    }
  }, [src])
  if (missing) return <div className="grid aspect-video place-items-center rounded-lg bg-slate-100 text-xs text-slate-400">Not available</div>
  if (!url) return <div className="aspect-video animate-pulse rounded-lg bg-slate-100" />
  return <img src={url} alt={alt} className="w-full rounded-lg" />
}

export function SessionDrawer({
  sessionId,
  canManage,
  onClose,
  onChanged,
}: {
  sessionId: number
  examId: number
  canManage: boolean
  onClose: () => void
  onChanged: () => void
}) {
  const [data, setData] = useState<{ session: SessionRow & { student?: { matric_no: string; full_name: string } }; violations: Violation[]; has_reference: boolean; has_id_photo: boolean } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [evidence, setEvidence] = useState<string | null>(null)
  const [note, setNote] = useState('')

  const load = () =>
    staffApi.session(sessionId).then(
      (r) => setData({ ...r, violations: r.violations ?? [] }),
      (e) => setError(errorMessage(e)),
    )
  useEffect(() => {
    void load()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [sessionId])

  const act = async (fn: () => Promise<unknown>) => {
    setError(null)
    try {
      await fn()
      await load()
      onChanged()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  const s = data?.session
  return (
    <div className="fixed inset-0 z-40 flex justify-end bg-slate-900/40" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Student session"
        className="h-full w-full max-w-xl overflow-y-auto bg-white p-6 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between">
          <div>
            <h2 className="text-lg font-semibold">{s?.student?.full_name ?? 'Session'}</h2>
            <p className="font-mono text-sm text-slate-500">{s?.student?.matric_no}</p>
          </div>
          <Button variant="ghost" onClick={onClose} aria-label="Close">
            ✕
          </Button>
        </div>
        <ErrorText>{error}</ErrorText>
        {!s ? (
          <Spinner />
        ) : (
          <div className="mt-4 space-y-6">
            <dl className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <dt className="text-slate-500">Status</dt>
                <dd>
                  <StatusBadge status={s.status} />
                </dd>
              </div>
              <div>
                <dt className="text-slate-500">Integrity</dt>
                <dd className="font-semibold">{Math.round(s.integrity_score)}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Face match at entry</dt>
                <dd>{s.face_match_score != null ? s.face_match_score.toFixed(3) : '—'}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Client IP</dt>
                <dd className="font-mono">{s.client_ip}</dd>
              </div>
            </dl>

            <div className="grid grid-cols-3 gap-3">
              <figure>
                <AuthedImg src={`/api/staff/sessions/${s.id}/image/latest`} alt="Latest frame" />
                <figcaption className="mt-1 text-xs text-slate-500">Latest</figcaption>
              </figure>
              <figure>
                <AuthedImg src={`/api/staff/sessions/${s.id}/image/reference`} alt="Selfie at entry" />
                <figcaption className="mt-1 text-xs text-slate-500">Entry selfie</figcaption>
              </figure>
              <figure>
                <AuthedImg src={`/api/staff/sessions/${s.id}/image/id`} alt="ID card" />
                <figcaption className="mt-1 text-xs text-slate-500">ID card</figcaption>
              </figure>
            </div>

            {canManage && s.status === 'ACTIVE' && (
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="secondary"
                  onClick={() =>
                    window.confirm('Unlock this session so the student can continue on another computer?') &&
                    act(() => staffApi.unlock(s.id))
                  }
                >
                  Unlock device
                </Button>
                <Button
                  variant="danger"
                  onClick={() =>
                    window.confirm('End this student’s exam now? Their answers so far are submitted.') &&
                    act(() => staffApi.terminate(s.id))
                  }
                >
                  End session
                </Button>
              </div>
            )}

            <div>
              <h3 className="font-semibold">Add a note</h3>
              <div className="mt-2 flex gap-2">
                <input
                  className="flex-1 rounded-lg border-0 px-3 py-2 text-sm ring-1 ring-inset ring-slate-300"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder="What did you observe?"
                  maxLength={500}
                />
                <Button
                  variant="secondary"
                  disabled={!note.trim()}
                  onClick={() =>
                    act(async () => {
                      await staffApi.flag(s.id, 'MEDIUM', note.trim())
                      setNote('')
                    })
                  }
                >
                  Flag
                </Button>
              </div>
            </div>

            <div>
              <h3 className="font-semibold">Timeline</h3>
              {data.violations.length === 0 ? (
                <p className="mt-2 text-sm text-slate-500">No events.</p>
              ) : (
                <ul className="mt-2 divide-y divide-slate-100 text-sm">
                  {data.violations.map((v) => (
                    <li key={v.id} className="flex items-center gap-3 py-2">
                      <SeverityBadge severity={v.severity} />
                      <span className="flex-1">
                        {eventLabels[v.event_type] ?? v.event_type}
                        {typeof v.metadata?.note === 'string' && <span className="block text-xs text-slate-500">“{v.metadata.note}”</span>}
                      </span>
                      <time className="text-xs text-slate-500">{new Date(v.occurred_at).toLocaleTimeString()}</time>
                      {v.media_ref && (
                        <button type="button" className="text-xs text-brand-700 underline" onClick={() => setEvidence(v.media_ref!)}>
                          Evidence
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>
            {evidence && (
              <div>
                <h3 className="font-semibold">Evidence</h3>
                <AuthedImg src={`/api/staff/media?key=${encodeURIComponent(evidence)}`} alt="Evidence frame" />
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
