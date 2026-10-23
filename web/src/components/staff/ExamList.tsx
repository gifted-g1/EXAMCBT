import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { staffApi } from '../../api/staff'
import type { Exam, User } from '../../api/types'
import { Button, Card, ErrorText, Field, Spinner, StatusBadge, errorMessage, inputClass } from '../ui'

const toLocalInput = (d: Date) => {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function ExamList({ user }: { user: User }) {
  const [exams, setExams] = useState<Exam[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  const load = useCallback(() => {
    staffApi.exams().then(setExams, (e) => setError(errorMessage(e)))
  }, [])
  useEffect(load, [load])

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold text-slate-900">Exams</h1>
        <Button onClick={() => setCreating((c) => !c)}>{creating ? 'Cancel' : 'New exam'}</Button>
      </div>
      {creating && <CreateExam onCreated={() => setCreating(false)} />}
      <ErrorText>{error}</ErrorText>
      {!exams ? (
        <Spinner />
      ) : exams.length === 0 ? (
        <Card>No exams yet.</Card>
      ) : (
        <div className="overflow-x-auto rounded-xl bg-white shadow-sm ring-1 ring-slate-200">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">Title</th>
                <th className="px-4 py-3">Window</th>
                <th className="px-4 py-3">Network</th>
                <th className="px-4 py-3">Questions</th>
                <th className="px-4 py-3">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {exams.map((e) => (
                <tr key={e.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3">
                    <Link to={`/staff/exams/${e.id}`} className="font-medium text-brand-700 hover:underline">
                      {e.title}
                    </Link>
                    {e.created_by === user.id && <span className="ml-2 text-xs text-slate-400">yours</span>}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-slate-600">
                    {new Date(e.starts_at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })} –{' '}
                    {new Date(e.ends_at).toLocaleTimeString([], { timeStyle: 'short' })}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-slate-600">{e.lan_cidr}</td>
                  <td className="px-4 py-3 text-slate-600">{e.questions_count}</td>
                  <td className="px-4 py-3">
                    <StatusBadge status={e.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function CreateExam({ onCreated }: { onCreated: () => void }) {
  const navigate = useNavigate()
  const now = new Date()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [start, setStart] = useState(toLocalInput(now))
  const [end, setEnd] = useState(toLocalInput(new Date(now.getTime() + 2 * 3600_000)))
  const [cidr, setCidr] = useState('')
  const [pin, setPin] = useState(true)
  const [duration, setDuration] = useState('')
  const [perStudent, setPerStudent] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const exam = await staffApi.createExam({
        title,
        description,
        starts_at: new Date(start).toISOString(),
        ends_at: new Date(end).toISOString(),
        lan_cidr: cidr.trim(),
        require_pin: pin,
        duration_minutes: duration.trim() === '' ? 0 : Number(duration),
        questions_per_student: perStudent.trim() === '' ? 0 : Number(perStudent),
      })
      onCreated()
      navigate(`/staff/exams/${exam.id}`)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <form onSubmit={submit} className="grid gap-4 md:grid-cols-2">
        <Field label="Title">
          <input className={inputClass} value={title} onChange={(e) => setTitle(e.target.value)} required minLength={3} maxLength={150} />
        </Field>
        <Field label="Exam network (CIDR)" hint="Leave blank to use this server's LAN, e.g. 192.168.1.0/24">
          <input className={`${inputClass} font-mono`} value={cidr} onChange={(e) => setCidr(e.target.value)} placeholder="auto" />
        </Field>
        <Field label="Starts">
          <input type="datetime-local" className={inputClass} value={start} onChange={(e) => setStart(e.target.value)} required />
        </Field>
        <Field label="Ends">
          <input type="datetime-local" className={inputClass} value={end} onChange={(e) => setEnd(e.target.value)} required />
        </Field>
        <Field label="Time limit (minutes)" hint="Each student gets this long from when they start, e.g. 25. Leave blank to allow the whole window.">
          <input
            type="number"
            min={1}
            max={720}
            className={`${inputClass} w-32`}
            value={duration}
            onChange={(e) => setDuration(e.target.value)}
            placeholder="none"
          />
        </Field>
        <Field label="Questions per student" hint="Each student gets this many random questions from the exam's question bank, e.g. 30. Leave blank to give everyone every question.">
          <input
            type="number"
            min={1}
            max={500}
            className={`${inputClass} w-32`}
            value={perStudent}
            onChange={(e) => setPerStudent(e.target.value)}
            placeholder="all"
          />
        </Field>
        <Field label="Instructions (shown to students)">
          <textarea className={inputClass} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        </Field>
        <label className="flex items-center gap-2 self-end text-sm">
          <input type="checkbox" checked={pin} onChange={(e) => setPin(e.target.checked)} />
          Require a 4-digit PIN (recommended)
        </label>
        <div className="md:col-span-2">
          <ErrorText>{error}</ErrorText>
          <Button type="submit" className="mt-2" disabled={busy}>
            {busy ? 'Creating…' : 'Create exam'}
          </Button>
        </div>
      </form>
    </Card>
  )
}
