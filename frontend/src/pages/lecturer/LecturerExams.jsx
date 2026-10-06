import AdminExams from '../admin/AdminExams'

// The lecturer's exam list is functionally identical to the admin's
// (same table, same create action) — the AdminExams component already
// adapts its links based on the current route prefix.
export default function LecturerExams() {
  return <AdminExams />
}
