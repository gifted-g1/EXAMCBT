import './Modal.css'

export default function Modal({ open, onClose, title, children, footer, width = 480 }) {
  if (!open) return null
  return (
    <div className="es-modal-overlay" onMouseDown={onClose}>
      <div
        className="es-modal"
        style={{ maxWidth: width }}
        onMouseDown={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        <div className="es-modal__header">
          <h3>{title}</h3>
          <button className="es-modal__close" onClick={onClose} aria-label="Close">×</button>
        </div>
        <div className="es-modal__body">{children}</div>
        {footer && <div className="es-modal__footer">{footer}</div>}
      </div>
    </div>
  )
}
