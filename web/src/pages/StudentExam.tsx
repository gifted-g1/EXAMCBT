import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { ApiError, isNetworkError } from '../api/client'
import { studentApi } from '../api/student'
import { proctoringNeedsMedia, type GateInfo, type Proctoring, type SessionState } from '../api/types'
import { CountdownTimer } from '../components/CountdownTimer'
import { QuestionPanel } from '../components/QuestionPanel'
import { Button, Card, ErrorText, Field, Spinner, inputClass } from '../components/ui'
import { WebcamFeed } from '../components/WebcamFeed'
import { useFullscreenLock } from '../hooks/useFullscreenLock'
import { useTabSwitchDetect } from '../hooks/useTabSwitchDetect'
import { AnswerQueue } from '../lib/offlineQueue'
import { clearExamCache, loadExamCache, pendingKey, saveExamCache } from '../lib/sessionCache'
import { InsecureContextError, captureFrame, mediaAvailable, openCamera, openMedia, recordClip, stopStream } from '../lib/media'
import { Locked } from './Locked'

const ALL_ON: Proctoring = {
  face_verification: true,
  camera_monitoring: true,
  audio_monitoring: true,
  fullscreen_required: true,
  tab_switch_detection: true,
  clipboard_blocking: true,
}

type Phase =
  | { k: 'loading' }
  | { k: 'locked'; code: string; message?: string }
  | { k: 'identify'; gate: GateInfo }
  | { k: 'exam'; state: SessionState; offsetMs: number }
  | { k: 'done'; auto: boolean }

const PIN_KEY = 'es_pin'

function readPin(): string | undefined {
  const url = new URL(window.location.href)
  const fromUrl = url.searchParams.get('pin')
  if (fromUrl && /^\d{4}$/.test(fromUrl)) {
    sessionStorage.setItem(PIN_KEY, fromUrl)
    // Drop the PIN from the address bar so it isn't shoulder-surfed or
    // captured in screenshots.
    url.searchParams.delete('pin')
    window.history.replaceState(null, '', url.pathname + url.search)
    return fromUrl
  }
  return sessionStorage.getItem(PIN_KEY) ?? undefined
}

function lockFrom(e: unknown): Phase {
  if (e instanceof InsecureContextError) return { k: 'locked', code: 'INSECURE_CONTEXT' }
  if (e instanceof DOMException && (e.name === 'NotAllowedError' || e.name === 'NotFoundError'))
    return { k: 'locked', code: 'CAMERA_DENIED' }
  if (e instanceof ApiError) return { k: 'locked', code: e.code, message: e.message }
  if (isNetworkError(e)) return { k: 'locked', code: 'NETWORK' }
  return { k: 'locked', code: 'UNKNOWN', message: e instanceof Error ? e.message : undefined }
}

/**
 * The only page a student ever sees. No navigation, no links, no logout:
 * gate → identify → fullscreen exam → done.
 */
