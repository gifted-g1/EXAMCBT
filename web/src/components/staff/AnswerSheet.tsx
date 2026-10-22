import { useEffect, useState } from 'react'
import { staffApi } from '../../api/staff'
import type { AnswerScript, AnswerStatus, ScriptItem } from '../../api/types'
import { Button, ErrorText, Spinner, StatusBadge, errorMessage } from '../ui'

const letter = (i: number) => String.fromCharCode(65 + i)

const STATUS: Record<AnswerStatus, { label: string; icon: string; chip: string; ring: string }> = {
  correct: { label: 'Correct', icon: '✓', chip: 'bg-emerald-100 text-emerald-800', ring: 'ring-emerald-200' },
  incorrect: { label: 'Wrong', icon: '✗', chip: 'bg-rose-100 text-rose-800', ring: 'ring-rose-200' },
  unanswered: { label: 'Not answered', icon: '—', chip: 'bg-slate-100 text-slate-700', ring: 'ring-slate-200' },
  needs_marking: { label: 'Needs marking', icon: '✎', chip: 'bg-amber-100 text-amber-800', ring: 'ring-amber-200' },
  marked: { label: 'Marked', icon: '✎', chip: 'bg-sky-100 text-sky-800', ring: 'ring-sky-200' },
  partial: { label: 'Partly right', icon: '◐', chip: 'bg-orange-100 text-orange-800', ring: 'ring-orange-200' },
}

type Filter = 'all' | 'missed' | AnswerStatus

