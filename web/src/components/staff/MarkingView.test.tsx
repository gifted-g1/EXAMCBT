import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Exam, MarkingOverview, QuestionScripts } from '../../api/types'
import { MarkingView } from './MarkingView'

const guide = [
  { point: 'Defines encryption', marks: 4, keywords: ['ciphertext'] },
  { point: 'Names an algorithm', marks: 6, keywords: ['AES'] },
]
const question = { question_id: 7, number: 2, prompt: 'Explain encryption.', marks: 10, marking_guide: guide, model_answer: '', answered: 1, marked: 0 }
const overview: MarkingOverview = { ai_enabled: true, questions: [question] }
const scripts: QuestionScripts = {
  question,
  scripts: [
    {
      answer_id: 3,
      candidate: 'Candidate 1',
      response: 'It produces ciphertext.',
      awarded: null,
      comment: '',
      marked_at: null,
      submitted: true,
      keyword_suggestion: {
        marks: 4,
        max: 10,
        highlights: ['ciphertext'],
        points: [
          { index: 0, point: 'Defines encryption', marks: 4, met: true, matched: ['ciphertext'] },
          { index: 1, point: 'Names an algorithm', marks: 6, met: false, matched: [] },
        ],
      },
      ai_suggestion: { marks: 5, reason: 'Defines it; no algorithm named.', points_met: [0], model: 'claude', at: '2026-10-22T10:00:00Z' },
    },
  ],
}

const markAnswer = vi.fn(() => Promise.resolve({ answer_id: 3, awarded: 5, session_id: 1, session_score: 7 }))
vi.mock('../../api/staff', () => ({
  staffApi: {
    marking: vi.fn(() => Promise.resolve(overview)),
    markingScripts: vi.fn(() => Promise.resolve(scripts)),
    markAnswer: (...a: unknown[]) => markAnswer(...(a as [])),
    aiSuggest: vi.fn(),
    setMarkingGuide: vi.fn(),
  },
}))

const exam = { id: 1 } as Exam

describe('MarkingView', () => {
  it('highlights keywords, shows suggestions and saves the examiner’s mark', async () => {
    render(<MarkingView exam={exam} canManage />)
    const card = (await screen.findByText('Candidate 1')).closest('li')!
    expect(within(card).getByText('ciphertext', { selector: 'mark' })).toBeInTheDocument()
    expect(within(card).getByText('Keyword suggestion: 4 / 10')).toBeInTheDocument()
    expect(within(card).getByText('Defines it; no algorithm named.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Get AI suggestions' })).toBeInTheDocument()

    // Accept the AI suggestion, add a comment, save.
    await userEvent.click(within(card).getAllByRole('button', { name: 'Use' })[1])
    expect(within(card).getByLabelText('Mark for Candidate 1')).toHaveValue(5)
    await userEvent.type(within(card).getByLabelText('Comment for Candidate 1'), 'Name an algorithm')
    await userEvent.click(within(card).getByRole('button', { name: 'Save mark' }))
    expect(markAnswer).toHaveBeenCalledWith(1, 3, 5, 'Name an algorithm', 'ai')
    expect(await within(card).findByText('Marked 5 / 10')).toBeInTheDocument()
  })

  it('is read-only for staff who cannot manage the exam', async () => {
    render(<MarkingView exam={exam} canManage={false} />)
    await screen.findByText('Candidate 1')
    expect(screen.queryByRole('button', { name: 'Save mark' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Get AI suggestions' })).toBeNull()
  })
})
