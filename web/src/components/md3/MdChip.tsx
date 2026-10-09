import React from 'react'
import { motion } from 'framer-motion'
import { X } from 'lucide-react'

interface MdChipProps {
  label: string
  selected?: boolean
  onClick?: () => void
  icon?: React.ReactNode
  onDelete?: () => void
  className?: string
}

export const MdChip: React.FC<MdChipProps> = ({
  label,
  selected = false,
  onClick,
  icon,
  onDelete,
  className = '',
}) => {
  return (
    <motion.div
      onClick={onClick}
      whileTap={onClick ? { scale: 0.95 } : undefined}
      whileHover={onClick ? { scale: 1.02 } : undefined}
      transition={{ type: 'spring', stiffness: 500, damping: 25 }}
      className={`inline-flex items-center gap-2 h-8 px-3.5 rounded-full text-xs font-semibold transition-colors duration-200 select-none ${
        onClick ? 'cursor-pointer' : ''
      } ${
        selected
          ? 'bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] border border-[var(--md-primary)]/30 shadow-xs'
          : 'bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] text-[var(--md-on-surface-variant)] border border-[var(--md-outline-variant)] hover:bg-[var(--md-surface-container-high)] hover:text-[var(--md-on-surface)]'
      } ${className}`}
    >
      {icon && <span className="text-sm shrink-0">{icon}</span>}
      <span>{label}</span>
      {onDelete && (
        <button
          onClick={(e) => {
            e.stopPropagation()
            onDelete()
          }}
          className="ml-1 p-0.5 rounded-full hover:bg-black/10 dark:hover:bg-white/10 text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-surface)] cursor-pointer"
        >
          <X className="w-3 h-3" />
        </button>
      )}
    </motion.div>
  )
}
