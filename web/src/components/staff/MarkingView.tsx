import { Fragment, useCallback, useEffect, useState, type ReactNode } from 'react'
import { staffApi } from '../../api/staff'
import type { Exam, MarkingOverview, MarkingPoint, MarkingQuestion, MarkingScript, QuestionScripts } from '../../api/types'
import { Button, Card, ErrorText, Field, Spinner, errorMessage, inputClass } from '../ui'

/**
 * Theory marking. The examiner writes a marking guide; each answer gets a
 * keyword-based suggested mark (and, when the server allows it, an AI
 * suggestion). Nothing counts until the examiner saves a mark.
 */
export function MarkingView({ exam, canManage }: { exam: Exam; canManage: boolean }) {
  const [ov, setOv] = useState<MarkingOverview | null>(null)
  const [qid, setQid] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)

  const loadOverview = useCallback(() => {
    staffApi.marking(exam.id).then(
      (r) => {
        setOv(r)
        setQid((cur) => cur ?? r.questions[0]?.question_id ?? null)
      },
      (e) => setError(errorMessage(e)),
    )
  }, [exam.id])
  useEffect(loadOverview, [loadOverview])

  if (error) return <ErrorText>{error}</ErrorText>
  if (!ov) return <Spinner />
  if (ov.questions.length === 0)
    return <p className="text-sm text-slate-500">This exam has no written (theory) questions. Multiple-choice questions are marked automatically.</p>

  return (
    <div className="grid gap-6 lg:grid-cols-[16rem_1fr]">
      <nav aria-label="Written questions" className="space-y-2">
        {ov.questions.map((q) => (
          <button
            key={q.question_id}
            type="button"
            aria-current={qid === q.question_id}
            onClick={() => setQid(q.question_id)}
            className={`w-full rounded-xl p-3 text-left ring-1 transition ${
              qid === q.question_id ? 'bg-brand-50 ring-brand-500' : 'bg-white ring-slate-200 hover:bg-slate-50'
            }`}
          >
            <p className="truncate text-sm font-medium text-slate-900">
              Q{q.number}. {q.prompt}
            </p>
            <p className="mt-1 text-xs text-slate-500">
              {q.marked} / {q.answered} marked · {q.marks} marks
              {q.marking_guide.length === 0 && <span className="ml-1 text-amber-700">· no guide</span>}
            </p>
            <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-slate-100" aria-hidden>
              <div className="h-full bg-emerald-500" style={{ width: `${q.answered ? (q.marked / q.answered) * 100 : 0}%` }} />
            </div>
          </button>
        ))}
      </nav>
      {qid != null && (
        <QuestionMarking key={qid} examId={exam.id} qid={qid} aiEnabled={ov.ai_enabled} canManage={canManage} onProgress={loadOverview} />
      )}
    </div>
  )
}

