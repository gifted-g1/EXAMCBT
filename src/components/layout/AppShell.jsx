import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import './AppShell.css'

const NAV_BY_ROLE = {
  SUPER_ADMIN: [
    { to: '/admin', label: 'Overview', end: true },
    { to: '/admin/users', label: 'Users & Institutions' },
    { to: '/admin/exams', label: 'Examinations' },
  ],
  ADMIN: [
    { to: '/admin', label: 'Overview', end: true },
    { to: '/admin/users', label: 'Users' },
    { to: '/admin/exams', label: 'Examinations' },
  ],
  LECTURER: [
    { to: '/lecturer', label: 'Overview', end: true },
    { to: '/lecturer/exams', label: 'My Examinations' },
  ],
  STUDENT: [
    { to: '/student', label: 'Overview', end: true },
    { to: '/student/exams', label: 'My Examinations' },
  ],
}

export default function AppShell() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const navItems = NAV_BY_ROLE[user?.role] || []

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  return (
    <div className="es-shell">
      <aside className="es-sidebar">
        <div className="es-sidebar__brand">
          <span className="es-sidebar__mark">◆</span>
          <span>Exam Shield</span>
        </div>
        <nav className="es-sidebar__nav">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => `es-sidebar__link${isActive ? ' es-sidebar__link--active' : ''}`}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="es-sidebar__footer">
          <div className="es-sidebar__role">{roleLabel(user?.role)}</div>
        </div>
      </aside>

      <div className="es-main">
        <header className="es-topbar">
          <div />
          <div className="es-topbar__user">
            <span>{user?.full_name}</span>
            <button className="es-topbar__logout" onClick={handleLogout}>Sign out</button>
          </div>
        </header>
        <main className="es-content">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

function roleLabel(role) {
  return { SUPER_ADMIN: 'Super Admin', ADMIN: 'Administrator', LECTURER: 'Lecturer', STUDENT: 'Student' }[role] || ''
}
