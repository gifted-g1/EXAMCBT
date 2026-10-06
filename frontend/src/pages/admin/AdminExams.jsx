import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { examApi } from '../../services/api'
import Card from '../../components/ui/Card'
import Table from '../../components/ui/Table'
import Button from '../../components/ui/Button'
import { StatusBadge } from '../../components/ui/Badge'

export default function AdminExams() {
  const [exams, setExams] = useState([])
  const [loading, setLoading] = useState(true)
  const basePath = useLocation().pathname.startsWith('/lecturer') ? '/lecturer' : '/admin'

  useEffect(() => {
    examApi.list().then(setExams).finally(() => setLoading(false))
  }, [])

  const columns = [
    { key: 'title', header: 'Title', render: (row) => <Link to={`${basePath}/exams/${row.id}`}>{row.title}</Link> },
    { key: 'course_code', header: 'Course code' },
    { key: 'mode', header: 'Mode' },
    { key: 'duration_minutes', header: 'Duration (min)' },
    { key: 'status', header: 'Status', render: (row) => <StatusBadge status={statusToBadge(row.status)} /> },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h1>Examinations</h1>
        <Button as={Link} to={`${basePath}/exams/create`}>Create examination</Button>
      </div>
      <Card>
        <Table columns={columns} rows={exams} emptyMessage={loading ? 'Loading…' : 'No examinations yet.'} />
      </Card>
    </div>
  )
}

function statusToBadge(status) {
  const map = { ACTIVE: 'NORMAL', SCHEDULED: 'ATTENTION', PUBLISHED: 'ATTENTION', ENDED: 'DISCONNECTED', DRAFT: 'DISCONNECTED' }
  return map[status] || status
}
