import React from 'react'
import { motion, type HTMLMotionProps } from 'framer-motion'

interface MdButtonProps extends Omit<HTMLMotionProps<'button'>, 'children'> {
  variant?: 'filled' | 'tonal' | 'outlined' | 'text' | 'danger'
  size?: 'sm' | 'md' | 'lg'
  icon?: React.ReactNode
  loading?: boolean
  disabled?: boolean
  children: React.ReactNode
}

const SIZES = {
  sm: 'h-8 px-3.5 text-xs gap-1.5',
  md: 'h-10 px-5 text-sm gap-2',
  lg: 'h-12 px-6 text-base gap-2.5',
}

const VARIANTS = {
  filled: 'bg-[var(--md-primary)] text-[var(--md-on-primary)] shadow-sm hover:shadow-md hover:brightness-105 active:brightness-95',
  tonal: 'bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] hover:brightness-95 active:brightness-90',
  outlined: 'border border-[var(--md-outline)] text-[var(--md-primary)] hover:bg-[var(--md-primary-container)]/20 active:bg-[var(--md-primary-container)]/30',
  text: 'text-[var(--md-primary)] hover:bg-[var(--md-primary-container)]/20 active:bg-[var(--md-primary-container)]/30',
  danger: 'bg-[var(--md-danger)] text-white hover:brightness-105 active:brightness-95',
}

export const MdButton: React.FC<MdButtonProps> = ({
  variant = 'filled',
  size = 'md',
  icon,
  loading = false,
  disabled = false,
  className = '',
  children,
  ...props
}) => {
  const sizeClass = SIZES[size] || SIZES.md
  const variantClass = VARIANTS[variant] || VARIANTS.filled

  const isInteractive = !disabled && !loading

  return (
    <motion.button
      disabled={disabled || loading}
      whileHover={isInteractive ? { scale: 1.02 } : undefined}
      whileTap={isInteractive ? { scale: 0.96 } : undefined}
      transition={{ type: 'spring', stiffness: 500, damping: 25 }}
      className={`inline-flex items-center justify-center font-semibold rounded-full transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:shadow-none select-none ${sizeClass} ${variantClass} ${className}`}
      {...props}
    >
      {loading ? (
        <svg className="animate-spin h-4 w-4 mr-1 text-current" viewBox="0 0 24 24" fill="none">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
        </svg>
      ) : (
        icon && <span className="inline-flex shrink-0">{icon}</span>
      )}
      <span>{children}</span>
    </motion.button>
  )
}
