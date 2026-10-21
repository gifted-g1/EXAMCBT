import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Locked } from './Locked'

describe('Locked', () => {
  it('explains LAN_REQUIRED with the 403 code', () => {
    render(<Locked code="LAN_REQUIRED" />)
    expect(screen.getByRole('heading')).toHaveTextContent('Exam hall network required')
    expect(screen.getByText(/403/)).toBeInTheDocument()
  })

  it('accepts only a 4-digit PIN', async () => {
    const onPin = vi.fn()
    render(<Locked code="PIN_REQUIRED" onPin={onPin} />)
    const input = screen.getByLabelText('Exam PIN')
    const button = screen.getByRole('button', { name: 'Continue' })
    await userEvent.type(input, '12a')
    expect(input).toHaveValue('12')
    expect(button).toBeDisabled()
    await userEvent.type(input, '34')
    await userEvent.click(button)
    expect(onPin).toHaveBeenCalledWith('1234')
  })

  it('shows a retry for EXAM_NOT_OPEN', async () => {
    const onRetry = vi.fn()
    render(<Locked code="EXAM_NOT_OPEN" onRetry={onRetry} />)
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(onRetry).toHaveBeenCalled()
  })

  it('never renders navigation or a logout control', () => {
    render(<Locked code="DEVICE_IN_USE" />)
    expect(screen.queryByRole('navigation')).toBeNull()
    expect(screen.queryByRole('link')).toBeNull()
    expect(screen.queryByText(/log ?out|sign out/i)).toBeNull()
  })
})