export default function StudentExam() {
  const [phase, setPhase] = useState<Phase>({ k: 'loading' })
  const pinRef = useRef<string | undefined>(undefined)
  const onDone = useCallback((auto: boolean) => {
    // Submitted: nothing of this exam should stay on a shared computer.
    clearExamCache()
    try {
      sessionStorage.removeItem(PIN_KEY)
    } catch {
      /* ignore */
    }
    setPhase({ k: 'done', auto })
  }, [])
  const onVerified = useCallback((state: SessionState) => {
    const offsetMs = new Date(state.server_time).getTime() - Date.now()
    saveExamCache(state, offsetMs)
    setPhase({ k: 'exam', state, offsetMs })
  }, [])

  const start = useCallback(async () => {
    setPhase({ k: 'loading' })
    // Resume first: a reload or reconnect keeps the session cookie.
    try {
      const st = await studentApi.session()
      if (st.status === 'ACTIVE') {
        const offsetMs = new Date(st.server_time).getTime() - Date.now()
        saveExamCache(st, offsetMs)
        setPhase({ k: 'exam', state: st, offsetMs })
        return
      }
      clearExamCache()
    } catch (e) {
      // Server unreachable (Wi-Fi down, server restarting after a power cut):
      // reopen the exam from this computer's copy and keep saving locally.
      const cached = isNetworkError(e) ? loadExamCache() : null
      if (cached) {
        setPhase({ k: 'exam', state: cached.state, offsetMs: cached.offsetMs })
        return
      }
      /* otherwise: no session yet */
    }
    try {
      const gate = await studentApi.gate(pinRef.current)
      // Camera/microphone need https://; an exam with face checks and
      // monitoring switched off works over plain http.
      if (proctoringNeedsMedia(gate.proctoring) && !mediaAvailable()) {
        setPhase({ k: 'locked', code: 'INSECURE_CONTEXT' })
        return
      }
      setPhase({ k: 'identify', gate })
    } catch (e) {
      setPhase(lockFrom(e))
    }
  }, [])

  useEffect(() => {
    pinRef.current = readPin()
    void start()
  }, [start])

  // Re-check every 15 s while waiting for the exam to open.
  useEffect(() => {
    if (phase.k !== 'locked' || phase.code !== 'EXAM_NOT_OPEN') return
    const id = setInterval(() => void start(), 15000)
    return () => clearInterval(id)
  }, [phase, start])

  switch (phase.k) {
    case 'loading':
      return (
        <main className="grid min-h-screen place-items-center bg-slate-900">
          <div className="rounded-lg bg-white px-4 py-3">
            <Spinner label="Connecting to the exam server…" />
          </div>
        </main>
      )
    case 'locked':
      return (
        <Locked
          code={phase.code}
          message={phase.message}
          onPin={(pin) => {
            pinRef.current = pin
            sessionStorage.setItem(PIN_KEY, pin)
            void start()
          }}
          onRetry={() => void start()}
        />
      )
    case 'identify':
      return (
        <Identify
          gate={phase.gate}
          pin={pinRef.current}
          onVerified={onVerified}
          onLocked={setPhase}
        />
      )
    case 'exam':
      return <ExamRoom initial={phase.state} offsetMs={phase.offsetMs} onDone={onDone} onLocked={setPhase} />
    case 'done':
      return (
        <main className="grid min-h-screen place-items-center bg-slate-900 px-4 text-center text-slate-100">
          <section>
            <div aria-hidden className="mx-auto mb-6 grid h-14 w-14 place-items-center rounded-2xl bg-emerald-600 text-2xl">
              ✓
            </div>
            <h1 className="text-2xl font-semibold">Exam submitted</h1>
            <p className="mt-3 text-slate-300">
              {phase.auto ? 'Time is up — your answers were submitted automatically.' : 'Your answers have been recorded.'} You may
              close this window.
            </p>
          </section>
        </main>
      )
  }
}

// --- Identity verification --------------------------------------------------------

