import { useEffect, useState } from 'react'
import { staffApi } from '../../api/staff'
import type { Exam, Report } from '../../api/types'
import { Button, Card, ErrorText, Spinner, StatusBadge, errorMessage } from '../ui'

export function ReportView({ exam }: { exam: Exam }) {
  const [rep, setRep] = useState<Report | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    staffApi.report(exam.id).then(setRep, (e) => setError(errorMessage(e)))
  }, [exam.id])

  const download = async () => {
    const url = await staffApi.reportCSV(exam.id)
    if (!url) return setError('Could not download the CSV.')
    const a = document.createElement('a')
    a.href = url
    a.download = `exam-${exam.id}-report.csv`
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }

  if (error) return <ErrorText>{error}</ErrorText>
  if (!rep) return <Spinner />
  const rows = rep.rows ?? []
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
        {[
          ['Candidates', rep.summary.candidates],
          ['Submitted', rep.summary.submitted],
          ['Avg score', rep.summary.average_score],
          ['Avg integrity', rep.summary.average_integrity],
          ['High alerts', rep.summary.flagged_high],
        ].map(([k, v]) => (
          <Card key={k as string} className="!p-4">
            <p className="text-xs text-slate-500">{k}</p>
            <p className="text-2xl font-semibold tabular-nums">{v}</p>
          </Card>
        ))}
      </div>
      <div className="flex justify-between">
        <p className="text-sm text-slate-500">
          Integrity scores are prompts for review, not verdicts. Written answers need manual marking.
        </p>
        <Button variant="secondary" onClick={download}>
          Download CSV
        </Button>
      </div>
      <div className="overflow-x-auto rounded-xl bg-white shadow-sm ring-1 ring-slate-200">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-4 py-3">Student</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3 text-right">Score</th>
              <th className="px-4 py-3 text-right">Integrity</th>
              <th className="px-4 py-3 text-right">Alerts</th>
              <th className="px-4 py-3 text-right">Face match</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {rows.map((r) => (
              <tr key={r.matric_no}>
                <td className="px-4 py-3">
                  <p className="font-medium">{r.full_name}</p>
                  <p className="font-mono text-xs text-slate-500">{r.matric_no}</p>
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={r.status} />
                  {r.auto_submitted && <span className="ml-1 text-xs text-slate-500">auto</span>}
                </td>
                <td className="px-4 py-3 text-right tabular-nums">
                  {r.score ?? '—'} / {r.max_score}
                </td>
                <td className="px-4 py-3 text-right tabular-nums">{Math.round(r.integrity_score)}</td>
                <td className="px-4 py-3 text-right tabular-nums">
                  {r.violations}
                  {r.high_violations > 0 && <span className="ml-1 text-rose-600">({r.high_violations} high)</span>}
                </td>
                <td className="px-4 py-3 text-right tabular-nums">{r.face_match_score?.toFixed(3) ?? '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && <p className="p-4 text-sm text-slate-500">No sessions yet.</p>}
      </div>
    </div>
  )
}
