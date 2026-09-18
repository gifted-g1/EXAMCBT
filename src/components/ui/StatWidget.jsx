import './StatWidget.css'

export default function StatWidget({ label, value, tone = 'neutral', hint }) {
  return (
    <div className={`es-stat es-stat--${tone}`}>
      <div className="es-stat__value">{value}</div>
      <div className="es-stat__label">{label}</div>
      {hint && <div className="es-stat__hint">{hint}</div>}
    </div>
  )
}
