import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import AppShell from './components/layout/AppShell'
import ExamShell from './components/layout/ExamShell'
import ProtectedRoute from './components/layout/ProtectedRoute'

import LoginPage from './pages/LoginPage'

import AdminOverview from './pages/admin/AdminOverview'
import AdminUsers from './pages/admin/AdminUsers'
import AdminExams from './pages/admin/AdminExams'
import ExamCreate from './pages/admin/ExamCreate'
import ExamDetail from './pages/admin/ExamDetail'
import MonitoringDashboard from './pages/admin/MonitoringDashboard'

import LecturerOverview from './pages/lecturer/LecturerOverview'
import LecturerExams from './pages/lecturer/LecturerExams'

import StudentOverview from './pages/student/StudentOverview'
import StudentExams from './pages/student/StudentExams'
import StudentVerify from './pages/student/StudentVerify'
import StudentWrite from './pages/student/StudentWrite'
import StudentResult from './pages/student/StudentResult'

function RoleHome() {
  const { user } = useAuth()
  if (!user) return <Navigate to="/login" replace />
  const home = { SUPER_ADMIN: '/admin', ADMIN: '/admin', LECTURER: '/lecturer', STUDENT: '/student' }[user.role]
  return <Navigate to={home || '/login'} replace />
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<RoleHome />} />

          {/* Admin / Super Admin */}
          <Route
            path="/admin"
            element={
              <ProtectedRoute roles={['SUPER_ADMIN', 'ADMIN']}>
                <AppShell />
              </ProtectedRoute>
            }
          >
            <Route index element={<AdminOverview />} />
            <Route path="users" element={<AdminUsers />} />
            <Route path="exams" element={<AdminExams />} />
            <Route path="exams/create" element={<ExamCreate />} />
            <Route path="exams/:examId" element={<ExamDetail />} />
            <Route path="monitoring/:examId" element={<MonitoringDashboard />} />
          </Route>

          {/* Lecturer */}
          <Route
            path="/lecturer"
            element={
              <ProtectedRoute roles={['LECTURER']}>
                <AppShell />
              </ProtectedRoute>
            }
          >
            <Route index element={<LecturerOverview />} />
            <Route path="exams" element={<LecturerExams />} />
            <Route path="exams/create" element={<ExamCreate />} />
            <Route path="exams/:examId" element={<ExamDetail />} />
            <Route path="monitoring/:examId" element={<MonitoringDashboard />} />
          </Route>

          {/* Student */}
          <Route
            path="/student"
            element={
              <ProtectedRoute roles={['STUDENT']}>
                <AppShell />
              </ProtectedRoute>
            }
          >
            <Route index element={<StudentOverview />} />
            <Route path="exams" element={<StudentExams />} />
          </Route>

          {/* Student exam-taking flow uses a distraction-free shell */}
          <Route
            path="/student/exams/:examId"
            element={
              <ProtectedRoute roles={['STUDENT']}>
                <ExamShell />
              </ProtectedRoute>
            }
          >
            <Route path="verify" element={<StudentVerify />} />
            <Route path="write" element={<StudentWrite />} />
            <Route path="result" element={<StudentResult />} />
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
