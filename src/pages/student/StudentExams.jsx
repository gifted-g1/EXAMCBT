import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { examApi } from '../../services/api'
import Card from '../../components/ui/Card'
import Table from '../../components/ui/Table'
import Button from '../../components/ui/Button'
import { StatusBadge } from '../../components/ui/Badge'

export default function StudentExams() {
  const [exams, setExams] = useState([])
  const navigate = useNavigate()

  useEffect(() => {
    examApi.list().then(setExams)
  }, [])

  const columns = [
    { key: 'title', header: 'Title' },
    { key: 'course', header: 'Course' },
    { key: 'duration_minutes', header: 'Duration (min)' },
    { key: 'status', header: 'Status', render: (row) => <StatusBadge status={statusToBadge(row.status)} /> },
    {
      key: 'action',
      header: '',
      render: (row) =>
        row.status === 'ACTIVE' ? (
          <Button
            size="sm"
            onClick={() => navigate(`/student/exams/${row.id}/${row.require_face_verification ? 'verify' : 'write'}`)}
          >
            Enter examination
          </Button>
        ) : null,
    },
  ]

  return (
    <div>
      <h1>My examinations</h1>
      <Card>
        <Table columns={columns} rows={exams} emptyMessage="No examinations assigned yet." />
      </Card>
    </div>
  )
}

function statusToBadge(status) {
  const map = { ACTIVE: 'NORMAL', SCHEDULED: 'ATTENTION', PUBLISHED: 'ATTENTION', ENDED: 'DISCONNECTED', DRAFT: 'DISCONNECTED' }
  return map[status] || status
}
