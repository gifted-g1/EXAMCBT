import { useEffect, useState } from 'react'
import { useNavigate, useParams, Link, useLocation } from 'react-router-dom'
import { examApi } from '../../services/api'
import Card from '../../components/ui/Card'
import Button from '../../components/ui/Button'
import Table from '../../components/ui/Table'
import Modal from '../../components/ui/Modal'
import Alert from '../../components/ui/Alert'
import { StatusBadge } from '../../components/ui/Badge'
import { Field, Input, Select } from '../../components/ui/Form'
import ExamServerPanel from './ExamServerPanel'

const NEXT_ACTION = {
  DRAFT: { label: 'Schedule', fn: 'schedule' },
  SCHEDULED: { label: 'Publish', fn: 'publish' },
  PUBLISHED: { label: 'Start examination', fn: 'start' },
  ACTIVE: { label: 'End examination', fn: 'end' },
}

export default function ExamDetail() {
  const { examId } = useParams()
  const navigate = useNavigate()
  const basePath = useLocation().pathname.startsWith('/lecturer') ? '/lecturer' : '/admin'
  const [exam, setExam] = useState(null)
  const [questions, setQuestions] = useState([])
  const [lanInfo, setLanInfo] = useState(null)
  const [addOpen, setAddOpen] = useState(false)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [uploadResult, setUploadResult] = useState(null)
  const [busy, setBusy] = useState(false)
  const [qForm, setQForm] = useState(emptyQuestion())

  const load = () => {
    examApi.get(examId).then((e) => {
      setExam(e)
      setQuestions(e.questions || [])
    })
  }

  useEffect(load, [examId])

  const runTransition = async (fn) => {
    setBusy(true)
    try {
      const result = await examApi[fn](examId)
      if (fn === 'start') {
        setExam(result.exam)
        setLanInfo(result.lan_server)
      } else {
        setExam(result)
      }
    } finally {
      setBusy(false)
    }
  }

  const addQuestion = async (e) => {
    e.preventDefault()
    const payload = { ...qForm }
    if (payload.type !== 'MULTIPLE_CHOICE') payload.options = []
    const created = await examApi.addQuestion(examId, payload)
    setQuestions((q) => [...q, created])
    setAddOpen(false)
    setQForm(emptyQuestion())
  }

  const deleteQuestion = async (question) => {
    if (!window.confirm(`Remove "${question.text.slice(0, 60)}${question.text.length > 60 ? '…' : ''}"?`)) return
    await examApi.deleteQuestion(examId, question.id)
    setQuestions((qs) => qs.filter((q) => q.id !== question.id))
  }

  const handlePDFUpload = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    setUploadError('')
    setUploadResult(null)
    try {
      const created = await examApi.uploadQuestionsPDF(examId, file)
      setQuestions((q) => [...q, ...created])
      setUploadResult(created)
    } catch (err) {
      setUploadError(
        err.response?.data?.error || 'Could not import questions from this PDF. Check the expected format below and try again.'
      )
    } finally {
      setUploading(false)
      e.target.value = ''
    }
  }

  if (!exam) return <p>Loading…</p>

  const action = NEXT_ACTION[exam.status]

  const columns = [
    { key: 'order_index', header: '#' },
    { key: 'text', header: 'Question' },
    { key: 'type', header: 'Type' },
    { key: 'points', header: 'Points' },
    {
      key: 'actions',
      header: '',
      render: (row) => (
        <Button size="sm" variant="ghost" onClick={() => deleteQuestion(row)}>Remove</Button>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
        <div>
          <Link to={`${basePath}/exams`} style={{ fontSize: 13 }}>← All examinations</Link>
          <h1 style={{ marginTop: 6 }}>{exam.title}</h1>
          <StatusBadge status={{ ACTIVE: 'NORMAL', SCHEDULED: 'ATTENTION', PUBLISHED: 'ATTENTION', ENDED: 'DISCONNECTED', DRAFT: 'DISCONNECTED' }[exam.status]} />
        </div>
        <div style={{ display: 'flex', gap: 10 }}>
          {exam.status === 'ACTIVE' && (
            <Button variant="secondary" onClick={() => navigate(`${basePath}/monitoring/${examId}`)}>
              Open monitoring dashboard
            </Button>
          )}
          {action && (
            <Button onClick={() => runTransition(action.fn)} disabled={busy}>
              {busy ? 'Working…' : action.label}
            </Button>
          )}
        </div>
      </div>

      {lanInfo && (
        <div style={{ marginBottom: 20 }}>
          <ExamServerPanel info={lanInfo} />
        </div>
      )}

      {exam.mode === 'LAN' && exam.lan_server_ip && !lanInfo && (
        <div style={{ marginBottom: 20 }}>
          <Alert tone="info">
            LAN exam server previously started at <strong>http://{exam.lan_server_ip}:{exam.lan_server_port}</strong>.
          </Alert>
        </div>
      )}

      <Card
        title="Questions"
        action={
          <div style={{ display: 'flex', gap: 8 }}>
            <Button size="sm" variant="secondary" onClick={() => { setUploadOpen(true); setUploadError(''); setUploadResult(null) }}>
              Upload PDF
            </Button>
            <Button size="sm" onClick={() => setAddOpen(true)}>Add question</Button>
          </div>
        }
      >
        <Table columns={columns} rows={questions} emptyMessage="No questions added yet." />
      </Card>

      <Modal open={uploadOpen} onClose={() => setUploadOpen(false)} title="Upload questions from a PDF" width={560}>
        <p style={{ fontSize: 13.5, color: 'var(--color-text-muted)', marginTop: 0 }}>
          Instead of typing each question, upload a PDF laid out like this — Exam Shield will
          extract multiple-choice, true/false, and short-answer questions automatically:
        </p>
        <pre style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-sm)',
          padding: '12px 14px',
          fontSize: 12.5,
          lineHeight: 1.6,
          overflowX: 'auto',
        }}>
{`1. What is an API?
A) Application Programming Interface
B) Automated Process Interface
C) Advanced Programming Instruction
D) Application Process Integration
Answer: A

2. The sky is blue during the day.
Answer: True

3. Explain the concept of polymorphism in OOP.`}
        </pre>

        {uploadError && <Alert tone="danger">{uploadError}</Alert>}
        {uploadResult && (
          <Alert tone="success">
            Imported {uploadResult.length} question{uploadResult.length === 1 ? '' : 's'}. Review them in the list — you can remove any that weren't extracted correctly.
          </Alert>
        )}

        <div style={{ marginTop: 14 }}>
          <input type="file" accept="application/pdf" onChange={handlePDFUpload} disabled={uploading} />
        </div>
        {uploading && <p style={{ fontSize: 13, color: 'var(--color-text-muted)' }}>Reading and parsing your PDF…</p>}
      </Modal>

      <Modal open={addOpen} onClose={() => setAddOpen(false)} title="Add a question" width={560}>
        <form onSubmit={addQuestion}>
          <Field label="Question type">
            <Select value={qForm.type} onChange={(e) => setQForm({ ...qForm, type: e.target.value })}>
              <option value="MULTIPLE_CHOICE">Multiple choice</option>
              <option value="TRUE_FALSE">True / False</option>
              <option value="SHORT_ANSWER">Short answer</option>
            </Select>
          </Field>
          <Field label="Question text">
            <Input required value={qForm.text} onChange={(e) => setQForm({ ...qForm, text: e.target.value })} />
          </Field>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <Field label="Points">
              <Input type="number" min={0} value={qForm.points} onChange={(e) => setQForm({ ...qForm, points: Number(e.target.value) })} />
            </Field>
            <Field label="Order">
              <Input type="number" min={0} value={qForm.order_index} onChange={(e) => setQForm({ ...qForm, order_index: Number(e.target.value) })} />
            </Field>
          </div>

          {qForm.type === 'TRUE_FALSE' && (
            <Field label="Correct answer">
              <Select value={qForm.correct_text} onChange={(e) => setQForm({ ...qForm, correct_text: e.target.value })}>
                <option value="true">True</option>
                <option value="false">False</option>
              </Select>
            </Field>
          )}

          {qForm.type === 'MULTIPLE_CHOICE' && (
            <div>
              <p style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Options</p>
              {qForm.options.map((opt, i) => (
                <div key={i} style={{ display: 'flex', gap: 8, marginBottom: 8, alignItems: 'center' }}>
                  <Input
                    placeholder={`Option ${i + 1}`}
                    value={opt.text}
                    onChange={(e) => {
                      const options = [...qForm.options]
                      options[i] = { ...options[i], text: e.target.value }
                      setQForm({ ...qForm, options })
                    }}
                  />
                  <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, whiteSpace: 'nowrap' }}>
                    <input
                      type="radio"
                      name="correct-option"
                      checked={opt.is_correct}
                      onChange={() => {
                        const options = qForm.options.map((o, j) => ({ ...o, is_correct: j === i }))
                        setQForm({ ...qForm, options })
                      }}
                    />
                    Correct
                  </label>
                </div>
              ))}
              <Button type="button" size="sm" variant="ghost" onClick={() => setQForm({ ...qForm, options: [...qForm.options, { text: '', is_correct: false }] })}>
                + Add option
              </Button>
            </div>
          )}

          <div style={{ marginTop: 18 }}>
            <Button type="submit" fullWidth>Add question</Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}

function emptyQuestion() {
  return {
    type: 'MULTIPLE_CHOICE',
    text: '',
    points: 1,
    order_index: 0,
    correct_text: 'true',
    options: [{ text: '', is_correct: true }, { text: '', is_correct: false }],
  }
}
