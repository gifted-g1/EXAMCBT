import { act, render, screen } from '@testing-library/react'
import { CountdownTimer, formatRemaining } from './CountdownTimer'

describe('formatRemaining', () => {
  it('formats minutes and hours and clamps at zero', () => {
    expect(formatRemaining(65_000)).toBe('01:05')
    expect(formatRemaining(3_725_000)).toBe('1:02:05')
    expect(formatRemaining(-5000)).toBe('00:00')
  })
})

describe('CountdownTimer', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('uses the server clock offset and fires onExpire exactly once', () => {
    const now = Date.now()
    const onExpire = vi.fn()
    // Client clock is 60 s behind the server, so 61 s of "client" time = 1 s left.
    render(<CountdownTimer endsAt={new Date(now + 61_000).toISOString()} offsetMs={60_000} onExpire={onExpire} />)
    expect(screen.getByRole('timer')).toHaveTextContent('00:01')
    act(() => {
      vi.advanceTimersByTime(3000)
    })
    expect(onExpire).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('timer')).toHaveTextContent('00:00')
  })
})