/** One student's marked answer sheet: what they chose vs the right answer. */
export function AnswerSheet({ sessionId, onClose }: { sessionId: number; onClose: () => void }) {
  const [data, setData] = useState<AnswerScript | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState<Filter>('all')

  useEffect(() => {
    staffApi.answerScript(sessionId).then(setData, (e) => setError(errorMessage(e)))
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [sessionId, onClose])

  const shown = (data?.items ?? []).filter((it) =>
    filter === 'all' ? true : filter === 'missed' ? it.status === 'incorrect' || it.status === 'unanswered' || it.status === 'partial' : it.status === filter,
  )
  const pct = data && data.auto_marked_max > 0 ? Math.round((data.score / data.auto_marked_max) * 100) : null

  return (
    <div className="fixed inset-0 z-50 flex justify-center overflow-y-auto bg-slate-900/50 p-4 print:static print:bg-white print:p-0" onClick={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Answer sheet"
        className="print-area my-4 h-fit w-full max-w-3xl rounded-2xl bg-white p-6 shadow-xl print:my-0 print:shadow-none"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-sm text-slate-500">{data?.exam_title ?? 'Answer sheet'}</p>
            <h2 className="text-xl font-semibold text-slate-900">{data?.full_name ?? '…'}</h2>
            <p className="font-mono text-sm text-slate-500">{data?.matric_no}</p>
          </div>
          <div className="flex gap-2 print:hidden">
            <Button variant="secondary" onClick={() => window.print()} disabled={!data}>
              Print
            </Button>
            <Button variant="ghost" onClick={onClose} aria-label="Close">
              ✕
            </Button>
          </div>
        </div>

        <ErrorText>{error}</ErrorText>
        {!data ? (
          !error && <Spinner />
        ) : (
          <>
            <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
              <div className="col-span-2 rounded-xl bg-slate-900 p-4 text-white sm:col-span-1">
                <p className="text-xs text-slate-400">Score</p>
                <p className="text-2xl font-bold tabular-nums">
                  {data.score}
                  <span className="text-base font-normal text-slate-400"> / {data.max_score}</span>
                </p>
                {pct != null && <p className="text-xs text-slate-400">{pct}% of marked so far</p>}
              </div>
              {(['correct', 'partial', 'incorrect', 'unanswered', 'needs_marking', 'marked'] as const).map((k) => (
                <div key={k} className={`rounded-xl p-4 ring-1 ${STATUS[k].ring}`}>
                  <p className="text-xs text-slate-500">{STATUS[k].label}</p>
                  <p className="text-2xl font-bold tabular-nums text-slate-900">{data.counts?.[k] ?? 0}</p>
                </div>
              ))}
            </div>
            <p className="mt-2 flex items-center gap-2 text-xs text-slate-500">
              Session <StatusBadge status={data.status} />
              {data.counts.needs_marking > 0 && <span>· written answers count once an examiner marks them (Marking tab)</span>}
            </p>

            <div role="group" aria-label="Filter questions" className="mt-5 flex flex-wrap gap-2 print:hidden">
              {(
                [
                  ['all', `All (${data.items.length})`],
                  ['missed', `Missed (${data.counts.incorrect + data.counts.unanswered + (data.counts.partial ?? 0)})`],
                  ['correct', `Correct (${data.counts.correct})`],
                  ['needs_marking', `Needs marking (${data.counts.needs_marking})`],
                  ['marked', `Marked (${data.counts.marked ?? 0})`],
                ] as [Filter, string][]
              ).map(([k, label]) => (
                <button
                  key={k}
                  type="button"
                  aria-pressed={filter === k}
                  onClick={() => setFilter(k)}
                  className={`rounded-full px-3 py-1 text-sm ring-1 ${
                    filter === k ? 'bg-brand-600 text-white ring-brand-600' : 'bg-white text-slate-700 ring-slate-300 hover:bg-slate-50'
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>

            <ol className="mt-4 space-y-3">
              {shown.map((it) => (
                <ScriptRow key={it.question_id} item={it} />
              ))}
              {shown.length === 0 && <p className="text-sm text-slate-500">No questions in this view.</p>}
            </ol>
          </>
        )}
      </div>
    </div>
  )
}

function ScriptRow({ item }: { item: ScriptItem }) {
  const s = STATUS[item.status]
  const chosen = item.response !== '' ? Number(item.response) : null
  return (
    <li className={`break-inside-avoid rounded-xl p-4 ring-1 ${s.ring}`}>
      <div className="flex items-start justify-between gap-3">
        <p className="font-medium text-slate-900">
          <span className="mr-2 text-slate-400">Q{item.number}.</span>
          <span className="whitespace-pre-wrap">{item.prompt}</span>
        </p>
        <span className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ${s.chip}`}>
          <span aria-hidden>{s.icon}</span> {s.label}
        </span>
      </div>
      <p className="mt-1 text-xs text-slate-500">
        {item.awarded != null ? `${item.awarded} / ${item.marks}` : `— / ${item.marks}`} mark{item.marks === 1 ? '' : 's'}
      </p>

      {item.kind === 'MCQ' ? (
        <ul className="mt-3 space-y-1.5 text-sm">
          {(item.options ?? []).map((o, oi) => {
            const isCorrect = item.correct_option === oi
            const isChosen = chosen === oi
            return (
              <li
                key={oi}
                className={`flex items-center gap-2 rounded-lg px-3 py-1.5 ${
                  isCorrect ? 'bg-emerald-50 text-emerald-900' : isChosen ? 'bg-rose-50 text-rose-900' : 'text-slate-600'
                }`}
              >
                <span className="w-5 font-semibold">{letter(oi)}.</span>
                <span className="flex-1">{o}</span>
                {isChosen && <span className="text-xs font-semibold">{isCorrect ? 'their answer ✓' : 'their answer'}</span>}
                {isCorrect && !isChosen && <span className="text-xs font-semibold">correct answer</span>}
              </li>
            )
          })}
        </ul>
      ) : item.kind === 'GAP' ? (
        <ol className="mt-3 space-y-1.5 text-sm">
          {(item.blanks ?? []).map((accepted, bi) => {
            const got = item.blank_answers?.[bi] ?? ''
            const ok = item.blank_correct?.[bi] ?? false
            return (
              <li key={bi} className={`flex flex-wrap items-center gap-2 rounded-lg px-3 py-1.5 ${ok ? 'bg-emerald-50 text-emerald-900' : 'bg-rose-50 text-rose-900'}`}>
                <span className="font-semibold">Blank {bi + 1}:</span>
                <span>{got || <span className="italic opacity-70">blank</span>}</span>
                <span className="text-xs font-semibold">{ok ? '✓' : `✗ expected ${accepted.join(' / ')}`}</span>
              </li>
            )
          })}
        </ol>
      ) : (
        <div className="mt-3 whitespace-pre-wrap rounded-lg bg-slate-50 p-3 text-sm text-slate-800">
          {item.response || <span className="italic text-slate-400">No answer written.</span>}
        </div>
      )}
      {item.comment && (
        <p className="mt-2 rounded-lg bg-sky-50 px-3 py-2 text-sm text-sky-900">
          <span className="font-semibold">Examiner: </span>
          {item.comment}
        </p>
      )}
    </li>
  )
}
