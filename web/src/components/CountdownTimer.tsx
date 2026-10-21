import { useEffect, useRef, useState } from 'react'

export function formatRemaining(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
}

/**
 * Counts down to `endsAt` using the server clock: `offsetMs` is
 * serverTime − clientTime measured at load, so a wrong PC clock can't give a
 * student extra time. Calls onExpire exactly once.
 */
export function CountdownTimer({
  endsAt,
  offsetMs = 0,
  onExpire,
}: {
  endsAt: string
  offsetMs?: number
  onExpire?: () => void
}) {
  const end = new Date(endsAt).getTime()
  const [remaining, setRemaining] = useState(() => end - (Date.now() + offsetMs))
  const fired = useRef(false)
  const onExpireRef = useRef(onExpire)
  onExpireRef.current = onExpire

  useEffect(() => {
    fired.current = false
    const tick = () => {
      const left = end - (Date.now() + offsetMs)
      setRemaining(left)
      if (left <= 0 && !fired.current) {
        fired.current = true
        onExpireRef.current?.()
      }
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [end, offsetMs])

  const urgent = remaining < 5 * 60 * 1000
  return (
    <div
      role="timer"
      aria-live={urgent ? 'assertive' : 'off'}
      className={`rounded-lg px-3 py-1.5 font-mono text-lg font-semibold tabular-nums ${
        urgent ? 'bg-rose-100 text-rose-700' : 'bg-slate-100 text-slate-800'
      }`}
    >
      {formatRemaining(remaining)}
    </div>
  )
}
