import './Button.css'

export default function Button({
  children,
  variant = 'primary',
  size = 'md',
  fullWidth = false,
  as: As,
  ...rest
}) {
  const className = `es-btn es-btn--${variant} es-btn--${size}${fullWidth ? ' es-btn--full' : ''}`
  if (As) {
    return (
      <As className={className} {...rest}>
        {children}
      </As>
    )
  }
  return (
    <button className={className} {...rest}>
      {children}
    </button>
  )
}
