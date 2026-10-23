import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { staffApi } from '../../api/staff'
import type { AccessCard, Exam, HubEvent, SessionRow, User, Violation } from '../../api/types'
import { useSocket } from '../../hooks/useSocket'
import { AlertFeed } from '../AlertFeed'
import { IPShareCard } from '../IPShareCard'
import { StudentTile } from '../StudentTile'
import { Button, Card, ErrorText, Spinner, StatusBadge, errorMessage } from '../ui'
import { MarkingView } from './MarkingView'
import { ProctoringSettings } from './ProctoringSettings'
import { QuestionsEditor } from './QuestionsEditor'
import { ReportView } from './ReportView'
import { SessionDrawer } from './SessionDrawer'

type Tab = 'live' | 'access' | 'proctoring' | 'questions' | 'marking' | 'report'

export function ExamDetail({ user }: { user: User }) {
  const { id } = useParams()
  const examId = Number(id)
  const navigate = useNavigate()
  const [exam, setExam] = useState<Exam | null>(null)
  const [canManage, setCanManage] = useState(false)
  const [tab, setTab] = useState<Tab>('live')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(() => {
    staffApi.exam(examId).then(
      (r) => {
        setExam(r.exam)
        setCanManage(r.can_manage)
      },
      (e) => setError(errorMessage(e)),
    )
  }, [examId])
  useEffect(load, [load])

  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    setError(null)
    try {
      await fn()
      load()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  if (!exam) return error ? <ErrorText>{error}</ErrorText> : <Spinner />

  const tabs: [Tab, string][] = [
    ['live', 'Live monitor'],
    ['access', 'Student access'],
    ['proctoring', 'Proctoring'],
    ['questions', `Questions (${exam.questions_count})`],
    ['marking', 'Marking'],
    ['report', 'Report'],
  ]

  return (
    <div className="space-y-6">
      <div>
        <Link to="/staff/exams" className="text-sm text-brand-700 hover:underline">
          ← All exams
        </Link>
        <div className="mt-2 flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-slate-900">{exam.title}</h1>
            <p className="mt-1 flex flex-wrap items-center gap-2 text-sm text-slate-600">
              <StatusBadge status={exam.status} />
              {new Date(exam.starts_at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })} –{' '}
              {new Date(exam.ends_at).toLocaleTimeString([], { timeStyle: 'short' })}
              <span className="font-mono text-xs">{exam.lan_cidr}</span>
            </p>
            <ExamSettings exam={exam} canEdit={canManage && exam.status !== 'CLOSED'} onSaved={setExam} />
          </div>
          {canManage && (
            <div className="flex flex-wrap gap-2">
              {exam.status === 'DRAFT' && (
                <Button variant="secondary" disabled={busy} onClick={() => act(() => staffApi.setStatus(examId, 'schedule'))}>
                  Schedule
                </Button>
              )}
              {(exam.status === 'DRAFT' || exam.status === 'SCHEDULED') && (
                <Button disabled={busy} onClick={() => act(() => staffApi.setStatus(examId, 'open'))}>
                  Open exam now
                </Button>
              )}
              {(exam.status === 'ACTIVE' || exam.status === 'SCHEDULED') && (
                <Button
                  variant="danger"
                  disabled={busy}
                  onClick={() =>
                    window.confirm('Close the exam? Every open session is submitted automatically.') &&
                    act(() => staffApi.setStatus(examId, 'close'))
                  }
                >
                  Close exam
                </Button>
              )}
              {(exam.status === 'DRAFT' || exam.status === 'CLOSED') && (
                <Button
                  variant="ghost"
                  disabled={busy}
                  onClick={() =>
                    window.confirm('Delete this exam and all its sessions permanently?') &&
                    act(async () => {
                      await staffApi.deleteExam(examId)
                      navigate('/staff/exams')
                    })
                  }
                >
                  Delete
                </Button>
              )}
            </div>
          )}
        </div>
        <ErrorText>{error}</ErrorText>
      </div>

      <div role="tablist" className="flex gap-1 overflow-x-auto border-b border-slate-200">
        {tabs.map(([k, label]) => (
          <button
            key={k}
            role="tab"
            type="button"
            aria-selected={tab === k}
            onClick={() => setTab(k)}
            className={`-mb-px whitespace-nowrap border-b-2 px-4 py-2 text-sm font-medium ${
              tab === k ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500 hover:text-slate-800'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === 'live' && <LiveMonitor exam={exam} canManage={canManage} />}
      {tab === 'access' && <AccessTab exam={exam} canManage={canManage} onChanged={load} />}
      {tab === 'proctoring' && <ProctoringSettings exam={exam} canManage={canManage} onChanged={setExam} />}
      {tab === 'questions' && <QuestionsEditor exam={exam} canManage={canManage} onChanged={load} />}
      {tab === 'marking' && <MarkingView exam={exam} canManage={canManage} />}
      {tab === 'report' && <ReportView exam={exam} />}
      <p className="sr-only">Signed in as {user.full_name}</p>
    </div>
  )
}

function AccessTab({ exam, canManage, onChanged }: { exam: Exam; canManage: boolean; onChanged: () => void }) {
  const [card, setCard] = useState<AccessCard | null>(null)
  const [error, setError] = useState<string | null>(null)
  const load = useCallback(() => {
    staffApi.accessCard(exam.id).then(setCard, (e) => setError(errorMessage(e)))
  }, [exam.id])
  useEffect(load, [load, exam.pin_code])

  const pin = async (enabled: boolean) => {
    try {
      await staffApi.setPin(exam.id, enabled)
      onChanged()
      load()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  return (
    <div className="space-y-4">
      <ErrorText>{error}</ErrorText>
      {card ? <IPShareCard card={card} canManage={canManage} onRotatePin={() => pin(true)} onClearPin={() => pin(false)} /> : <Spinner />}
      <Card className="text-sm text-slate-600">
        <p className="font-semibold text-slate-800">How students get in</p>
        <ol className="mt-2 list-decimal space-y-1 pl-5">
          <li>Students connect to the exam hall Wi-Fi / LAN ({exam.lan_cidr}).</li>
          <li>They type the address above (or scan the QR code). Devices outside the hall network are refused.</li>
          <li>They enter the PIN, their matric number, take a selfie (and optionally a photo of their ID card).</li>
          <li>After a face match against the roster photo, the exam opens in fullscreen. There is no student dashboard.</li>
        </ol>
      </Card>
    </div>
  )
}

function LiveMonitor({ exam, canManage }: { exam: Exam; canManage: boolean }) {
  const [rows, setRows] = useState<SessionRow[]>([])
  const [alerts, setAlerts] = useState<Violation[]>([])
  const [selected, setSelected] = useState<number | null>(null)
  const [now, setNow] = useState(Date.now())
  const [notice, setNotice] = useState<string | null>(null)
  const rowsRef = useRef(rows)
  rowsRef.current = rows
  const reloadTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const loadRows = useCallback(() => {
    staffApi.sessions(exam.id).then(
      (r) => {
        setRows(r.sessions ?? [])
        setNow(new Date(r.server_time).getTime())
      },
      () => undefined,
    )
  }, [exam.id])
  const loadAlerts = useCallback(() => {
    staffApi.violations(exam.id).then(setAlerts, () => undefined)
  }, [exam.id])

  useEffect(() => {
    loadRows()
    loadAlerts()
    const id = setInterval(() => {
      loadRows()
      setNow(Date.now())
    }, 10000)
    return () => {
      clearInterval(id)
      if (reloadTimer.current) clearTimeout(reloadTimer.current)
    }
  }, [loadRows, loadAlerts])

  // Coalesce bursts of hub events into one reload.
  const scheduleReload = useCallback(() => {
    if (reloadTimer.current) return
    reloadTimer.current = setTimeout(() => {
      reloadTimer.current = null
      loadRows()
    }, 800)
  }, [loadRows])

  const onEvent = useCallback(
    (ev: HubEvent) => {
      if (ev.type === 'violation') {
        const v = ev.data.violation as Violation
        const row = rowsRef.current.find((r) => r.id === v.session_id)
        setAlerts((a) => [{ ...v, matric_no: row?.matric_no, full_name: row?.full_name }, ...a].slice(0, 300))
        setRows((rs) =>
          rs.map((r) =>
            r.id === v.session_id
              ? {
                  ...r,
                  integrity_score: Number(ev.data.integrity_score ?? r.integrity_score),
                  violations_count: r.violations_count + 1,
                  high_count: r.high_count + (v.severity === 'HIGH' || v.severity === 'CRITICAL' ? 1 : 0),
                }
              : r,
          ),
        )
      } else if (ev.type === 'verify_failed') {
        setNotice(`${String(ev.data.full_name)} (${String(ev.data.matric_no)}) failed face verification ${String(ev.data.attempts)} times — please check in person.`)
      } else {
        scheduleReload()
      }
    },
    [scheduleReload],
  )
  const status = useSocket(exam.id, onEvent)

  const review = async (v: Violation, st: 'CONFIRMED' | 'DISMISSED') => {
    try {
      const r = await staffApi.review(exam.id, v.id, st)
      setAlerts((a) => a.map((x) => (x.id === v.id ? { ...x, review_status: st } : x)))
      setRows((rs) => rs.map((x) => (x.id === v.session_id ? { ...x, integrity_score: r.integrity_score } : x)))
    } catch {
      /* surfaced on next reload */
    }
  }

  const counts = useMemo(
    () => ({
      active: rows.filter((r) => r.status === 'ACTIVE').length,
      submitted: rows.filter((r) => r.status === 'SUBMITTED').length,
      flagged: rows.filter((r) => r.high_count > 0).length,
    }),
    [rows],
  )

  return (
    <div className="grid gap-6 xl:grid-cols-[1fr_22rem]">
      <section aria-label="Live student grid" className="space-y-4">
        <div className="flex flex-wrap items-center gap-4 text-sm text-slate-600">
          <span>
            <b className="text-slate-900">{counts.active}</b> writing
          </span>
          <span>
            <b className="text-slate-900">{counts.submitted}</b> submitted
          </span>
          <span>
            <b className="text-rose-700">{counts.flagged}</b> with high-severity alerts
          </span>
          <span className="ml-auto inline-flex items-center gap-1.5 text-xs">
            <span className={`h-2 w-2 rounded-full ${status === 'open' ? 'bg-emerald-500' : 'bg-amber-500'}`} />
            {status === 'open' ? 'Live' : 'Reconnecting…'}
          </span>
        </div>
        {notice && (
          <p role="alert" className="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
            {notice}{' '}
            <button type="button" className="underline" onClick={() => setNotice(null)}>
              Dismiss
            </button>
          </p>
        )}
        {rows.length === 0 ? (
          <Card className="text-sm text-slate-500">
            {exam.status === 'ACTIVE' ? 'Waiting for students to join…' : 'No student sessions.'}
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 2xl:grid-cols-4">
            {rows.map((r) => (
              <StudentTile key={r.id} row={r} now={now} onSelect={setSelected} cameraOn={exam.proctoring?.camera_monitoring !== false} />
            ))}
          </div>
        )}
      </section>
      <aside className="max-h-[80vh] overflow-y-auto rounded-xl bg-white shadow-sm ring-1 ring-slate-200">
        <h2 className="sticky top-0 border-b border-slate-100 bg-white px-4 py-3 font-semibold">Alerts</h2>
        <AlertFeed items={alerts} onReview={review} onSelectSession={setSelected} />
      </aside>
      {selected != null && (
        <SessionDrawer
          sessionId={selected}
          examId={exam.id}
          canManage={canManage}
          onClose={() => setSelected(null)}
          onChanged={() => {
            loadRows()
            loadAlerts()
          }}
        />
      )}
    </div>
  )
}

/** Shows and edits the time limit and how many random questions each student gets. */
function ExamSettings({ exam, canEdit, onSaved }: { exam: Exam; canEdit: boolean; onSaved: (e: Exam) => void }) {
  const [editing, setEditing] = useState(false)
  const [minutes, setMinutes] = useState(exam.duration_minutes ? String(exam.duration_minutes) : '')
  const [perStudent, setPerStudent] = useState(exam.questions_per_student ? String(exam.questions_per_student) : '')
  const [error, setError] = useState<string | null>(null)
  const windowMin = Math.round((new Date(exam.ends_at).getTime() - new Date(exam.starts_at).getTime()) / 60000)
  const pool = exam.questions_count
  const drawn = exam.questions_per_student > 0 && exam.questions_per_student < pool

  const save = async () => {
    setError(null)
    try {
      const updated = await staffApi.updateExam(exam.id, {
        title: exam.title,
        description: exam.description,
        starts_at: exam.starts_at,
        ends_at: exam.ends_at,
        lan_cidr: exam.lan_cidr,
        duration_minutes: minutes.trim() === '' ? 0 : Number(minutes),
        questions_per_student: perStudent.trim() === '' ? 0 : Number(perStudent),
      })
      onSaved({ ...exam, ...updated, questions_count: exam.questions_count })
      setEditing(false)
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  if (!editing)
    return (
      <p className="mt-1 text-sm text-slate-600">
        Time limit:{' '}
        <b>{exam.duration_minutes ? `${exam.duration_minutes} minutes per student` : `none (whole ${windowMin}-minute window)`}</b>
        {' · '}Questions:{' '}
        <b>
          {drawn
            ? `${exam.questions_per_student} random questions per student from ${pool}`
            : `all ${pool} for every student`}
        </b>
        {exam.questions_per_student > pool && pool > 0 && (
          <span className="ml-1 text-amber-700">(only {pool} in the bank, so everyone gets all)</span>
        )}
        {canEdit && (
          <button type="button" className="ml-2 text-brand-700 underline" onClick={() => setEditing(true)}>
            Change
          </button>
        )}
      </p>
    )
  return (
    <div className="mt-2 flex flex-wrap items-center gap-2 text-sm">
      <label htmlFor="time-limit">Time limit (minutes)</label>
      <input
        id="time-limit"
        type="number"
        min={1}
        max={windowMin}
        className="w-24 rounded-md border-0 px-2 py-1 ring-1 ring-slate-300"
        value={minutes}
        placeholder="none"
        onChange={(e) => setMinutes(e.target.value)}
      />
      <label htmlFor="per-student">Questions per student</label>
      <input
        id="per-student"
        type="number"
        min={1}
        max={500}
        className="w-24 rounded-md border-0 px-2 py-1 ring-1 ring-slate-300"
        value={perStudent}
        placeholder="all"
        onChange={(e) => setPerStudent(e.target.value)}
      />
      <span className="text-slate-500">of {pool}</span>
      <Button className="px-3 py-1" onClick={save}>
        Save
      </Button>
      <Button variant="ghost" className="px-3 py-1" onClick={() => setEditing(false)}>
        Cancel
      </Button>
      {error && <span className="text-rose-600">{error}</span>}
    </div>
  )
}
