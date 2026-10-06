import { useEffect, useState } from 'react'
import { userApi } from '../../services/api'
import Card from '../../components/ui/Card'
import Table from '../../components/ui/Table'
import Button from '../../components/ui/Button'
import Modal from '../../components/ui/Modal'
import { Field, Input, Select } from '../../components/ui/Form'
import { Badge } from '../../components/ui/Badge'
import { authApi } from '../../services/api'

export default function AdminUsers() {
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [form, setForm] = useState({ full_name: '', email: '', password: '', role: 'STUDENT' })
  const [submitting, setSubmitting] = useState(false)
  const currentUser = authApi.currentUser()

  const load = () => {
    setLoading(true)
    userApi.list().then(setUsers).finally(() => setLoading(false))
  }

  useEffect(load, [])

  const handleCreate = async (e) => {
    e.preventDefault()
    setSubmitting(true)
    try {
      await authApi.register(form)
      setCreateOpen(false)
      setForm({ full_name: '', email: '', password: '', role: 'STUDENT' })
      load()
    } finally {
      setSubmitting(false)
    }
  }

  const toggleSuspend = async (user) => {
    if (user.suspended) await userApi.reinstate(user.id)
    else await userApi.suspend(user.id)
    load()
  }

  const columns = [
    { key: 'full_name', header: 'Name' },
    { key: 'email', header: 'Email' },
    { key: 'role', header: 'Role' },
    {
      key: 'status',
      header: 'Status',
      render: (row) => (row.suspended ? <Badge tone="danger">Suspended</Badge> : <Badge tone="success">Active</Badge>),
    },
    {
      key: 'actions',
      header: '',
      render: (row) => (
        <Button size="sm" variant="ghost" onClick={() => toggleSuspend(row)}>
          {row.suspended ? 'Reinstate' : 'Suspend'}
        </Button>
      ),
    },
  ]

  const assignableRoles = currentUser?.role === 'SUPER_ADMIN'
    ? ['ADMIN', 'LECTURER', 'STUDENT']
    : ['LECTURER', 'STUDENT']

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h1>Users</h1>
        <Button onClick={() => setCreateOpen(true)}>Add user</Button>
      </div>

      <Card>
        <Table columns={columns} rows={users} emptyMessage={loading ? 'Loading…' : 'No users yet.'} />
      </Card>

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Add a new user">
        <form onSubmit={handleCreate}>
          <Field label="Full name">
            <Input required value={form.full_name} onChange={(e) => setForm({ ...form, full_name: e.target.value })} />
          </Field>
          <Field label="Email">
            <Input type="email" required value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
          </Field>
          <Field label="Temporary password">
            <Input type="password" required value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
          </Field>
          <Field label="Role">
            <Select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
              {assignableRoles.map((r) => (
                <option key={r} value={r}>{r}</option>
              ))}
            </Select>
          </Field>
          <Button type="submit" fullWidth disabled={submitting}>{submitting ? 'Creating…' : 'Create account'}</Button>
        </form>
      </Modal>
    </div>
  )
}
