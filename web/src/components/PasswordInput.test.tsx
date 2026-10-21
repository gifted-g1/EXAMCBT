import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { PasswordInput } from './ui'

function Harness() {
  const [v, setV] = useState('')
  return (
    <label>
      Password
      <PasswordInput value={v} onChange={(x) => setV(x.toUpperCase())} />
    </label>
  )
}

describe('PasswordInput', () => {
  it('hides by default and toggles visibility with the eye button', async () => {
    render(<Harness />)
    const input = screen.getByLabelText('Password', { selector: 'input' })
    expect(input).toHaveAttribute('type', 'password')

    await userEvent.type(input, 'adebayo')
    expect(input).toHaveValue('ADEBAYO')

    const toggle = screen.getByRole('button', { name: 'Show password' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    await userEvent.click(toggle)
    expect(input).toHaveAttribute('type', 'text')
    expect(screen.getByRole('button', { name: 'Hide password' })).toHaveAttribute('aria-pressed', 'true')

    await userEvent.click(screen.getByRole('button', { name: 'Hide password' }))
    expect(input).toHaveAttribute('type', 'password')
  })
})
