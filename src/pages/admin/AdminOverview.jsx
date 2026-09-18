import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { dashboardApi } from '../../services/api'
import StatWidget from '../../components/ui/StatWidget'
import Card from '../../components/ui/Card'
import Table from '../../components/ui/Table'
import { StatusBadge } from '../../components/ui/Badge'
import Button from '../../components/ui/Button'

export default function AdminOverview() {
  const [summary, setSummary] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    dashboardApi.summary().then(setSummary).finally(() => setLoading(false))
  }, [])

  if (loading) return <p>Loading dashboard…</p>

  const columns = [
    { key: 'title', header: 'Examination' },
    { key: 'course', header: 'Course' },
    { key: 'mode', header: 'Mode' },
    { key: 'status', header: 'Status', render: (row) => <StatusBadge status={statusToBadge(row.status)} /> },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h1>Overview</h1>
          <p style={{ color: 'var(--color-text-muted)', margin: 0 }}>Examination activity across your institution.</p>
        </div>
        <Button as={Link} to="/admin/exams/create">Create examination</Button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 24 }}>
        <StatWidget label="Total examinations" value={summary.total_exams} />
        <StatWidget label="Active now" value={summary.active_exams?.length || 0} tone="primary" />
        <StatWidget label="Upcoming" value={summary.upcoming_exams?.length || 0} />
        <StatWidget label="Completed" value={summary.completed_exams?.length || 0} tone="success" />
      </div>

      <Card title="Active examinations">
        <Table
          columns={columns}
          rows={summary.active_exams}
          emptyMessage="No examinations are currently active."
        />
      </Card>
    </div>
  )
}

function statusToBadge(status) {
  const map = { ACTIVE: 'NORMAL', SCHEDULED: 'ATTENTION', PUBLISHED: 'ATTENTION', ENDED: 'DISCONNECTED', DRAFT: 'DISCONNECTED' }
  return map[status] || status
}
