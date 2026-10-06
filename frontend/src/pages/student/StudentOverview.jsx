import { useEffect, useState } from 'react'
import { examApi } from '../../services/api'
import Card from '../../components/ui/Card'
import StatWidget from '../../components/ui/StatWidget'
import Table from '../../components/ui/Table'
import { Link } from 'react-router-dom'

export default function StudentOverview() {
  const [exams, setExams] = useState([])

  useEffect(() => {
    examApi.list().then(setExams)
  }, [])

  const upcoming = exams.filter((e) => ['SCHEDULED', 'PUBLISHED'].includes(e.status))
  const active = exams.filter((e) => e.status === 'ACTIVE')
  const completed = exams.filter((e) => e.status === 'ENDED')

  return (
    <div>
      <h1>Overview</h1>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 24 }}>
        <StatWidget label="Active now" value={active.length} tone="primary" />
        <StatWidget label="Upcoming" value={upcoming.length} />
        <StatWidget label="Completed" value={completed.length} tone="success" />
      </div>

      <Card title="Active examinations">
        <Table
          columns={[
            { key: 'title', header: 'Title', render: (row) => <Link to="/student/exams">{row.title}</Link> },
            { key: 'duration_minutes', header: 'Duration (min)' },
          ]}
          rows={active}
          emptyMessage="No examinations are currently active for you."
        />
      </Card>
    </div>
  )
}
