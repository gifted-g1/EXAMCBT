import { useCallback, useEffect, useState } from 'react'
import { staffApi } from '../../api/staff'
import type { Exam, QuestionStat, Report } from '../../api/types'
import { Button, Card, ErrorText, Spinner, StatusBadge, errorMessage } from '../ui'
import { AnswerSheet } from './AnswerSheet'

export function ReportView({ exam }: { exam: Exam }) {
  const [rep, setRep] = useState<Report | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [sheet, setSheet] = useState<number | null>(null)
  const closeSheet = useCallback(() => setSheet(null), [])

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
    <div className="space-y-6">
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
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-slate-500">
          Click <b>View answers</b> to see what a student got right and missed. Integrity scores are prompts for review, not verdicts.
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
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {rows.map((r) => (
              <tr key={r.matric_no} className="hover:bg-slate-50">
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
                <td className="px-4 py-3 text-right">
                  <Button variant="secondary" className="px-3 py-1 text-xs" onClick={() => setSheet(r.session_id)}>
                    View answers
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && <p className="p-4 text-sm text-slate-500">No sessions yet.</p>}
      </div>

      <QuestionAnalysis examId={exam.id} />
      {sheet != null && <AnswerSheet sessionId={sheet} onClose={closeSheet} />}
    </div>
  )
}

const letter = (i: number) => String.fromCharCode(65 + i)

/** How the whole class did on each question; hardest questions stand out. */
function QuestionAnalysis({ examId }: { examId: number }) {
  const [data, setData] = useState<{ candidates: number; questions: QuestionStat[] } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [hardestFirst, setHardestFirst] = useState(false)

  useEffect(() => {
    staffApi.itemAnalysis(examId).then(
      (r) => setData({ candidates: r.candidates, questions: r.questions ?? [] }),
      (e) => setError(errorMessage(e)),
    )
  }, [examId])

  if (error) return <ErrorText>{error}</ErrorText>
  if (!data) return <Spinner />
  if (data.questions.length === 0) return null

  const list = [...data.questions]
  if (hardestFirst)
    list.sort((a, b) => (a.percent_right ?? 101) - (b.percent_right ?? 101))

  return (
    <section aria-labelledby="qa-title" className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h3 id="qa-title" className="text-lg font-semibold text-slate-900">
            Question analysis
          </h3>
          <p className="text-sm text-slate-500">
            How the {data.candidates} candidate{data.candidates === 1 ? '' : 's'} did on each question.
          </p>
        </div>
        <label className="flex items-center gap-2 text-sm text-slate-600">
          <input type="checkbox" checked={hardestFirst} onChange={(e) => setHardestFirst(e.target.checked)} />
          Most missed first
        </label>
      </div>
      <div className="space-y-2">
        {list.map((q) => {
          const total = Math.max(1, q.correct + q.incorrect + q.unanswered)
          const pct = q.percent_right
          return (
            <Card key={q.question_id} className="!p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <p className="min-w-0 flex-1 text-sm font-medium text-slate-900">
                  <span className="mr-2 text-slate-400">Q{q.number}.</span>
                  {q.prompt}
                </p>
                {q.kind === 'MCQ' ? (
                  <span
                    className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${
                      pct == null ? 'bg-slate-100 text-slate-600' : pct >= 70 ? 'bg-emerald-100 text-emerald-800' : pct >= 40 ? 'bg-amber-100 text-amber-800' : 'bg-rose-100 text-rose-800'
                    }`}
                  >
                    {pct == null ? 'no data' : `${Math.round(pct)}% right`}
                  </span>
                ) : (
                  <span className="rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-semibold text-amber-800">written · {q.answered} answered</span>
                )}
              </div>
              {q.kind === 'MCQ' && (
                <>
                  <div className="mt-3 flex h-2.5 overflow-hidden rounded-full bg-slate-100" aria-hidden>
                    <div className="bg-emerald-500" style={{ width: `${(q.correct / total) * 100}%` }} />
                    <div className="bg-rose-500" style={{ width: `${(q.incorrect / total) * 100}%` }} />
                    <div className="bg-slate-300" style={{ width: `${(q.unanswered / total) * 100}%` }} />
                  </div>
                  <p className="mt-2 text-xs text-slate-500">
                    <span className="text-emerald-700">{q.correct} right</span> · <span className="text-rose-700">{q.incorrect} wrong</span> ·{' '}
                    {q.unanswered} skipped
                    {q.correct_option != null && (
                      <>
                        {' '}
                        · answer <b>{letter(q.correct_option)}</b>
                      </>
                    )}
                    {q.common_wrong != null && q.options && (
                      <>
                        {' '}
                        · most chosen wrong: <b>{letter(q.common_wrong)}</b> “{q.options[q.common_wrong]}” ({q.option_counts?.[q.common_wrong]})
                      </>
                    )}
                  </p>
                </>
              )}
            </Card>
          )
        })}
      </div>
    </section>
  )
}
