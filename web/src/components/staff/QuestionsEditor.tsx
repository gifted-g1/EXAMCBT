import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { staffApi, type QuestionInput } from '../../api/staff'
import type { Exam, Question } from '../../api/types'
import { Button, Card, ErrorText, Field, Spinner, errorMessage, inputClass } from '../ui'
import { PdfImport } from './PdfImport'

export function QuestionsEditor({ exam, canManage, onChanged }: { exam: Exam; canManage: boolean; onChanged: () => void }) {
  const [qs, setQs] = useState<Question[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const locked = exam.status === 'ACTIVE' || exam.status === 'CLOSED'
  const editable = canManage && !locked

  const load = useCallback(() => {
    staffApi.questions(exam.id).then(setQs, (e) => setError(errorMessage(e)))
  }, [exam.id])
  useEffect(load, [load])

  const remove = async (qid: number) => {
    try {
      await staffApi.deleteQuestion(exam.id, qid)
      load()
      onChanged()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  return (
    <div className="space-y-6">
      {locked && <p className="rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-600">Questions are locked once the exam opens.</p>}
      <ErrorText>{error}</ErrorText>
      {!qs ? (
        <Spinner />
      ) : (
        <ol className="space-y-3">
          {qs.map((q, i) => (
            <li key={q.id}>
              <Card>
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <p className="text-xs font-medium text-slate-500">
                      Q{i + 1} · {q.kind} · {q.marks} mark{q.marks === 1 ? '' : 's'}
                    </p>
                    <p className="mt-1 whitespace-pre-wrap font-medium">{q.prompt}</p>
                    {q.options && (
                      <ol className="mt-2 space-y-1 text-sm">
                        {q.options.map((o, oi) => (
                          <li key={oi} className={oi === q.correct_option ? 'font-semibold text-emerald-700' : 'text-slate-600'}>
                            {String.fromCharCode(65 + oi)}. {o} {oi === q.correct_option && '✓'}
                          </li>
                        ))}
                      </ol>
                    )}
                  </div>
                  {editable && (
                    <Button variant="ghost" className="text-xs" onClick={() => remove(q.id)}>
                      Delete
                    </Button>
                  )}
                </div>
              </Card>
            </li>
          ))}
        </ol>
      )}
      {editable && (
        <PdfImport
          examId={exam.id}
          onImported={() => {
            load()
            onChanged()
          }}
        />
      )}
      {editable && (
        <AddQuestion
          onAdd={async (q) => {
            await staffApi.addQuestions(exam.id, [q])
            load()
            onChanged()
          }}
        />
      )}
    </div>
  )
}

function AddQuestion({ onAdd }: { onAdd: (q: QuestionInput) => Promise<void> }) {
  const [kind, setKind] = useState<'MCQ' | 'TEXT'>('MCQ')
  const [prompt, setPrompt] = useState('')
  const [options, setOptions] = useState(['', '', '', ''])
  const [correct, setCorrect] = useState(0)
  const [marks, setMarks] = useState(1)
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    const opts = options.map((o) => o.trim())
    try {
      await onAdd(
        kind === 'MCQ'
          ? { kind, prompt, options: opts.filter(Boolean), correct_option: opts.slice(0, correct).filter(Boolean).length, marks }
          : { kind, prompt, marks },
      )
      setPrompt('')
      setOptions(['', '', '', ''])
      setCorrect(0)
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <Card>
      <form onSubmit={submit} className="space-y-4">
        <h3 className="font-semibold">Add a question</h3>
        <div className="flex gap-4 text-sm">
          {(['MCQ', 'TEXT'] as const).map((k) => (
            <label key={k} className="flex items-center gap-1.5">
              <input type="radio" checked={kind === k} onChange={() => setKind(k)} />
              {k === 'MCQ' ? 'Multiple choice' : 'Written answer'}
            </label>
          ))}
        </div>
        <Field label="Question">
          <textarea className={inputClass} rows={3} value={prompt} onChange={(e) => setPrompt(e.target.value)} required />
        </Field>
        {kind === 'MCQ' && (
          <fieldset className="space-y-2">
            <legend className="mb-1 text-sm font-medium text-slate-700">Options (select the correct one)</legend>
            {options.map((o, i) => (
              <div key={i} className="flex items-center gap-2">
                <input type="radio" name="correct" aria-label={`Option ${i + 1} is correct`} checked={correct === i} onChange={() => setCorrect(i)} />
                <input
                  className={inputClass}
                  value={o}
                  placeholder={`Option ${String.fromCharCode(65 + i)}`}
                  onChange={(e) => setOptions((os) => os.map((x, j) => (j === i ? e.target.value : x)))}
                />
              </div>
            ))}
          </fieldset>
        )}
        <Field label="Marks">
          <input type="number" min={0} max={100} step={0.5} className={`${inputClass} w-28`} value={marks} onChange={(e) => setMarks(Number(e.target.value))} />
        </Field>
        <ErrorText>{error}</ErrorText>
        <Button type="submit">Add question</Button>
      </form>
    </Card>
  )
}
