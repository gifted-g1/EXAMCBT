import { useEffect, useState } from 'react'
import type { QuestionView } from '../api/types'
import { Button } from './ui'

export function QuestionPanel({
  questions,
  answers,
  onAnswer,
  onDraft,
}: {
  questions: QuestionView[]
  answers: Record<string, string>
  onAnswer: (questionId: number, response: string) => void
  /** Called on every keystroke so typed text is saved on this computer immediately. */
  onDraft?: (questionId: number, response: string) => void
}) {
  const [index, setIndex] = useState(0)
  const q = questions[index]
  const [draft, setDraft] = useState('')

  // Keep the TEXT draft in sync when moving between questions.
  useEffect(() => {
    if (q) setDraft(answers[String(q.id)] ?? '')
  }, [q?.id])

  // Debounced save for free-text answers.
  useEffect(() => {
    if (!q || q.kind !== 'TEXT' || draft === (answers[String(q.id)] ?? '')) return
    const t = setTimeout(() => onAnswer(q.id, draft), 500)
    return () => clearTimeout(t)
  }, [draft, q, answers, onAnswer])

  if (!q) return <p className="text-slate-500">This exam has no questions.</p>
  const answered = (id: number) => (answers[String(id)] ?? '') !== ''

  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <nav aria-label="Questions" className="order-2 lg:order-1 lg:w-48">
        <ol className="grid grid-cols-6 gap-2 lg:grid-cols-4">
          {questions.map((qq, i) => (
            <li key={qq.id}>
              <button
                type="button"
                onClick={() => setIndex(i)}
                aria-current={i === index ? 'step' : undefined}
                aria-label={`Question ${i + 1}${answered(qq.id) ? ', answered' : ''}`}
                className={`h-9 w-full rounded-md text-sm font-semibold ring-1 ring-inset ${
                  i === index
                    ? 'bg-brand-600 text-white ring-brand-600'
                    : answered(qq.id)
                      ? 'bg-emerald-50 text-emerald-800 ring-emerald-300'
                      : 'bg-white text-slate-700 ring-slate-300'
                }`}
              >
                {i + 1}
              </button>
            </li>
          ))}
        </ol>
      </nav>

      <section className="order-1 flex-1 lg:order-2" aria-labelledby="q-title">
        <p className="text-xs font-medium uppercase tracking-wide text-slate-500">
          Question {index + 1} of {questions.length} · {q.marks} mark{q.marks === 1 ? '' : 's'}
        </p>
        <h2 id="q-title" className="mt-2 whitespace-pre-wrap text-lg font-semibold text-slate-900">
          {q.prompt}
        </h2>

        {q.kind === 'MCQ' ? (
          <fieldset className="mt-5 space-y-2">
            <legend className="sr-only">Options</legend>
            {(q.options ?? []).map((opt, oi) => {
              const checked = answers[String(q.id)] === String(oi)
              return (
                <label
                  key={oi}
                  className={`flex cursor-pointer items-start gap-3 rounded-lg p-3 ring-1 ring-inset ${
                    checked ? 'bg-brand-50 ring-brand-500' : 'bg-white ring-slate-200 hover:bg-slate-50'
                  }`}
                >
                  <input
                    type="radio"
                    name={`q-${q.id}`}
                    className="mt-1"
                    checked={checked}
                    onChange={() => onAnswer(q.id, String(oi))}
                  />
                  <span>
                    <span className="mr-2 font-semibold text-slate-500">{String.fromCharCode(65 + oi)}.</span>
                    {opt}
                  </span>
                </label>
              )
            })}
          </fieldset>
        ) : (
          <textarea
            aria-label="Your answer"
            className="mt-5 block min-h-[12rem] w-full rounded-lg border-0 p-3 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"
            value={draft}
            maxLength={20000}
            spellCheck={false}
            onChange={(e) => {
              setDraft(e.target.value)
              onDraft?.(q.id, e.target.value)
            }}
            onBlur={() => draft !== (answers[String(q.id)] ?? '') && onAnswer(q.id, draft)}
          />
        )}

        <div className="mt-6 flex justify-between">
          <Button variant="secondary" disabled={index === 0} onClick={() => setIndex((i) => i - 1)}>
            Previous
          </Button>
          <Button variant="secondary" disabled={index === questions.length - 1} onClick={() => setIndex((i) => i + 1)}>
            Next
          </Button>
        </div>
      </section>
    </div>
  )
}
