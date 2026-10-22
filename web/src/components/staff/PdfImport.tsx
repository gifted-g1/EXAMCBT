import { useState } from 'react'
import { staffApi, type ParsedQuestion, type QuestionInput } from '../../api/staff'
import { Button, Card, ErrorText, errorMessage } from '../ui'

type Row = ParsedQuestion & { include: boolean }

const letter = (i: number) => String.fromCharCode(65 + i)

/** Upload a question-paper PDF, review what was extracted, fix anything the
 * parser couldn't determine (usually the correct option), then import. */
export function PdfImport({ examId, onImported }: { examId: number; onImported: () => void }) {
  const [rows, setRows] = useState<Row[] | null>(null)
  const [warnings, setWarnings] = useState<string[]>([])
  const [meta, setMeta] = useState<{ file: string; pages: number } | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState<string | null>(null)

  const upload = async (file: File | undefined) => {
    if (!file) return
    setBusy(true)
    setError(null)
    setDone(null)
    try {
      const res = await staffApi.parseQuestionsPDF(examId, file)
      setRows(res.questions.map((q) => ({ ...q, include: true })))
      setWarnings(res.warnings)
      setMeta({ file: file.name, pages: res.pages })
    } catch (e) {
      setRows(null)
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  const downloadSample = async () => {
    const url = await staffApi.samplePDF()
    if (!url) return setError('Could not download the example PDF.')
    const a = document.createElement('a')
    a.href = url
    a.download = 'exam-shield-sample-questions.pdf'
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }

  const update = (i: number, patch: Partial<Row>) => setRows((rs) => rs && rs.map((r, j) => (j === i ? { ...r, ...patch } : r)))

  const selected = rows?.filter((r) => r.include) ?? []
  const missingAnswer = selected.filter((r) => r.kind === 'MCQ' && r.correct_option == null)
  const tooFewOptions = selected.filter((r) => r.kind === 'MCQ' && (r.options?.length ?? 0) < 2)
  const gapBad = selected.filter((r) => r.kind === 'GAP' && (r.blanks?.length ?? 0) !== countBlanks(r.prompt))
  const blocked = selected.length === 0 || missingAnswer.length > 0 || tooFewOptions.length > 0 || gapBad.length > 0

  const importAll = async () => {
    setBusy(true)
    setError(null)
    try {
      const payload: QuestionInput[] = selected.map((r) =>
        r.kind === 'MCQ'
          ? { kind: 'MCQ', prompt: r.prompt, options: r.options, correct_option: r.correct_option ?? 0, marks: r.marks }
          : r.kind === 'GAP'
            ? { kind: 'GAP', prompt: r.prompt, blanks: r.blanks ?? [], marks: r.marks }
            : { kind: 'TEXT', prompt: r.prompt, marks: r.marks },
      )
      await staffApi.addQuestions(examId, payload)
      setDone(`Imported ${payload.length} question(s) from ${meta?.file}.`)
      setRows(null)
      onImported()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="font-semibold">Import questions from a PDF</h3>
          <p className="mt-1 max-w-2xl text-sm text-slate-500">
            Number questions <code>1.</code> <code>2.</code>, letter options <code>A.</code> <code>B.</code>, and give answers as{' '}
            <code>Answer: B</code> (or in an <em>Answer Key</em> section). Questions with blanks (<code>_____</code>) become fill-in-the-gap, with <code>Answer: (i) ... (ii) ...</code>. Other questions without options become written answers. Text PDFs
            only — scanned images can&apos;t be read.
          </p>
        </div>
        <Button variant="secondary" onClick={downloadSample}>
          Download example PDF
        </Button>
      </div>

      <label className="block">
        <span className="sr-only">Question paper PDF</span>
        <input
          type="file"
          accept="application/pdf,.pdf"
          disabled={busy}
          className="text-sm"
          onChange={(e) => {
            void upload(e.target.files?.[0])
            e.target.value = ''
          }}
        />
      </label>
      {busy && !rows && <p className="text-sm text-slate-500">Reading PDF…</p>}
      <ErrorText>{error}</ErrorText>
      {done && <p className="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-800">{done}</p>}

      {rows && (
        <div className="space-y-3">
          <p className="text-sm text-slate-600">
            Found <b>{rows.length}</b> question(s) in {meta?.file} ({meta?.pages} page{meta?.pages === 1 ? '' : 's'}). Review them, then
            import.
          </p>
          {warnings.map((w) => (
            <p key={w} className="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
              {w}
            </p>
          ))}
          <ol className="space-y-3">
            {rows.map((r, i) => (
              <li key={i} className={`rounded-lg p-3 ring-1 ${r.include ? 'ring-slate-200' : 'opacity-50 ring-slate-100'}`}>
                <div className="flex items-start gap-3">
                  <input
                    type="checkbox"
                    className="mt-1"
                    aria-label={`Include question ${r.number}`}
                    checked={r.include}
                    onChange={(e) => update(i, { include: e.target.checked })}
                  />
                  <div className="min-w-0 flex-1 space-y-2">
                    <p className="text-xs font-medium text-slate-500">
                      Q{r.number} · {r.kind === 'MCQ' ? 'Multiple choice' : r.kind === 'GAP' ? 'Fill in the gap' : 'Written answer'} ·{' '}
                      <label>
                        marks{' '}
                        <input
                          type="number"
                          min={0}
                          max={100}
                          step={0.5}
                          value={r.marks}
                          onChange={(e) => update(i, { marks: Number(e.target.value) })}
                          className="w-16 rounded border-0 px-1 py-0.5 text-xs ring-1 ring-slate-300"
                        />
                      </label>
                    </p>
                    <p className="whitespace-pre-wrap font-medium">{r.prompt}</p>
                    {r.kind === 'GAP' && (
                      <div className="space-y-1 text-sm">
                        {Array.from({ length: countBlanks(r.prompt) }, (_, bi) => (
                          <label key={bi} className="flex items-center gap-2">
                            <span className="w-16 shrink-0 text-slate-500">Blank {bi + 1}</span>
                            <input
                              aria-label={`Question ${r.number} blank ${bi + 1} answer`}
                              className="flex-1 rounded border-0 px-2 py-0.5 ring-1 ring-slate-300"
                              value={(r.blanks?.[bi] ?? []).join('/')}
                              onChange={(e) => {
                                const b = Array.from({ length: countBlanks(r.prompt) }, (_, j) => r.blanks?.[j] ?? [])
                                b[bi] = e.target.value === '' ? [] : e.target.value.split('/') // trimmed on the server
                                update(i, { blanks: b })
                              }}
                            />
                          </label>
                        ))}
                      </div>
                    )}
                    {r.kind === 'MCQ' && (
                      <fieldset className="space-y-1 text-sm">
                        <legend className="sr-only">Correct option for question {r.number}</legend>
                        {(r.options ?? []).map((o, oi) => (
                          <label key={oi} className="flex items-start gap-2">
                            <input
                              type="radio"
                              name={`pdf-q-${i}`}
                              checked={r.correct_option === oi}
                              onChange={() => update(i, { correct_option: oi })}
                              className="mt-0.5"
                            />
                            <span className={r.correct_option === oi ? 'font-semibold text-emerald-700' : 'text-slate-700'}>
                              {letter(oi)}. {o}
                            </span>
                          </label>
                        ))}
                      </fieldset>
                    )}
                    {r.warnings
                      ?.filter((w) => !(w.startsWith('no answer') && r.correct_option != null))
                      .map((w) => (
                        <p key={w} className="text-xs text-amber-700">
                          ⚠ {w}
                        </p>
                      ))}
                  </div>
                </div>
              </li>
            ))}
          </ol>
          <div className="flex flex-wrap items-center gap-3">
            <Button onClick={importAll} disabled={busy || blocked}>
              Import {selected.length} question{selected.length === 1 ? '' : 's'}
            </Button>
            <Button variant="ghost" onClick={() => setRows(null)} disabled={busy}>
              Cancel
            </Button>
            {missingAnswer.length > 0 && (
              <span className="text-sm text-amber-700">
                Choose the correct option for Q{missingAnswer.map((r) => r.number).join(', Q')} (or untick them).
              </span>
            )}
            {gapBad.length > 0 && (
              <span className="text-sm text-amber-700">Give an answer for every blank in Q{gapBad.map((r) => r.number).join(', Q')}.</span>
            )}
            {tooFewOptions.length > 0 && (
              <span className="text-sm text-amber-700">Q{tooFewOptions.map((r) => r.number).join(', Q')} need at least two options.</span>
            )}
          </div>
        </div>
      )}
    </Card>
  )
}

const countBlanks = (s: string) => (s.match(/_{3,}/g) ?? []).length
