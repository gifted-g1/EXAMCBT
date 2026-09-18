import './Alert.css'

export default function Alert({ tone = 'info', title, children }) {
  return (
    <div className={`es-alert es-alert--${tone}`}>
      {title && <div className="es-alert__title">{title}</div>}
      <div className="es-alert__body">{children}</div>
    </div>
  )
}
