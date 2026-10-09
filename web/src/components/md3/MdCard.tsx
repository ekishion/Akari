import React from 'react'
import { motion, type HTMLMotionProps } from 'framer-motion'

interface MdCardProps extends HTMLMotionProps<'div'> {
  variant?: 'elevated' | 'filled' | 'outlined'
  interactive?: boolean
  children: React.ReactNode
}

export const MdCard: React.FC<MdCardProps> = ({
  variant = 'filled',
  interactive = false,
  className = '',
  children,
  ...props
}) => {
  let baseStyle = 'rounded-[28px] p-6 transition-colors duration-200 '

  if (variant === 'elevated') {
    baseStyle += 'bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] shadow-lg hover:shadow-xl border border-[var(--md-outline-variant)]/60 '
  } else if (variant === 'outlined') {
    baseStyle += 'bg-[var(--md-surface-container-lowest)] dark:bg-[var(--md-surface-container-lowest)] border border-[var(--md-outline-variant)] '
  } else {
    // filled
    baseStyle += 'bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 '
  }

  if (interactive) {
    baseStyle += 'cursor-pointer hover:border-[var(--md-primary)]/50 hover:bg-[var(--md-surface-container)] dark:hover:bg-[var(--md-surface-container-high)] '
  }

  return (
    <motion.div
      whileHover={interactive ? { y: -3, transition: { duration: 0.2 } } : undefined}
      whileTap={interactive ? { scale: 0.985 } : undefined}
      className={`${baseStyle} ${className}`}
      {...props}
    >
      {children}
    </motion.div>
  )
}
