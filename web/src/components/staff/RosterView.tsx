import { useCallback, useEffect, useState } from 'react'
import { staffApi } from '../../api/staff'
import type { Student, User } from '../../api/types'
import { Button, Card, ErrorText, Spinner, errorMessage, inputClass } from '../ui'

export function RosterView({ user }: { user: User }) {
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<{ students: Student[]; total: number; per_page: number } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)

  const load = useCallback(() => {
    staffApi.students(q, page).then(
      (r) => setData({ students: r.students ?? [], total: r.total, per_page: r.per_page }),
      (e) => setError(errorMessage(e)),
    )
  }, [q, page])
  useEffect(() => {
    const t = setTimeout(load, 250)
    return () => clearTimeout(t)
  }, [load])

  const importCSV = async (f: File | undefined) => {
    if (!f) return
    setError(null)
    try {
      const r = await staffApi.importRoster(f)
      setMessage(
        `Imported ${r.imported} student(s).` +
          (r.rejected.length ? ` Rejected: ${r.rejected.map((x) => `line ${x.line} (${x.error})`).join('; ')}` : ''),
      )
      load()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  const uploadPhotos = async (files: FileList | null) => {
    if (!files?.length) return
    setError(null)
    try {
      const r = await staffApi.uploadPhotos(files)
      const ok = r.results.filter((x) => x.status === 'ok').length
      const bad = r.results.filter((x) => x.status !== 'ok')
      setMessage(`${ok} photo(s) saved.` + (bad.length ? ` Problems: ${bad.map((b) => `${b.file}: ${b.status}`).join('; ')}` : ''))
      load()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  const erase = async (s: Student) => {
    if (!window.confirm(`Permanently erase ${s.full_name} (${s.matric_no}) and ALL their exam records, photos and recordings?`)) return
    try {
      await staffApi.deleteStudent(s.id)
      load()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  const pages = data ? Math.max(1, Math.ceil(data.total / data.per_page)) : 1

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-900">Student roster</h1>
      <Card className="grid gap-4 text-sm md:grid-cols-2">
        <div>
          <p className="font-semibold">Import roster (CSV)</p>
          <p className="mt-1 text-slate-500">
            Header: <code>matric_no,full_name</code> (optional <code>department</code>). Matric format <code>IFT/22/9203</code>. Existing students are updated.
          </p>
          <input type="file" accept=".csv,text/csv" className="mt-2 text-sm" onChange={(e) => importCSV(e.target.files?.[0])} />
        </div>
        <div>
          <p className="font-semibold">Upload reference photos</p>
          <p className="mt-1 text-slate-500">
            Name each file after the matric number with dashes, e.g. <code>IFT-22-9203.jpg</code>. One clear face per photo.
          </p>
          <input type="file" accept="image/jpeg,image/png" multiple className="mt-2 text-sm" onChange={(e) => uploadPhotos(e.target.files)} />
        </div>
      </Card>
      {message && <p className="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800">{message}</p>}
      <ErrorText>{error}</ErrorText>
      <input
        className={`${inputClass} max-w-sm`}
        placeholder="Search matric number or name"
        value={q}
        onChange={(e) => {
          setQ(e.target.value)
          setPage(1)
        }}
        aria-label="Search students"
      />
      {!data ? (
        <Spinner />
      ) : (
        <div className="overflow-x-auto rounded-xl bg-white shadow-sm ring-1 ring-slate-200">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">Matric no.</th>
                <th className="px-4 py-3">Full name</th>
                <th className="px-4 py-3">Department</th>
                <th className="px-4 py-3">Photo</th>
                {user.role === 'admin' && <th className="px-4 py-3" />}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {data.students.map((s) => (
                <tr key={s.id}>
                  <td className="px-4 py-3 font-mono">{s.matric_no}</td>
                  <td className="px-4 py-3">{s.full_name}</td>
                  <td className="px-4 py-3 text-slate-600">{s.department || '—'}</td>
                  <td className="px-4 py-3">{s.photo_url ? '✓' : <span className="text-amber-600">missing</span>}</td>
                  {user.role === 'admin' && (
                    <td className="px-4 py-3 text-right">
                      <Button variant="ghost" className="text-xs text-rose-700" onClick={() => erase(s)}>
                        Erase data
                      </Button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
          <div className="flex items-center justify-between px-4 py-3 text-sm text-slate-500">
            <span>{data.total} students</span>
            <span className="flex gap-2">
              <Button variant="secondary" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                Prev
              </Button>
              <Button variant="secondary" disabled={page >= pages} onClick={() => setPage((p) => p + 1)}>
                Next
              </Button>
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
