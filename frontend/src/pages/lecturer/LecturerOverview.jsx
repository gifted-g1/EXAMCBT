import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { dashboardApi } from '../../services/api'
import StatWidget from '../../components/ui/StatWidget'
import Card from '../../components/ui/Card'
import Table from '../../components/ui/Table'
import Button from '../../components/ui/Button'
import { StatusBadge } from '../../components/ui/Badge'

export default function LecturerOverview() {
  const [summary, setSummary] = useState(null)

  useEffect(() => {
    dashboardApi.summary().then(setSummary)
  }, [])

  if (!summary) return <p>Loading dashboard…</p>

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h1>Overview</h1>
        <Button as={Link} to="/lecturer/exams/create">Create examination</Button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 24 }}>
        <StatWidget label="Active examinations" value={summary.active_exams?.length || 0} tone="primary" />
        <StatWidget label="Upcoming" value={summary.upcoming_exams?.length || 0} />
        <StatWidget label="Completed" value={summary.completed_exams?.length || 0} tone="success" />
      </div>

      <Card title="Active examinations">
        <Table
          columns={[
            { key: 'title', header: 'Title', render: (row) => <Link to={`/lecturer/exams/${row.id}`}>{row.title}</Link> },
            { key: 'status', header: 'Status', render: (row) => <StatusBadge status="NORMAL" /> },
          ]}
          rows={summary.active_exams}
          emptyMessage="No active examinations."
        />
      </Card>
    </div>
  )
}
