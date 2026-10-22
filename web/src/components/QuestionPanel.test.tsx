import { fireEvent, render, screen } from '@testing-library/react'
import type { QuestionView } from '../api/types'
import { QuestionPanel, parseGap } from './QuestionPanel'

const gap: QuestionView = { id: 5, position: 1, kind: 'GAP', prompt: 'TCP opens with _____ then _____.', marks: 2, blanks: 2 }

describe('QuestionPanel fill-in-the-gap', () => {
  it('shows one box per blank and saves a JSON list', () => {
    const onAnswer = vi.fn()
    render(<QuestionPanel questions={[gap]} answers={{ '5': '["SYN",""]' }} onAnswer={onAnswer} />)
    expect(screen.getByLabelText('Blank 1')).toHaveValue('SYN')
    const b2 = screen.getByLabelText('Blank 2')
    fireEvent.change(b2, { target: { value: 'SYN-ACK' } })
    fireEvent.blur(b2)
    expect(onAnswer).toHaveBeenCalledWith(5, '["SYN","SYN-ACK"]')
    expect(screen.getByRole('button', { name: 'Question 1, answered' })).toBeInTheDocument()
  })

  it('parses saved answers defensively', () => {
    expect(parseGap('', 2)).toEqual(['', ''])
    expect(parseGap('not json', 1)).toEqual([''])
    expect(parseGap('["a","b","c"]', 2)).toEqual(['a', 'b'])
  })
})
