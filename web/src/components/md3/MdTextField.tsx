import React from 'react'

interface MdTextFieldProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string
  helperText?: string
  error?: string
  leadingIcon?: React.ReactNode
  trailingIcon?: React.ReactNode
}

export const MdTextField: React.FC<MdTextFieldProps> = ({
  label,
  helperText,
  error,
  leadingIcon,
  trailingIcon,
  className = '',
  disabled,
  id,
  ...props
}) => {
  const inputId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined)

  return (
    <div className="flex flex-col gap-1.5 w-full">
      {label && (
        <label
          htmlFor={inputId}
          className="text-xs font-semibold text-[var(--md-on-surface-variant)] px-1"
        >
          {label}
        </label>
      )}
      <div
        className={`relative flex items-center bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] rounded-2xl border transition-all duration-200 px-4 py-2.5 focus-within:ring-2 focus-within:ring-[var(--md-primary)] focus-within:border-[var(--md-primary)] focus-within:bg-[var(--md-surface-container-lowest)] dark:focus-within:bg-[var(--md-surface-container-low)] ${
          error
            ? 'border-[var(--md-danger)] ring-1 ring-[var(--md-danger)]'
            : 'border-[var(--md-outline-variant)] hover:border-[var(--md-outline)]'
        } ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
      >
        {leadingIcon && <span className="mr-3 text-[var(--md-on-surface-variant)] shrink-0">{leadingIcon}</span>}
        <input
          id={inputId}
          disabled={disabled}
          className={`w-full bg-transparent text-[var(--md-on-surface)] text-sm placeholder:text-[var(--md-on-surface-variant)]/50 outline-none disabled:cursor-not-allowed font-medium ${className}`}
          {...props}
        />
        {trailingIcon && <span className="ml-3 text-[var(--md-on-surface-variant)] shrink-0">{trailingIcon}</span>}
      </div>
      {(error || helperText) && (
        <span
          className={`text-xs px-1 ${
            error ? 'text-[var(--md-danger)] font-medium' : 'text-[var(--md-on-surface-variant)]'
          }`}
        >
          {error || helperText}
        </span>
      )}
    </div>
  )
}
