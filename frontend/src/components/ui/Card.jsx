import './Card.css'

export default function Card({ title, action, children, padded = true, className = '' }) {
  return (
    <div className={`es-card ${className}`}>
      {(title || action) && (
        <div className="es-card__header">
          {title && <h3 className="es-card__title">{title}</h3>}
          {action}
        </div>
      )}
      <div className={padded ? 'es-card__body' : ''}>{children}</div>
    </div>
  )
}
