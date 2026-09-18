import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { monitoringApi, openMonitoringSocket } from '../../services/api'
import Card from '../../components/ui/Card'
import StatWidget from '../../components/ui/StatWidget'
import Table from '../../components/ui/Table'
import Modal from '../../components/ui/Modal'
import { StatusBadge } from '../../components/ui/Badge'
import Button from '../../components/ui/Button'

export default function MonitoringDashboard() {
  const { examId } = useParams()
  const [students, setStudents] = useState({}) // student_id -> live state
  const [events, setEvents] = useState([])
  const [selected, setSelected] = useState(null)
  const socketRef = useRef(null)

  useEffect(() => {
    monitoringApi.listEvents(examId).then(setEvents)

    const ws = openMonitoringSocket(examId)
    socketRef.current = ws

    ws.onmessage = (msg) => {
      const data = JSON.parse(msg.data)
      handleEvent(data)
    }

    return () => ws.close()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [examId])

  const handleEvent = (msg) => {
    const studentId = msg.payload?.student_id
    if (!studentId) return

    setStudents((prev) => {
      const existing = prev[studentId] || { student_id: studentId, status: 'NORMAL', camera: 'ACTIVE', verification: 'PENDING', suspiciousEvents: 0 }
      let next = { ...existing }

      switch (msg.type) {
        case 'STUDENT_JOINED':
        case 'STUDENT_STARTED':
          next.status = 'NORMAL'
          break
        case 'STUDENT_VERIFIED':
          next.verification = 'VERIFIED'
          break
        case 'FACE_VERIFICATION':
          next.verification = msg.payload.matched ? 'VERIFIED' : 'FAILED'
          break
        case 'STUDENT_SUBMITTED':
          next.status = 'DISCONNECTED'
          next.completed = true
          break
        case 'STUDENT_DISCONNECTED':
          next.status = 'DISCONNECTED'
          break
        case 'CAMERA_STATUS':
          next.camera = msg.payload.active ? 'ACTIVE' : 'INACTIVE'
          break
        case 'AI_MONITORING_EVENT':
          setEvents((prevEvents) => [msg.payload, ...prevEvents])
          break
        case 'SUSPICIOUS_ACTIVITY':
          next.suspiciousEvents = (next.suspiciousEvents || 0) + 1
          next.status = severityToStatus(msg.payload.severity)
          break
        default:
          break
      }
      return { ...prev, [studentId]: next }
    })
  }

  const studentList = useMemo(() => Object.values(students), [students])

  const counts = useMemo(() => {
    const c = { total: studentList.length, active: 0, completed: 0, notStarted: 0, disconnected: 0, alerts: events.length, highRisk: 0 }
    studentList.forEach((s) => {
      if (s.completed) c.completed++
      else if (s.status === 'DISCONNECTED') c.disconnected++
      else c.active++
    })
    events.forEach((e) => {
      if (e.severity === 'HIGH' || e.severity === 'CRITICAL') c.highRisk++
    })
    return c
  }, [studentList, events])

  const columns = [
    { key: 'student_id', header: 'Student ID' },
    { key: 'status', header: 'Status', render: (row) => <StatusBadge status={row.status} /> },
    { key: 'verification', header: 'Verification' },
    { key: 'camera', header: 'Camera' },
    { key: 'suspiciousEvents', header: 'Suspicious events', render: (row) => row.suspiciousEvents || 0 },
  ]

  return (
    <div>
      <h1>Examination monitoring</h1>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(6, 1fr)', gap: 14, marginBottom: 24 }}>
        <StatWidget label="Total students" value={counts.total} />
        <StatWidget label="Active" value={counts.active} tone="primary" />
        <StatWidget label="Completed" value={counts.completed} tone="success" />
        <StatWidget label="Disconnected" value={counts.disconnected} />
        <StatWidget label="Alerts" value={counts.alerts} tone="warning" />
        <StatWidget label="High-risk events" value={counts.highRisk} tone="danger" />
      </div>

      <Card title="Students">
        <Table
          columns={columns}
          rows={studentList}
          onRowClick={(row) => setSelected(row)}
          emptyMessage="Waiting for students to join…"
        />
      </Card>

      <div style={{ height: 20 }} />

      <Card title="AI event timeline">
        <Table
          columns={[
            { key: 'timestamp', header: 'Time', render: (row) => new Date(row.timestamp).toLocaleTimeString() },
            { key: 'student_id', header: 'Student' },
            { key: 'event_type', header: 'Event' },
            { key: 'severity', header: 'Severity', render: (row) => <StatusBadge status={row.severity} /> },
            { key: 'description', header: 'Description' },
          ]}
          rows={events}
          emptyMessage="No AI events recorded yet."
        />
      </Card>

      <Modal open={!!selected} onClose={() => setSelected(null)} title={`Student ${selected?.student_id || ''}`}>
        {selected && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <Row label="Examination status"><StatusBadge status={selected.status} /></Row>
            <Row label="Verification">{selected.verification}</Row>
            <Row label="Camera">{selected.camera}</Row>
            <Row label="Suspicious events">{selected.suspiciousEvents || 0}</Row>
            <div style={{ marginTop: 8 }}>
              <Button size="sm" variant="ghost" onClick={() => setSelected(null)}>Close</Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}

function Row({ label, children }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 14 }}>
      <span style={{ color: 'var(--color-text-muted)' }}>{label}</span>
      <span>{children}</span>
    </div>
  )
}

function severityToStatus(severity) {
  return { LOW: 'ATTENTION', MEDIUM: 'ATTENTION', HIGH: 'SUSPICIOUS', CRITICAL: 'CRITICAL' }[severity] || 'NORMAL'
}
