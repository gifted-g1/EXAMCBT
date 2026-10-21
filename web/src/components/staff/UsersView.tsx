import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { staffApi } from '../../api/staff'
import type { User } from '../../api/types'
import { Button, Card, ErrorText, Field, Spinner, errorMessage, inputClass } from '../ui'

export function UsersView({ me }: { me: User }) {
  const [users, setUsers] = useState<User[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const load = useCallback(() => {
    staffApi.users().then(setUsers, (e) => setError(errorMessage(e)))
  }, [])
  useEffect(load, [load])

  const patch = async (u: User, p: Parameters<typeof staffApi.updateUser>[1]) => {
    setError(null)
    try {
      await staffApi.updateUser(u.id, p)
      load()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-900">Staff accounts</h1>
      <CreateUser onCreated={load} />
      <ErrorText>{error}</ErrorText>
      {!users ? (
        <Spinner />
      ) : (
        <div className="overflow-x-auto rounded-xl bg-white shadow-sm ring-1 ring-slate-200">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">Staff ID</th>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Role</th>
                <th className="px-4 py-3">Active</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {users.map((u) => (
                <tr key={u.id} className={u.is_active ? '' : 'opacity-50'}>
                  <td className="px-4 py-3 font-mono">{u.username}</td>
                  <td className="px-4 py-3">{u.full_name}</td>
                  <td className="px-4 py-3">
                    <select
                      aria-label={`Role of ${u.full_name}`}
                      className="rounded-md border-0 py-1 text-sm ring-1 ring-slate-300"
                      value={u.role}
                      disabled={u.id === me.id}
                      onChange={(e) => patch(u, { role: e.target.value })}
                    >
                      <option value="proctor">proctor</option>
                      <option value="admin">admin</option>
                    </select>
                  </td>
                  <td className="px-4 py-3">{u.is_active ? 'yes' : 'no'}</td>
                  <td className="space-x-2 whitespace-nowrap px-4 py-3 text-right">
                    <Button
                      variant="ghost"
                      className="text-xs"
                      onClick={() => {
                        const pw = window.prompt(`New password for ${u.full_name} (uppercase letters, 6+):`)
                        if (pw) void patch(u, { password: pw.toUpperCase() })
                      }}
                    >
                      Reset password
                    </Button>
                    {u.id !== me.id && (
                      <Button variant="ghost" className="text-xs" onClick={() => patch(u, { is_active: !u.is_active })}>
                        {u.is_active ? 'Deactivate' : 'Activate'}
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function CreateUser({ onCreated }: { onCreated: () => void }) {
  const [username, setUsername] = useState('')
  const [fullName, setFullName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('proctor')
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      await staffApi.createUser({ username, password, full_name: fullName, role })
      setUsername('')
      setFullName('')
      setPassword('')
      onCreated()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <Card>
      <form onSubmit={submit} className="grid items-end gap-4 md:grid-cols-5">
        <Field label="Staff ID (8 digits)">
          <input className={`${inputClass} font-mono`} value={username} maxLength={8} onChange={(e) => setUsername(e.target.value.replace(/\D/g, ''))} required />
        </Field>
        <Field label="Full name">
          <input className={inputClass} value={fullName} onChange={(e) => setFullName(e.target.value)} required />
        </Field>
        <Field label="Password">
          <input className={inputClass} type="password" value={password} onChange={(e) => setPassword(e.target.value.toUpperCase())} required minLength={6} />
        </Field>
        <Field label="Role">
          <select className={inputClass} value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="proctor">proctor</option>
            <option value="admin">admin</option>
          </select>
        </Field>
        <Button type="submit">Add staff</Button>
        <div className="md:col-span-5">
          <ErrorText>{error}</ErrorText>
        </div>
      </form>
    </Card>
  )
}
