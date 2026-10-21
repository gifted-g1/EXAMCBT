import { useState, type FormEvent } from 'react'
import { Button, ErrorText, inputClass } from '../components/ui'

export type LockCode =
  | 'LAN_REQUIRED'
  | 'EXAM_NOT_OPEN'
  | 'PIN_REQUIRED'
  | 'PIN_INVALID'
  | 'CONFLICT'
  | 'INSECURE_CONTEXT'
  | 'CAMERA_DENIED'
  | 'DEVICE_IN_USE'
  | 'STUDENT_ACTIVE_ELSEWHERE'
  | 'DEVICE_MISMATCH'
  | 'ALREADY_SUBMITTED'
  | 'SESSION_ENDED'
  | 'SESSION_LOCK_LOST'
  | 'RATE_LIMITED'
  | 'NETWORK'
  | string

const copy: Record<string, { title: string; body: string; status?: string }> = {
  LAN_REQUIRED: {
    status: '403',
    title: 'Exam hall network required',
    body: 'This exam only opens on the exam hall network. Connect to the hall Wi-Fi or LAN, then type the address your invigilator shows you.',
  },
  EXAM_NOT_OPEN: {
    status: '423',
    title: 'No exam is open right now',
    body: 'The exam has not started yet, or it has ended. Wait for your invigilator to open it — this page checks again automatically.',
  },
  PIN_REQUIRED: { status: '401', title: 'Enter the exam PIN', body: 'Type the 4-digit PIN shown on the projector.' },
  PIN_INVALID: { status: '401', title: 'That PIN is not correct', body: 'Check the PIN on the projector and try again.' },
  CONFLICT: {
    title: 'More than one exam is open here',
    body: 'Your invigilator needs to set a PIN for each exam. Please let them know.',
  },
  INSECURE_CONTEXT: {
    title: 'Secure connection required',
    body: 'Your browser only allows camera access over a secure (https://) connection. Ask your invigilator for the https:// address, or to enable camera access for this exam computer.',
  },
  CAMERA_DENIED: {
    title: 'Camera access is needed',
    body: 'This exam is proctored. Allow camera access in your browser (the camera icon in the address bar), then try again.',
  },
  DEVICE_IN_USE: {
    title: 'This computer is in use',
    body: "Another student's exam is running on this computer. Please move to a free computer or call an invigilator.",
  },
  STUDENT_ACTIVE_ELSEWHERE: {
    title: 'You are already signed in elsewhere',
    body: 'Your exam is open on another computer. If you changed computers, call an invigilator to unlock your session.',
  },
  DEVICE_MISMATCH: {
    title: 'Your exam is on another computer',
    body: 'For security your exam stays on the computer where you started. Call an invigilator to unlock it.',
  },
  SESSION_LOCK_LOST: {
    title: 'Your session was moved',
    body: 'An invigilator unlocked your session or it was opened on another computer. Call an invigilator if this is unexpected.',
  },
  ALREADY_SUBMITTED: { title: 'Already submitted', body: 'You have already submitted this exam. You may leave quietly.' },
  SESSION_ENDED: { title: 'Exam ended', body: 'This exam session has ended.' },
  RATE_LIMITED: { status: '429', title: 'Too many attempts', body: 'Please wait a few minutes, or call an invigilator.' },
  NETWORK: { title: 'Cannot reach the exam server', body: 'Check that you are still connected to the hall network.' },
}

export function Locked({
  code,
  message,
  onPin,
  onRetry,
}: {
  code: LockCode
  message?: string
  onPin?: (pin: string) => void
  onRetry?: () => void
}) {
  const c = copy[code] ?? { title: 'Unable to continue', body: message ?? 'Please call an invigilator.' }
  const wantsPin = (code === 'PIN_REQUIRED' || code === 'PIN_INVALID') && onPin
  const [pin, setPin] = useState('')

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (/^\d{4}$/.test(pin)) onPin?.(pin)
  }

  return (
    <main className="grid min-h-screen place-items-center bg-slate-900 px-4 py-10 text-slate-100">
      <section className="w-full max-w-md text-center" aria-labelledby="lock-title">
        <div aria-hidden className="mx-auto mb-6 grid h-14 w-14 place-items-center rounded-2xl bg-brand-600 text-2xl">
          🛡️
        </div>
        <h1 id="lock-title" className="text-2xl font-semibold">
          {c.title}
        </h1>
        <p className="mt-3 leading-relaxed text-slate-300">{c.body}</p>

        {wantsPin && (
          <form onSubmit={submit} className="mx-auto mt-6 max-w-xs space-y-3 text-left">
            <label htmlFor="pin" className="sr-only">
              Exam PIN
            </label>
            <input
              id="pin"
              inputMode="numeric"
              autoComplete="one-time-code"
              autoFocus
              maxLength={4}
              value={pin}
              onChange={(e) => setPin(e.target.value.replace(/\D/g, ''))}
              className={`${inputClass} text-center font-mono text-3xl tracking-[0.5em]`}
              placeholder="••••"
            />
            {code === 'PIN_INVALID' && <ErrorText>Incorrect PIN.</ErrorText>}
            <Button type="submit" className="w-full" disabled={pin.length !== 4}>
              Continue
            </Button>
          </form>
        )}

        {!wantsPin && onRetry && (
          <Button variant="secondary" className="mt-6" onClick={onRetry}>
            Try again
          </Button>
        )}
        {c.status && <p className="mt-8 text-xs text-slate-500">Error {c.status} · {code}</p>}
      </section>
    </main>
  )
}