function QuestionMarking({
  examId,
  qid,
  aiEnabled,
  canManage,
  onProgress,
}: {
  examId: number
  qid: number
  aiEnabled: boolean
  canManage: boolean
  onProgress: () => void
}) {
  const [data, setData] = useState<QuestionScripts | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [aiBusy, setAiBusy] = useState(false)
  const [aiNote, setAiNote] = useState<string | null>(null)
  const [onlyUnmarked, setOnlyUnmarked] = useState(false)

  const load = useCallback(() => {
    staffApi.markingScripts(examId, qid).then(setData, (e) => setError(errorMessage(e)))
  }, [examId, qid])
  useEffect(load, [load])

  const askAI = async () => {
    setAiBusy(true)
    setAiNote(null)
    try {
      const r = await staffApi.aiSuggest(examId, qid)
      setAiNote(
        r.requested === 0
          ? 'Every answer already has a mark.'
          : `AI suggested marks for ${r.suggested} of ${r.requested} answers.${r.failed ? ` ${r.failed} failed: ${r.errors?.[0] ?? ''}` : ''}`,
      )
      load()
    } catch (e) {
      setAiNote(errorMessage(e))
    } finally {
      setAiBusy(false)
    }
  }

  if (error) return <ErrorText>{error}</ErrorText>
  if (!data) return <Spinner />
  const q = data.question
  const shown = onlyUnmarked ? data.scripts.filter((s) => s.awarded == null) : data.scripts

  return (
    <div className="space-y-5">
      <Card>
        <p className="text-sm text-slate-500">
          Question {q.number} · {q.marks} marks
        </p>
        <p className="mt-1 whitespace-pre-wrap font-medium text-slate-900">{q.prompt}</p>
        <GuideEditor examId={examId} question={q} canManage={canManage} onSaved={() => (load(), onProgress())} />
      </Card>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-slate-500">
          {data.scripts.length} answer{data.scripts.length === 1 ? '' : 's'} · names are hidden while you mark.
        </p>
        <div className="flex flex-wrap items-center gap-3">
          <label className="flex items-center gap-2 text-sm text-slate-600">
            <input type="checkbox" checked={onlyUnmarked} onChange={(e) => setOnlyUnmarked(e.target.checked)} />
            Unmarked only
          </label>
          {canManage && aiEnabled && (
            <Button variant="secondary" onClick={askAI} disabled={aiBusy || (q.marking_guide.length === 0 && !q.model_answer)}>
              {aiBusy ? 'Asking AI…' : 'Get AI suggestions'}
            </Button>
          )}
        </div>
      </div>
      {aiEnabled ? (
        canManage && (
          <p className="text-xs text-slate-500">
            AI suggestions send the question, your guide and the answer text (no names or matric numbers) to the Anthropic API. They are
            suggestions only — you decide every mark.
          </p>
        )
      ) : (
        <p className="text-xs text-slate-500">AI-suggested marks are off on this server (an administrator can enable them).</p>
      )}
      {aiNote && <p className="rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-700">{aiNote}</p>}

      <ol className="space-y-4">
        {shown.map((s) => (
          <ScriptCard
            key={s.answer_id}
            examId={examId}
            max={q.marks}
            guide={q.marking_guide}
            script={s}
            canManage={canManage}
            onSaved={(next) => {
              setData((d) => d && { ...d, scripts: d.scripts.map((x) => (x.answer_id === next.answer_id ? next : x)) })
              onProgress()
            }}
          />
        ))}
        {shown.length === 0 && <p className="text-sm text-slate-500">Nothing to mark here.</p>}
      </ol>
    </div>
  )
}

