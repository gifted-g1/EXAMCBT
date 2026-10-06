import { Outlet } from 'react-router-dom'
import './ExamShell.css'

export default function ExamShell() {
  return (
    <div className="es-exam-shell">
      <Outlet />
    </div>
  )
}
