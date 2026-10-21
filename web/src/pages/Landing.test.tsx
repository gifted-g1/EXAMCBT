import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import Landing from './Landing'

beforeAll(() => {
  // jsdom has no IntersectionObserver: the hook must fall back to showing content.
  vi.stubGlobal('IntersectionObserver', undefined)
})

describe('Landing page', () => {
  it('offers both entry points and explains the student steps', () => {
    render(
      <MemoryRouter>
        <Landing />
      </MemoryRouter>,
    )
    expect(screen.getAllByRole('link', { name: /start my exam/i })[0]).toHaveAttribute('href', '/exam')
    expect(screen.getAllByRole('link', { name: /staff sign in/i })[0]).toHaveAttribute('href', '/staff')
    expect(screen.getByRole('heading', { name: 'Join the exam hall Wi-Fi' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /light goes off/i })).toBeInTheDocument()
  })

  it('switches the how-it-works steps to staff', async () => {
    render(
      <MemoryRouter>
        <Landing />
      </MemoryRouter>,
    )
    await userEvent.click(screen.getByRole('tab', { name: /staff/i }))
    expect(screen.getByRole('heading', { name: 'Choose the proctoring level' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Join the exam hall Wi-Fi' })).toBeNull()
  })

  it('offers to resume an exam left open on this computer', () => {
    localStorage.setItem(
      'es_exam_cache',
      JSON.stringify({
        offsetMs: 0,
        savedAt: Date.now(),
        state: { status: 'ACTIVE', ends_at: new Date(Date.now() + 60_000).toISOString(), questions: [] },
      }),
    )
    render(
      <MemoryRouter>
        <Landing />
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: /continue where you left off/i })).toHaveAttribute('href', '/exam')
  })
})

describe('Landing sign-in format', () => {
  it('explains the staff and student login formats without real credentials', () => {
    render(
      <MemoryRouter>
        <Landing />
      </MemoryRouter>,
    )
    expect(screen.getByRole('heading', { name: 'What to type, exactly' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Staff login' })).toBeInTheDocument()
    expect(screen.getByText(/Exactly 8 digits/)).toBeInTheDocument()
    expect(screen.getByText(/Capital letters A–Z, at least 6/)).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Student entry' })).toBeInTheDocument()
    expect(screen.getByText('DEPT / YEAR / NUMBER')).toBeInTheDocument()
    // The public page must not reveal the seeded demo logins.
    expect(document.body.textContent).not.toMatch(/MUSTAPHA|OLAWALE|20010078|19990045/)
  })
})
