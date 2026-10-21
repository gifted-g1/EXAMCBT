import { useEffect, useState } from 'react'
import { NavLink, Navigate, Route, Routes } from 'react-router-dom'
import { onAuthChange, refresh, staffApi } from '../api/staff'
import type { User } from '../api/types'
import { Spinner } from '../components/ui'
import { ExamDetail } from '../components/staff/ExamDetail'
import { ExamList } from '../components/staff/ExamList'
import { RosterView } from '../components/staff/RosterView'
import { UsersView } from '../components/staff/UsersView'
import StaffLogin from './StaffLogin'

/** Staff area: restores the session from the refresh cookie, then shows the
 * dashboard (or the login form). */
export default function StaffDashboard() {
  const [user, setUser] = useState<User | null | undefined>(undefined)

  useEffect(() => {
    const off = onAuthChange(setUser)
    void refresh().then((ok) => {
      if (!ok) setUser(null)
    })
    return off
  }, [])

  if (user === undefined)
    return (
      <main className="grid min-h-screen place-items-center">
        <Spinner />
      </main>
    )
  if (user === null) return <StaffLogin />

  const link = ({ isActive }: { isActive: boolean }) =>
    `block rounded-lg px-3 py-2 text-sm font-medium ${isActive ? 'bg-slate-800 text-white' : 'text-slate-300 hover:bg-slate-800/60'}`

  return (
    <div className="min-h-screen lg:flex">
      <aside className="bg-slate-900 text-white lg:fixed lg:inset-y-0 lg:w-60">
        <div className="flex items-center justify-between px-4 py-4 lg:block">
          <div>
            <p className="font-semibold">🛡️ Exam Shield</p>
            <p className="text-xs text-slate-400">Proctor dashboard</p>
          </div>
        </div>
        <nav className="flex gap-1 overflow-x-auto px-3 pb-3 lg:block lg:space-y-1" aria-label="Main">
          <NavLink to="/staff/exams" className={link}>
            Exams
          </NavLink>
          <NavLink to="/staff/roster" className={link}>
            Student roster
          </NavLink>
          {user.role === 'admin' && (
            <NavLink to="/staff/users" className={link}>
              Staff accounts
            </NavLink>
          )}
        </nav>
        <div className="hidden border-t border-slate-800 px-4 py-4 text-sm lg:absolute lg:bottom-0 lg:block lg:w-full">
          <p className="font-medium">{user.full_name}</p>
          <p className="text-xs text-slate-400">
            {user.username} · {user.role}
          </p>
          <button type="button" className="mt-2 text-xs text-slate-300 underline" onClick={() => void staffApi.logout()}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="flex-1 px-4 py-6 lg:ml-60 lg:px-8">
        <div className="mb-4 flex justify-end lg:hidden">
          <button type="button" className="text-sm text-slate-600 underline" onClick={() => void staffApi.logout()}>
            Sign out ({user.username})
          </button>
        </div>
        <Routes>
          <Route index element={<Navigate to="exams" replace />} />
          <Route path="exams" element={<ExamList user={user} />} />
          <Route path="exams/:id" element={<ExamDetail user={user} />} />
          <Route path="roster" element={<RosterView user={user} />} />
          {user.role === 'admin' && <Route path="users" element={<UsersView me={user} />} />}
          <Route path="*" element={<Navigate to="exams" replace />} />
        </Routes>
      </main>
    </div>
  )
}
