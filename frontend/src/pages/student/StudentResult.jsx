import { useNavigate, useParams } from 'react-router-dom'
import Button from '../../components/ui/Button'
import './StudentResult.css'

export default function StudentResult() {
  const { examId } = useParams()
  const navigate = useNavigate()

  return (
    <div className="es-result">
      <div className="es-result__card">
        <div className="es-result__check">✓</div>
        <h2>Examination submitted</h2>
        <p className="es-result__hint">
          Your answers have been recorded. Your lecturer will review your result once grading is complete,
          including any short-answer questions that require manual review.
        </p>
        <Button onClick={() => navigate('/student/exams')} fullWidth>Back to my examinations</Button>
      </div>
    </div>
  )
}
