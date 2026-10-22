import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AnswerScript } from '../../api/types'
import { AnswerSheet } from './AnswerSheet'

const script: AnswerScript = {
  session_id: 4,
  exam_id: 1,
  exam_title: 'Networks 101',
  matric_no: 'IFT/22/9203',
  full_name: 'Awodele Kehinde Gabriel',
  status: 'SUBMITTED',
  score: 2,
  max_score: 9,
  auto_marked_max: 4,
  counts: { correct: 1, incorrect: 1, unanswered: 1, needs_marking: 1, marked: 0 },
  items: [
    { question_id: 1, number: 1, kind: 'MCQ', prompt: 'Which layer routes?', options: ['Link', 'Network'], correct_option: 1, response: '1', status: 'correct', marks: 2, awarded: 2 },
    { question_id: 2, number: 2, kind: 'MCQ', prompt: 'HTTPS port?', options: ['80', '443'], correct_option: 1, response: '0', status: 'incorrect', marks: 1, awarded: 0 },
    { question_id: 3, number: 3, kind: 'MCQ', prompt: 'Locked status?', options: ['423', '403'], correct_option: 0, response: '', status: 'unanswered', marks: 1, awarded: 0 },
    { question_id: 4, number: 4, kind: 'TEXT', prompt: 'Explain CIDR', response: 'Classless routing', status: 'needs_marking', marks: 5, awarded: null },
  ],
}

vi.mock('../../api/staff', () => ({ staffApi: { answerScript: vi.fn(() => Promise.resolve(script)) } }))

describe('AnswerSheet', () => {
  it('shows score, what was chosen vs correct, and filters missed questions', async () => {
    render(<AnswerSheet sessionId={4} onClose={() => undefined} />)
    expect(await screen.findByRole('heading', { name: 'Awodele Kehinde Gabriel' })).toBeInTheDocument()
    expect(screen.getByText('Score').parentElement).toHaveTextContent('2 / 9')

    const q2 = screen.getByText('HTTPS port?').closest('li')!
    expect(within(q2).getByText('Wrong')).toBeInTheDocument()
    expect(within(q2).getByText('their answer')).toBeInTheDocument()
    expect(within(q2).getByText('correct answer')).toBeInTheDocument()
    expect(screen.getByText('Classless routing')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: /Missed \(2\)/ }))
    expect(screen.queryByText('Which layer routes?')).toBeNull()
    expect(screen.getByText('HTTPS port?')).toBeInTheDocument()
    expect(screen.getByText('Locked status?')).toBeInTheDocument()
  })
})
