import { Suspense, lazy } from 'react'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { Spinner } from '../components/ui'
import Landing from '../pages/Landing'
import StudentExam from '../pages/StudentExam'

// Staff code is a separate chunk: students never download the dashboard.
const StaffDashboard = lazy(() => import('../pages/StaffDashboard'))

/** "/" shows the landing page, except for exam links that carry a PIN
 * (QR codes, older links): those go straight into the exam. */
function Home() {
  const { search } = useLocation()
  if (new URLSearchParams(search).has('pin')) return <Navigate to={`/exam${search}`} replace />
  return <Landing />
}

/**
 * Route map:
 *   /          → Landing (how it works, start my exam, staff sign in)
 *   /exam      → StudentExam (no nav, no links, no logout)
 *   /staff/*   → StaffDashboard (login, exams, live monitor, roster, users)
 *   anything else → /
 */
export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/exam" element={<StudentExam />} />
        <Route
          path="/staff/*"
          element={
            <Suspense
              fallback={
                <div className="grid min-h-screen place-items-center">
                  <Spinner />
                </div>
              }
            >
              <StaffDashboard />
            </Suspense>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
