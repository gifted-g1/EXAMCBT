import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import Button from '../components/ui/Button'
import { Field, Input } from '../components/ui/Form'
import Alert from '../components/ui/Alert'
import './LoginPage.css'

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const user = await login(email, password)
      const home = { SUPER_ADMIN: '/admin', ADMIN: '/admin', LECTURER: '/lecturer', STUDENT: '/student' }[user.role]
      navigate(home || '/')
    } catch (err) {
      setError(err.response?.data?.error || 'Unable to sign in. Check your credentials and try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="es-login">
      <div className="es-login__panel">
        <div className="es-login__brand">
          <span className="es-login__mark">◆</span>
          <h1>Exam Shield</h1>
        </div>
        <p className="es-login__tagline">Examination integrity, invigilation and monitoring.</p>

        <form onSubmit={handleSubmit}>
          {error && <Alert tone="danger">{error}</Alert>}
          <Field label="Email">
            <Input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@institution.edu" />
          </Field>
          <Field label="Password">
            <Input type="password" required value={password} onChange={(e) => setPassword(e.target.value)} placeholder="••••••••" />
          </Field>
          <Button type="submit" fullWidth disabled={loading}>
            {loading ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </div>
      <div className="es-login__side">
        <div className="es-login__side-content">
          <h2>Secure examinations, wherever they happen.</h2>
          <p>Online or on a local network, Exam Shield verifies identity, monitors integrity, and gives staff a clear, reviewable record — without declaring a verdict on its own.</p>
        </div>
      </div>
    </div>
  )
}