function GuideEditor({ examId, question, canManage, onSaved }: { examId: number; question: MarkingQuestion; canManage: boolean; onSaved: () => void }) {
  const [open, setOpen] = useState(question.marking_guide.length === 0 && canManage)
  const [points, setPoints] = useState<MarkingPoint[]>(question.marking_guide)
  const [kwText, setKwText] = useState<string[]>(question.marking_guide.map((p) => p.keywords.join(', ')))
  const [modelAnswer, setModelAnswer] = useState(question.model_answer)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const total = points.reduce((t, p) => t + (Number(p.marks) || 0), 0)

  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      const pts = points.map((p, i) => ({
        point: p.point,
        marks: Number(p.marks),
        keywords: kwText[i].split(',').map((k) => k.trim()).filter(Boolean),
      }))
      await staffApi.setMarkingGuide(examId, question.question_id, pts, modelAnswer)
      setOpen(false)
      onSaved()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  if (!open)
    return (
      <div className="mt-4 border-t border-slate-100 pt-4">
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm font-semibold text-slate-700">Marking guide</p>
          {canManage && (
            <Button variant="ghost" className="px-2 py-1 text-xs" onClick={() => setOpen(true)}>
              Edit guide
            </Button>
          )}
        </div>
        {question.marking_guide.length === 0 ? (
          <p className="mt-1 text-sm text-slate-500">No guide yet, so there are no suggested marks.</p>
        ) : (
          <ul className="mt-2 space-y-1 text-sm">
            {question.marking_guide.map((p, i) => (
              <li key={i} className="flex gap-2">
                <span className="w-14 shrink-0 tabular-nums text-slate-500">{p.marks} mk</span>
                <span>
                  {p.point}
                  {p.keywords.length > 0 && <span className="text-slate-400"> — {p.keywords.join(', ')}</span>}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    )

  return (
    <div className="mt-4 space-y-3 border-t border-slate-100 pt-4">
      <p className="text-sm font-semibold text-slate-700">Marking guide</p>
      <p className="text-xs text-slate-500">
        List the key points you expect, the marks for each, and words that show the point was made (comma separated). An answer gets a
        point's marks when any of its keywords appears.
      </p>
      {points.map((p, i) => (
        <div key={i} className="grid gap-2 rounded-lg bg-slate-50 p-3 sm:grid-cols-[1fr_5rem_1fr_auto]">
          <input
            aria-label={`Point ${i + 1}`}
            className={inputClass}
            placeholder="Key point, e.g. Defines encryption"
            value={p.point}
            onChange={(e) => setPoints(points.map((x, j) => (j === i ? { ...x, point: e.target.value } : x)))}
          />
          <input
            aria-label={`Point ${i + 1} marks`}
            className={inputClass}
            type="number"
            min={0.5}
            step={0.5}
            value={p.marks}
            onChange={(e) => setPoints(points.map((x, j) => (j === i ? { ...x, marks: Number(e.target.value) } : x)))}
          />
          <input
            aria-label={`Point ${i + 1} keywords`}
            className={inputClass}
            placeholder="Keywords: ciphertext, encrypt"
            value={kwText[i]}
            onChange={(e) => setKwText(kwText.map((x, j) => (j === i ? e.target.value : x)))}
          />
          <Button
            variant="ghost"
            aria-label={`Remove point ${i + 1}`}
            onClick={() => (setPoints(points.filter((_, j) => j !== i)), setKwText(kwText.filter((_, j) => j !== i)))}
          >
            ✕
          </Button>
        </div>
      ))}
      <div className="flex flex-wrap items-center gap-3">
        <Button
          variant="secondary"
          onClick={() => (setPoints([...points, { point: '', marks: 1, keywords: [] }]), setKwText([...kwText, '']))}
          disabled={points.length >= 20}
        >
          Add point
        </Button>
        <span className={`text-sm tabular-nums ${total > question.marks ? 'text-rose-600' : 'text-slate-500'}`}>
          Points total {total} / {question.marks}
        </span>
      </div>
      <Field label="Model answer (optional)" hint="Used by the AI when AI suggestions are on, and shown to markers.">
        <textarea className={`${inputClass} min-h-24`} value={modelAnswer} onChange={(e) => setModelAnswer(e.target.value)} />
      </Field>
      <ErrorText>{error}</ErrorText>
      <div className="flex gap-2">
        <Button onClick={save} disabled={busy || total > question.marks}>
          {busy ? 'Saving…' : 'Save guide'}
        </Button>
        <Button variant="ghost" onClick={() => setOpen(false)}>
          Cancel
        </Button>
      </div>
    </div>
  )
}

/** Wraps keyword matches in <mark> so the examiner sees them at a glance. */
export function highlight(text: string, words: string[]): ReactNode {
  const ws = words.filter(Boolean)
  if (ws.length === 0) return text
  const esc = ws.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).sort((a, b) => b.length - a.length)
  // Match the keyword plus any word ending, as the server does (encrypt → encrypted).
  const re = new RegExp(`\\b(${esc.join('|')})[\\p{L}\\p{N}]*`, 'giu')
  const out: ReactNode[] = []
  let last = 0
  for (const m of text.matchAll(re)) {
    const i = m.index ?? 0
    out.push(text.slice(last, i), <mark key={i} className="rounded bg-yellow-200 px-0.5 text-slate-900">{m[0]}</mark>)
    last = i + m[0].length
  }
  out.push(text.slice(last))
  return out.map((n, i) => <Fragment key={i}>{n}</Fragment>)
}

function ScriptCard({
  examId,
  max,
  guide,
  script,
  canManage,
  onSaved,
}: {
  examId: number
  max: number
  guide: MarkingPoint[]
  script: MarkingScript
  canManage: boolean
  onSaved: (s: MarkingScript) => void
}) {
  const kw = script.keyword_suggestion
  const ai = script.ai_suggestion
  const [marks, setMarks] = useState<string>(script.awarded != null ? String(script.awarded) : '')
  const [comment, setComment] = useState(script.comment)
  const [source, setSource] = useState<'manual' | 'keyword' | 'ai'>('manual')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const dirty = marks !== (script.awarded != null ? String(script.awarded) : '') || comment !== script.comment

  const save = async (value: string = marks) => {
    setBusy(true)
    setError(null)
    try {
      const n = value.trim() === '' ? null : Number(value)
      if (n != null && (Number.isNaN(n) || n < 0 || n > max)) throw new Error(`Enter a mark between 0 and ${max}.`)
      const r = await staffApi.markAnswer(examId, script.answer_id, n, comment, source)
      onSaved({ ...script, awarded: r.awarded, comment, marked_at: new Date().toISOString() })
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  const use = (v: number, src: 'keyword' | 'ai') => {
    setMarks(String(v))
    setSource(src)
  }

  return (
    <li className={`rounded-xl bg-white p-4 shadow-sm ring-1 ${script.awarded != null ? 'ring-emerald-200' : 'ring-slate-200'}`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="font-semibold text-slate-900">{script.candidate}</p>
        {script.awarded != null ? (
          <span className="rounded-full bg-emerald-100 px-2.5 py-0.5 text-xs font-semibold text-emerald-800">
            Marked {script.awarded} / {max}
          </span>
        ) : (
          <span className="rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-semibold text-amber-800">Needs marking</span>
        )}
      </div>
      <div className="mt-3 whitespace-pre-wrap rounded-lg bg-slate-50 p-3 text-sm leading-relaxed text-slate-800">
        {highlight(script.response, kw?.highlights ?? [])}
      </div>

      <div className="mt-3 grid gap-3 md:grid-cols-2">
        {kw && (
          <div className="rounded-lg p-3 ring-1 ring-slate-200">
            <div className="flex items-center justify-between gap-2">
              <p className="text-sm font-semibold text-slate-700">
                Keyword suggestion: {kw.marks} / {kw.max}
              </p>
              {canManage && (
                <Button variant="ghost" className="px-2 py-1 text-xs" onClick={() => use(kw.marks, 'keyword')}>
                  Use
                </Button>
              )}
            </div>
            <ul className="mt-2 space-y-1 text-xs">
              {kw.points.map((p) => (
                <li key={p.index} className={p.met ? 'text-emerald-800' : 'text-slate-500'}>
                  <span aria-hidden>{p.met ? '✓' : '✗'}</span> {p.point} ({p.marks})
                  <span className="sr-only">{p.met ? ' — found' : ' — not found'}</span>
                  {p.met && <span className="text-slate-400"> — {p.matched.join(', ')}</span>}
                </li>
              ))}
            </ul>
            <p className="mt-2 text-xs text-slate-400">Keywords can miss correct answers in other words — read the answer.</p>
          </div>
        )}
        {ai && (
          <div className="rounded-lg bg-violet-50 p-3 ring-1 ring-violet-200">
            <div className="flex items-center justify-between gap-2">
              <p className="text-sm font-semibold text-violet-900">
                AI suggestion: {ai.marks} / {max}
              </p>
              {canManage && (
                <Button variant="ghost" className="px-2 py-1 text-xs" onClick={() => use(ai.marks, 'ai')}>
                  Use
                </Button>
              )}
            </div>
            <p className="mt-1 text-sm text-violet-900">{ai.reason}</p>
            {ai.points_met && ai.points_met.length > 0 && (
              <p className="mt-1 text-xs text-violet-700">
                Points made: {ai.points_met.map((i) => guide[i]?.point ?? `#${i + 1}`).join('; ')}
              </p>
            )}
          </div>
        )}
      </div>

      {canManage ? (
        <div className="mt-3 flex flex-wrap items-end gap-3">
          <label className="text-sm">
            <span className="mb-1 block font-medium text-slate-700">Mark (out of {max})</span>
            <input
              aria-label={`Mark for ${script.candidate}`}
              className={`${inputClass} w-24`}
              type="number"
              min={0}
              max={max}
              step={0.5}
              value={marks}
              onChange={(e) => (setMarks(e.target.value), setSource('manual'))}
            />
          </label>
          <label className="min-w-48 flex-1 text-sm">
            <span className="mb-1 block font-medium text-slate-700">Comment (optional)</span>
            <input
              aria-label={`Comment for ${script.candidate}`}
              className={inputClass}
              value={comment}
              onChange={(e) => setComment(e.target.value)}
            />
          </label>
          <Button onClick={() => save()} disabled={busy || !dirty || marks.trim() === ''}>
            {busy ? 'Saving…' : 'Save mark'}
          </Button>
          {script.awarded != null && (
            <Button variant="ghost" onClick={() => (setMarks(''), save(''))} disabled={busy}>
              Clear
            </Button>
          )}
        </div>
      ) : (
        script.comment && <p className="mt-2 text-sm text-slate-600">Examiner: {script.comment}</p>
      )}
      <ErrorText>{error}</ErrorText>
    </li>
  )
}
