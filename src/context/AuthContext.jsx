import { createContext, useContext, useMemo, useState } from 'react'
import { authApi } from '../services/api'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(() => authApi.currentUser())

  const login = async (email, password) => {
    const result = await authApi.login(email, password)
    authApi.saveSession(result)
    setUser(result.user)
    return result.user
  }

  const logout = async () => {
    try {
      await authApi.logout()
    } finally {
      authApi.clearSession()
      setUser(null)
    }
  }

  const value = useMemo(() => ({ user, login, logout }), [user])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
