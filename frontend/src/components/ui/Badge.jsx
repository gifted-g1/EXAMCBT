import './Badge.css'

const STATUS_MAP = {
  NORMAL: { label: 'Normal', tone: 'success', dot: '🟢' },
  ATTENTION: { label: 'Attention', tone: 'warning', dot: '🟡' },
  SUSPICIOUS: { label: 'Suspicious', tone: 'danger', dot: '🟠' },
  CRITICAL: { label: 'Critical', tone: 'critical', dot: '🔴' },
  DISCONNECTED: { label: 'Disconnected', tone: 'disconnected', dot: '⚫' },
  INFO: { label: 'Info', tone: 'disconnected' },
  LOW: { label: 'Low', tone: 'success' },
  MEDIUM: { label: 'Medium', tone: 'warning' },
  HIGH: { label: 'High', tone: 'danger' },
}

export function StatusBadge({ status }) {
  const meta = STATUS_MAP[status] || { label: status, tone: 'disconnected' }
  return (
    <span className={`es-badge es-badge--${meta.tone}`}>
      {meta.dot && <span aria-hidden="true">{meta.dot}</span>}
      {meta.label}
    </span>
  )
}

export function Badge({ tone = 'neutral', children }) {
  return <span className={`es-badge es-badge--${tone}`}>{children}</span>
}
