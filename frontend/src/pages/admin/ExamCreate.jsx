import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { examApi } from '../../services/api'
import Card from '../../components/ui/Card'
import Button from '../../components/ui/Button'
import { Field, Input, Select, Textarea, Checkbox } from '../../components/ui/Form'

const initial = {
  title: '',
  course: '',
  course_code: '',
  duration_minutes: 60,
  mode: 'ONLINE',
  randomize_questions: false,
  randomize_answers: false,
  allow_navigation: true,
  enable_ai_monitoring: true,
  require_face_verification: true,
  passing_score: 50,
  instructions: '',
}

export default function ExamCreate() {
  const [form, setForm] = useState(initial)
  const [submitting, setSubmitting] = useState(false)
  const navigate = useNavigate()
  const basePath = useLocation().pathname.startsWith('/lecturer') ? '/lecturer' : '/admin'

  const update = (patch) => setForm((f) => ({ ...f, ...patch }))

  const handleSubmit = async (e) => {
    e.preventDefault()
    setSubmitting(true)
    try {
      const exam = await examApi.create(form)
      navigate(`${basePath}/exams/${exam.id}`)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div style={{ maxWidth: 720 }}>
      <h1>Create examination</h1>
      <form onSubmit={handleSubmit}>
        <Card title="Details">
          <Field label="Examination title">
            <Input required value={form.title} onChange={(e) => update({ title: e.target.value })} />
          </Field>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <Field label="Course">
              <Input value={form.course} onChange={(e) => update({ course: e.target.value })} />
            </Field>
            <Field label="Course code">
              <Input value={form.course_code} onChange={(e) => update({ course_code: e.target.value })} />
            </Field>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <Field label="Duration (minutes)">
              <Input type="number" min={1} required value={form.duration_minutes} onChange={(e) => update({ duration_minutes: Number(e.target.value) })} />
            </Field>
            <Field label="Passing score">
              <Input type="number" min={0} value={form.passing_score} onChange={(e) => update({ passing_score: Number(e.target.value) })} />
            </Field>
          </div>
          <Field label="Instructions">
            <Textarea value={form.instructions} onChange={(e) => update({ instructions: e.target.value })} placeholder="Shown to students before they begin." />
          </Field>
        </Card>

        <div style={{ height: 16 }} />

        <Card title="Mode & policy">
          <Field label="Examination mode" hint="LAN mode stands up a local exam server students join over Wi-Fi.">
            <Select value={form.mode} onChange={(e) => update({ mode: e.target.value })}>
              <option value="ONLINE">Online</option>
              <option value="LAN">Local network (LAN)</option>
            </Select>
          </Field>
          <Checkbox label="Randomize question order" checked={form.randomize_questions} onChange={(e) => update({ randomize_questions: e.target.checked })} />
          <Checkbox label="Randomize answer order" checked={form.randomize_answers} onChange={(e) => update({ randomize_answers: e.target.checked })} />
          <Checkbox label="Allow free navigation between questions" checked={form.allow_navigation} onChange={(e) => update({ allow_navigation: e.target.checked })} />
          <Checkbox label="Enable AI monitoring" checked={form.enable_ai_monitoring} onChange={(e) => update({ enable_ai_monitoring: e.target.checked })} />
          <Checkbox label="Require face verification to enter" checked={form.require_face_verification} onChange={(e) => update({ require_face_verification: e.target.checked })} />
        </Card>

        <div style={{ marginTop: 20 }}>
          <Button type="submit" disabled={submitting}>{submitting ? 'Creating…' : 'Create examination'}</Button>
        </div>
      </form>
    </div>
  )
}
