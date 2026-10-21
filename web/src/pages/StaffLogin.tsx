import { useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { staffApi } from '../api/staff'
import { Button, ErrorText, Field, inputClass } from '../components/ui'

/** The only login form in Exam Shield: 8-digit staff ID + password. */
export default function StaffLogin() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const valid = /^\d{8}$/.test(username) && /^[A-Z]{6,}$/.test(password)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await staffApi.login(username, password)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Cannot reach the server.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="grid min-h-screen place-items-center bg-slate-100 px-4">
      <form onSubmit={submit} className="w-full max-w-sm space-y-5 rounded-2xl bg-white p-8 shadow-sm ring-1 ring-slate-200">
        <div>
          <p className="text-sm font-semibold text-brand-700">Exam Shield</p>
          <h1 className="text-2xl font-semibold text-slate-900">Staff sign in</h1>
        </div>
        <Field label="Staff ID" hint="8 digits">
          <input
            className={`${inputClass} font-mono`}
            inputMode="numeric"
            autoComplete="username"
            maxLength={8}
            value={username}
            onChange={(e) => setUsername(e.target.value.replace(/\D/g, ''))}
            autoFocus
            required
          />
        </Field>
        <Field label="Password" hint="Uppercase letters">
          <input
            className={inputClass}
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value.toUpperCase())}
            required
          />
        </Field>
        <ErrorText>{error}</ErrorText>
        <Button type="submit" className="w-full" disabled={!valid || busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </Button>
      </form>
    </main>
  )
}
