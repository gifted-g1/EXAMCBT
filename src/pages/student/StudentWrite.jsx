import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { examApi, monitoringApi } from '../../services/api'
import Button from '../../components/ui/Button'
import Alert from '../../components/ui/Alert'
import './StudentWrite.css'

const FRAME_ANALYSIS_INTERVAL_MS = 8000

export default function StudentWrite() {
  const { examId } = useParams()
  const navigate = useNavigate()

  const [exam, setExam] = useState(null)
  const [attempt, setAttempt] = useState(null)
  const [questions, setQuestions] = useState([])
  const [answers, setAnswers] = useState({}) // questionId -> answer value
  const [currentIndex, setCurrentIndex] = useState(0)
  const [secondsLeft, setSecondsLeft] = useState(0)
  const [cameraActive, setCameraActive] = useState(false)
  const [connectionOk, setConnectionOk] = useState(true)
  const [warning, setWarning] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const videoRef = useRef(null)
  const canvasRef = useRef(null)

  useEffect(() => {
    async function init() {
      const examData = await examApi.get(examId)
      const attemptData = await examApi.startAttempt(examId)
      setExam(examData)
      setQuestions(examData.questions || [])
      setAttempt(attemptData)
      setSecondsLeft(examData.duration_minutes * 60)
    }
    init()
  }, [examId])

  // Camera + periodic AI monitoring frame capture.
  useEffect(() => {
    if (!exam?.enable_ai_monitoring || !attempt) return
    let stream
    let intervalId

    navigator.mediaDevices
      ?.getUserMedia({ video: { width: 320, height: 240 } })
      .then((s) => {
        stream = s
        if (videoRef.current) videoRef.current.srcObject = s
        setCameraActive(true)

        intervalId = setInterval(() => {
          captureFrame()
        }, FRAME_ANALYSIS_INTERVAL_MS)
      })
      .catch(() => {
        setCameraActive(false)
        setWarning('Camera is disabled or unavailable. AI monitoring cannot run without it.')
      })

    return () => {
      stream?.getTracks().forEach((t) => t.stop())
      if (intervalId) clearInterval(intervalId)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exam, attempt])

  const captureFrame = async () => {
    if (!videoRef.current || !canvasRef.current || !attempt) return
    const video = videoRef.current
    const canvas = canvasRef.current
    if (!video.videoWidth) return
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    canvas.getContext('2d').drawImage(video, 0, 0)
    const imageBase64 = canvas.toDataURL('image/jpeg', 0.6)

    try {
      await monitoringApi.analyzeFrame({ exam_id: examId, attempt_id: attempt.id, image_base64: imageBase64 })
      setConnectionOk(true)
    } catch {
      setConnectionOk(false)
    }
  }

  // Countdown timer.
  useEffect(() => {
    if (secondsLeft <= 0) return
    const t = setInterval(() => setSecondsLeft((s) => Math.max(0, s - 1)), 1000)
    return () => clearInterval(t)
  }, [secondsLeft])

  useEffect(() => {
    if (secondsLeft === 0 && attempt) {
      handleSubmit()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [secondsLeft])

  const currentQuestion = questions[currentIndex]

  const recordAnswer = async (value) => {
    if (!currentQuestion || !attempt) return
    setAnswers((a) => ({ ...a, [currentQuestion.id]: value }))
    const payload = { attempt_id: attempt.id, question_id: currentQuestion.id }
    if (currentQuestion.type === 'MULTIPLE_CHOICE') payload.selected_option_id = value
    else payload.text_answer = value
    await examApi.submitAnswer(payload)
  }

  const handleSubmit = async () => {
    if (!attempt) return
    setSubmitting(true)
    try {
      await examApi.submitAttempt(attempt.id)
      navigate(`/student/exams/${examId}/result`)
    } finally {
      setSubmitting(false)
    }
  }

  const timeLabel = useMemo(() => {
    const m = Math.floor(secondsLeft / 60)
    const s = secondsLeft % 60
    return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  }, [secondsLeft])

  if (!exam || !currentQuestion) return <div className="es-write__loading">Loading examination…</div>

  return (
    <div className="es-write">
      <header className="es-write__header">
        <div>
          <div className="es-write__title">{exam.title}</div>
          <div className="es-write__timer">TIME REMAINING: {timeLabel}</div>
        </div>
        <div className="es-write__status-strip">
          <StatusChip label="AI Monitoring" ok={exam.enable_ai_monitoring} />
          <StatusChip label="Camera" ok={cameraActive} />
          <StatusChip label="Connection" ok={connectionOk} okLabel="Good" badLabel="Poor" />
        </div>
      </header>

      {warning && <div className="es-write__warning"><Alert tone="warning">{warning}</Alert></div>}

      <div className="es-write__body">
        <div className="es-write__navigator">
          {questions.map((q, i) => (
            <button
              key={q.id}
              className={`es-write__nav-item${i === currentIndex ? ' es-write__nav-item--current' : ''}${answers[q.id] ? ' es-write__nav-item--answered' : ''}`}
              onClick={() => exam.allow_navigation && setCurrentIndex(i)}
              disabled={!exam.allow_navigation && i !== currentIndex}
            >
              {i + 1}
            </button>
          ))}
        </div>

        <div className="es-write__question-panel">
          <div className="es-write__question-meta">Question {currentIndex + 1} of {questions.length}</div>
          <h2 className="es-write__question-text">{currentQuestion.text}</h2>

          {currentQuestion.type === 'MULTIPLE_CHOICE' && (
            <div className="es-write__options">
              {currentQuestion.options?.map((opt) => (
                <label key={opt.id} className="es-write__option">
                  <input
                    type="radio"
                    name={currentQuestion.id}
                    checked={answers[currentQuestion.id] === opt.id}
                    onChange={() => recordAnswer(opt.id)}
                  />
                  {opt.text}
                </label>
              ))}
            </div>
          )}

          {currentQuestion.type === 'TRUE_FALSE' && (
            <div className="es-write__options">
              {['true', 'false'].map((val) => (
                <label key={val} className="es-write__option">
                  <input
                    type="radio"
                    name={currentQuestion.id}
                    checked={answers[currentQuestion.id] === val}
                    onChange={() => recordAnswer(val)}
                  />
                  {val === 'true' ? 'True' : 'False'}
                </label>
              ))}
            </div>
          )}

          {currentQuestion.type === 'SHORT_ANSWER' && (
            <textarea
              className="es-write__short-answer"
              value={answers[currentQuestion.id] || ''}
              onChange={(e) => recordAnswer(e.target.value)}
            />
          )}

          <div className="es-write__nav-buttons">
            <Button variant="ghost" disabled={currentIndex === 0} onClick={() => setCurrentIndex((i) => i - 1)}>Previous</Button>
            {currentIndex < questions.length - 1 ? (
              <Button onClick={() => setCurrentIndex((i) => i + 1)}>Next</Button>
            ) : (
              <Button onClick={handleSubmit} disabled={submitting}>{submitting ? 'Submitting…' : 'Submit examination'}</Button>
            )}
          </div>
        </div>
      </div>

      <video ref={videoRef} autoPlay playsInline muted style={{ display: 'none' }} />
      <canvas ref={canvasRef} style={{ display: 'none' }} />
    </div>
  )
}

function StatusChip({ label, ok, okLabel = 'Active', badLabel = 'Inactive' }) {
  return (
    <div className={`es-write__chip${ok ? '' : ' es-write__chip--bad'}`}>
      {label}: {ok ? okLabel : badLabel}
    </div>
  )
}