function Identify({
  gate,
  pin,
  onVerified,
  onLocked,
}: {
  gate: GateInfo
  pin?: string
  onVerified: (s: SessionState) => void
  onLocked: (p: Phase) => void
}) {
  const [stream, setStream] = useState<MediaStream | null>(null)
  const video = useRef<HTMLVideoElement>(null)
  const [matric, setMatric] = useState('')
  const [consent, setConsent] = useState(false)
  const [selfie, setSelfie] = useState<Blob | null>(null)
  const [idPhoto, setIdPhoto] = useState<Blob | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const previews = useMemo(
    () => ({ selfie: selfie && URL.createObjectURL(selfie), id: idPhoto && URL.createObjectURL(idPhoto) }),
    [selfie, idPhoto],
  )
  useEffect(
    () => () => {
      if (previews.selfie) URL.revokeObjectURL(previews.selfie)
      if (previews.id) URL.revokeObjectURL(previews.id)
    },
    [previews],
  )

  const p = gate.proctoring
  const faceCheck = p?.face_verification ?? true

  useEffect(() => {
    if (!faceCheck) return
    let s: MediaStream | null = null
    let cancelled = false
    openCamera(false)
      .then((st) => {
        if (cancelled) stopStream(st)
        else {
          s = st
          setStream(st)
        }
      })
      .catch((e) => !cancelled && onLocked(lockFrom(e)))
    return () => {
      cancelled = true
      stopStream(s)
    }
  }, [onLocked, faceCheck])

  // What the consent notice says depends on what this exam actually records.
  const recorded = [
    p?.camera_monitoring !== false && 'your webcam image',
    p?.audio_monitoring !== false && 'short microphone samples',
    faceCheck && 'the photos you take now',
    (p?.tab_switch_detection !== false || p?.clipboard_blocking !== false || p?.fullscreen_required !== false) &&
      'browser activity (such as tab switches, copy/paste and fullscreen exits)',
  ].filter(Boolean) as string[]

  const snap = async (which: 'selfie' | 'id') => {
    if (!video.current) return
    const b = await captureFrame(video.current, which === 'id' ? 1024 : 640, 0.9)
    if (!b) return setError('Camera not ready yet — try again in a moment.')
    setError(null)
    if (which === 'selfie') setSelfie(b)
    else setIdPhoto(b)
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (faceCheck && !selfie) return
    setBusy(true)
    setError(null)
    try {
      const res = await studentApi.verify(pin, matric.trim(), consent, selfie ?? undefined, idPhoto ?? undefined)
      onVerified(res.state)
    } catch (err) {
      if (err instanceof ApiError && ['FACE_MISMATCH', 'NO_FACE_DETECTED', 'NO_REFERENCE_PHOTO', 'MULTIPLE_FACES', 'NOT_ON_ROSTER', 'VALIDATION_FAILED', 'CONSENT_REQUIRED'].includes(err.code)) {
        setError(err.message)
        if (err.code !== 'NOT_ON_ROSTER') setSelfie(null)
      } else {
        onLocked(lockFrom(err))
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="min-h-screen bg-slate-100 px-4 py-8">
      <div className="mx-auto max-w-4xl">
        <header className="mb-6">
          <p className="text-sm font-medium text-brand-700">Exam Shield</p>
          <h1 className="text-2xl font-semibold text-slate-900">{gate.title}</h1>
          <p className="text-sm text-slate-600">
            Ends at {new Date(gate.ends_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </p>
        </header>

        <form onSubmit={submit} className={`grid gap-6 ${faceCheck ? 'md:grid-cols-2' : 'mx-auto max-w-lg'}`}>
          {faceCheck && (
          <Card className="space-y-4">
            <h2 className="font-semibold text-slate-900">1. Confirm who you are</h2>
            <WebcamFeed ref={video} stream={stream} className="aspect-video" />
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Button variant="secondary" className="w-full" onClick={() => snap('selfie')}>
                  {selfie ? 'Retake selfie' : 'Take selfie'}
                </Button>
                {previews.selfie && <img src={previews.selfie} alt="Your selfie" className="mt-2 rounded-lg" />}
              </div>
              <div>
                <Button variant="secondary" className="w-full" onClick={() => snap('id')}>
                  {idPhoto ? 'Retake ID photo' : 'Photo of ID card'}
                </Button>
                {previews.id && <img src={previews.id} alt="Your ID card" className="mt-2 rounded-lg" />}
              </div>
            </div>
            <p className="text-xs text-slate-500">
              Look straight at the camera in good light. For the ID photo, hold your student ID card up to the camera.
            </p>
          </Card>
          )}

          <Card className="space-y-4">
            <h2 className="font-semibold text-slate-900">{faceCheck ? '2. Your details' : 'Your details'}</h2>
            <Field label="Matric number" hint="For example IFT/22/9203">
              <input
                className={`${inputClass} font-mono uppercase`}
                value={matric}
                onChange={(e) => setMatric(e.target.value.toUpperCase())}
                placeholder="IFT/22/9203"
                autoComplete="off"
                required
                pattern="[A-Za-z]{2,4}/\d{2}/\d{4}"
              />
            </Field>

            <div className="rounded-lg bg-slate-50 p-3 text-xs leading-relaxed text-slate-600">
              <p className="font-semibold text-slate-800">Proctoring notice</p>
              <p className="mt-1">
                {recorded.length > 0
                  ? `During this exam ${recorded.join(', ')} are recorded on the exam server and checked automatically. `
                  : 'Your answers and submission times are recorded on the exam server. '}
                Automated flags are reviewed by an invigilator before any decision is made. Recordings are kept only for the
                institution's retention period and then deleted. Notice version {gate.consent_version}.
              </p>
            </div>
            <label className="flex items-start gap-2 text-sm">
              <input type="checkbox" className="mt-1" checked={consent} onChange={(e) => setConsent(e.target.checked)} />
              <span>I have read the notice and consent to being proctored for this exam.</span>
            </label>

            <ErrorText>{error}</ErrorText>
            <Button type="submit" className="w-full" disabled={!consent || (faceCheck && !selfie) || !matric || busy}>
              {busy ? 'Verifying…' : faceCheck ? 'Verify and start exam' : 'Start exam'}
            </Button>
          </Card>
        </form>
      </div>
    </main>
  )
}

// --- Exam room ------------------------------------------------------------

function ExamRoom({
  initial,
  offsetMs,
  onDone,
  onLocked,
}: {
  initial: SessionState
  offsetMs: number
  onDone: (auto: boolean) => void
  onLocked: (p: Phase) => void
}) {
  const queue = useMemo(
    () =>
      new AnswerQueue(
        (qid, resp) => studentApi.saveAnswer(qid, resp),
        (e) => e instanceof ApiError && (e.status === 400 || e.status === 410),
        pendingKey(initial.session_id),
      ),
    [initial.session_id],
  )
  // Saved answers from the server, overlaid with anything typed on this
  // computer that has not reached the server yet (e.g. before a power cut).
  const [answers, setAnswers] = useState<Record<string, string>>(() => {
    const merged = { ...(initial.answers ?? {}) }
    for (const [qid, resp] of queue.entries()) merged[String(qid)] = resp
    return merged
  })
  const [pending, setPending] = useState(0)
  const [stream, setStream] = useState<MediaStream | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  // Waiting to submit because the server can't be reached right now.
  const [submitWaiting, setSubmitWaiting] = useState(false)
  const [online, setOnline] = useState(() => navigator.onLine)
  const [endsAt, setEndsAt] = useState(initial.ends_at)
  // Proctoring switches; refreshed by every heartbeat so an examiner's
  // change applies mid-exam within ~30 s.
  const [proc, setProc] = useState<Proctoring>(initial.proctoring ?? ALL_ON)
  const video = useRef<HTMLVideoElement>(null)
  const mediaBusy = useRef(false)
  const finished = useRef(false)
  const pendingRef = useRef(0)
  pendingRef.current = pending
  const onlineRef = useRef(online)
  onlineRef.current = online

  const report = useCallback((type: string, meta?: Record<string, string | number | boolean>) => {
    void studentApi.event(type, meta).catch(() => undefined)
  }, [])

  useEffect(() => {
    const unsub = queue.subscribe(setPending)
    void queue.flush()
    const goOnline = () => {
      setOnline(true)
      void queue.flush()
    }
    const goOffline = () => setOnline(false)
    window.addEventListener('online', goOnline)
    window.addEventListener('offline', goOffline)
    return () => {
      unsub()
      window.removeEventListener('online', goOnline)
      window.removeEventListener('offline', goOffline)
      queue.dispose()
    }
  }, [queue])

  // Keep this computer's copy of the exam current, so a reload during an
  // outage reopens exactly where the student was.
  useEffect(() => {
    const t = setTimeout(() => saveExamCache({ ...initial, answers, ends_at: endsAt, proctoring: proc }, offsetMs), 800)
    return () => clearTimeout(t)
  }, [initial, answers, endsAt, proc, offsetMs])

  const finish = useCallback(
    async (auto: boolean) => {
      if (finished.current) return
      finished.current = true
      setSubmitting(true)
      const attempt = async (): Promise<void> => {
        // Every answer must reach the server before we submit.
        await queue.flush()
        try {
          if (queue.size > 0) throw new TypeError('answers still pending')
          await studentApi.submit()
        } catch (e) {
          if (isNetworkError(e)) {
            // No connection (or the server is restarting): answers are safe on
            // this computer; keep trying until the server confirms.
            setSubmitWaiting(true)
            setOnline(false)
            setTimeout(() => void attempt(), 5000)
            return
          }
          // 410 = the server already closed the exam (time up): still done.
          if (!(e instanceof ApiError && (e.status === 410 || e.code === 'ALREADY_SUBMITTED'))) {
            finished.current = false
            setSubmitting(false)
            setSubmitWaiting(false)
            if (e instanceof ApiError && (e.status === 401 || e.code === 'DEVICE_MISMATCH')) onLocked(lockFrom(e))
            return
          }
        }
        queue.clear()
        stopStream(stream)
        if (document.fullscreenElement) void document.exitFullscreen().catch(() => undefined)
        onDone(auto)
      }
      await attempt()
    },
    [queue, stream, onDone, onLocked],
  )

  const fs = useFullscreenLock(proc.fullscreen_required, () => report('FULLSCREEN_EXIT'))
  useTabSwitchDetect(true, report, { tabs: proc.tab_switch_detection, clipboard: proc.clipboard_blocking })

  const wantVideo = proc.camera_monitoring
  const wantAudio = proc.audio_monitoring

  // Camera and/or microphone, only as this exam's settings require. Re-opened
  // if an examiner switches either on or off mid-exam.
  useEffect(() => {
    if (!wantVideo && !wantAudio) {
      setStream(null)
      return
    }
    let s: MediaStream | null = null
    let cancelled = false
    const open = async () => {
      try {
        s = await openMedia(wantVideo, wantAudio)
      } catch {
        if (wantVideo && wantAudio) {
          try {
            s = await openMedia(true, false)
            report('MIC_UNAVAILABLE')
          } catch {
            report('CAMERA_UNAVAILABLE')
            return
          }
        } else {
          report(wantVideo ? 'CAMERA_UNAVAILABLE' : 'MIC_UNAVAILABLE')
          return
        }
      }
      if (cancelled) stopStream(s)
      else setStream(s)
    }
    void open()
    return () => {
      cancelled = true
      stopStream(s)
    }
  }, [report, wantVideo, wantAudio])

  // Webcam frames every 5 s (camera monitoring), audio clip every 20 s (mic).
  useEffect(() => {
    if (!stream) return
    const timers: ReturnType<typeof setInterval>[] = []
    if (wantVideo && stream.getVideoTracks().length) {
      timers.push(
        setInterval(async () => {
          // Answers first: skip frames while offline, while answers are still
          // sending, or while the previous frame upload is in flight.
          if (!video.current || !onlineRef.current || pendingRef.current > 0 || mediaBusy.current) return
          const jpeg = await captureFrame(video.current)
          if (!jpeg) return
          mediaBusy.current = true
          await studentApi.frame(jpeg).catch(() => undefined)
          mediaBusy.current = false
        }, 5000),
      )
    }
    if (wantAudio && stream.getAudioTracks().length) {
      timers.push(
        setInterval(async () => {
          if (!onlineRef.current || pendingRef.current > 0 || mediaBusy.current) return
          const pcm = await recordClip(stream, 1).catch(() => null)
          if (!pcm) return
          mediaBusy.current = true
          await studentApi.audio(pcm).catch(() => undefined)
          mediaBusy.current = false
        }, 20000),
      )
    }
    return () => timers.forEach(clearInterval)
  }, [stream, wantVideo, wantAudio])

  // Heartbeat keeps the device lock, picks up time extensions and proctoring
  // changes, and detects when the connection comes back (every 30 s online,
  // every 10 s while offline so recovery is quick).
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>
    let stopped = false
    const beat = async () => {
      try {
        const r = await studentApi.heartbeat()
        setEndsAt(r.ends_at)
        if (r.proctoring) setProc((cur) => (JSON.stringify(cur) === JSON.stringify(r.proctoring) ? cur : r.proctoring))
        if (!onlineRef.current) {
          setOnline(true)
          void queue.flush()
        }
      } catch (e) {
        if (isNetworkError(e)) setOnline(false)
        else if (e instanceof ApiError) {
          if (e.status === 410) onDone(true)
          else if (e.code === 'SESSION_LOCK_LOST' || e.code === 'DEVICE_MISMATCH' || e.status === 401) onLocked(lockFrom(e))
        }
      }
      if (!stopped) timer = setTimeout(beat, onlineRef.current ? 30000 : 10000)
    }
    void beat()
    return () => {
      stopped = true
      clearTimeout(timer)
    }
  }, [onDone, onLocked, queue])

  // Warn before closing the tab mid-exam.
  useEffect(() => {
    const h = (e: BeforeUnloadEvent) => {
      if (!finished.current) e.preventDefault()
    }
    window.addEventListener('beforeunload', h)
    return () => window.removeEventListener('beforeunload', h)
  }, [])

  const onAnswer = useCallback(
    (qid: number, value: string) => {
      setAnswers((a) => ({ ...a, [String(qid)]: value }))
      queue.enqueue(qid, value)
    },
    [queue],
  )
  // Every keystroke is written to this computer's disk at once; the
  // debounced onAnswer then sends it.
  const onDraft = useCallback((qid: number, value: string) => queue.stash(qid, value), [queue])

  const answeredCount = initial.questions.filter((q) => (answers[String(q.id)] ?? '') !== '').length

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="sticky top-0 z-10 border-b border-slate-200 bg-white/95 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <div className="min-w-0">
            <p className="truncate font-semibold text-slate-900">{initial.exam_title}</p>
            <p className="truncate text-xs text-slate-500">
              {initial.student.full_name} · <span className="font-mono">{initial.student.matric_no}</span>
            </p>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden text-xs text-slate-500 sm:inline" aria-live="polite">
              {!online ? `Offline · ${pending} saved on this computer` : pending > 0 ? `Saving ${pending}…` : 'All answers saved'}
            </span>
            <CountdownTimer endsAt={endsAt} offsetMs={offsetMs} onExpire={() => void finish(true)} />
            {wantVideo && <WebcamFeed ref={video} stream={stream} className="h-12 w-16" />}
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-6">
        {!online && (
          <div role="status" className="mb-4 flex items-start gap-3 rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-900 ring-1 ring-amber-200">
            <span aria-hidden className="mt-1 h-2.5 w-2.5 shrink-0 animate-pulse rounded-full bg-amber-500" />
            <span>
              <b>Connection lost — keep working.</b> Every answer is saved on this computer
              {pending > 0 ? ` (${pending} waiting to send)` : ''} and will be sent automatically when the connection returns.
              If the computer switches off, reopen this page on the same computer: your work will still be here.
            </span>
          </div>
        )}
        {wantVideo && !stream && (
          <p role="alert" className="mb-4 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
            Your camera is not on. The invigilator has been notified.
          </p>
        )}
        <Card>
          <QuestionPanel questions={initial.questions} answers={answers} onAnswer={onAnswer} onDraft={onDraft} />
        </Card>
        <div className="mt-6 flex items-center justify-between">
          <p className="text-sm text-slate-600">
            {answeredCount} of {initial.questions.length} answered
          </p>
          <Button onClick={() => setConfirming(true)} disabled={submitting}>
            Submit exam
          </Button>
        </div>
      </main>

      {proc.fullscreen_required && fs.supported && !fs.isFullscreen && !submitting && (
        <div role="dialog" aria-modal="true" aria-labelledby="fs-title" className="fixed inset-0 z-20 grid place-items-center bg-slate-900/95 px-4">
          <div className="max-w-sm text-center text-white">
            <h2 id="fs-title" className="text-xl font-semibold">
              Fullscreen is required
            </h2>
            <p className="mt-2 text-sm text-slate-300">
              The exam runs in fullscreen. Leaving fullscreen is recorded and shown to your invigilator.
            </p>
            <Button className="mt-6" onClick={() => void fs.enter()}>
              Enter fullscreen
            </Button>
          </div>
        </div>
      )}

      {submitWaiting && (
        <div role="alertdialog" aria-modal="true" aria-labelledby="wait-title" className="fixed inset-0 z-40 grid place-items-center bg-slate-900/90 px-4">
          <div className="max-w-md text-center text-white">
            <div aria-hidden className="mx-auto mb-5 h-12 w-12 animate-spin rounded-full border-4 border-white/20 border-t-amber-400" />
            <h2 id="wait-title" className="text-xl font-semibold">
              Waiting for the connection to submit
            </h2>
            <p className="mt-2 text-sm text-slate-300">
              Your answers are safe on this computer{pending > 0 ? ` (${pending} still to send)` : ''}. Please don't close this window —
              your exam will be submitted automatically as soon as the exam server is reachable again.
            </p>
          </div>
        </div>
      )}

      {confirming && (
        <div role="dialog" aria-modal="true" aria-labelledby="confirm-title" className="fixed inset-0 z-30 grid place-items-center bg-slate-900/60 px-4">
          <Card className="max-w-sm">
            <h2 id="confirm-title" className="text-lg font-semibold">
              Submit your exam?
            </h2>
            <p className="mt-2 text-sm text-slate-600">
              You answered {answeredCount} of {initial.questions.length} questions. You cannot change answers after submitting.
            </p>
            {pending > 0 && <p className="mt-2 text-sm text-amber-700">{pending} answer(s) are still saving and will be sent first.</p>}
            <div className="mt-5 flex justify-end gap-2">
              <Button variant="secondary" onClick={() => setConfirming(false)} disabled={submitting}>
                Keep working
              </Button>
              <Button onClick={() => void finish(false)} disabled={submitting}>
                {submitting ? 'Submitting…' : 'Submit'}
              </Button>
            </div>
          </Card>
        </div>
      )}
    </div>
  )
}
