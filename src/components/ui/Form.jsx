import './Form.css'

export function Field({ label, hint, error, children }) {
  return (
    <label className="es-field">
      {label && <span className="es-field__label">{label}</span>}
      {children}
      {hint && !error && <span className="es-field__hint">{hint}</span>}
      {error && <span className="es-field__error">{error}</span>}
    </label>
  )
}

export function Input(props) {
  return <input className="es-input" {...props} />
}

export function Select({ children, ...props }) {
  return (
    <select className="es-input es-select" {...props}>
      {children}
    </select>
  )
}

export function Textarea(props) {
  return <textarea className="es-input es-textarea" {...props} />
}

export function Checkbox({ label, ...props }) {
  return (
    <label className="es-checkbox">
      <input type="checkbox" {...props} />
      <span>{label}</span>
    </label>
  )
}
