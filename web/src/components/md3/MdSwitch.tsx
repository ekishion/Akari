import React from 'react'
import { motion } from 'framer-motion'
import { Check } from 'lucide-react'

interface MdSwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  label?: string
  className?: string
}

export const MdSwitch: React.FC<MdSwitchProps> = ({
  checked,
  onChange,
  disabled = false,
  label,
  className = '',
}) => {
  return (
    <label
      className={`inline-flex items-center gap-3 cursor-pointer select-none ${
        disabled ? 'opacity-50 cursor-not-allowed' : ''
      } ${className}`}
    >
      <motion.div
        onClick={() => !disabled && onChange(!checked)}
        whileTap={{ scale: disabled ? 1 : 0.95 }}
        className={`relative inline-flex h-8 w-14 items-center rounded-full transition-colors duration-200 border-2 ${
          checked
            ? 'bg-[var(--md-primary)] border-[var(--md-primary)] shadow-sm'
            : 'bg-[var(--md-surface-container-highest)] border-[var(--md-outline)]'
        }`}
      >
        <motion.span
          layout
          transition={{ type: 'spring', stiffness: 500, damping: 30 }}
          className={`inline-flex items-center justify-center rounded-full shadow-sm ${
            checked
              ? 'translate-x-6.5 w-6 h-6 bg-[var(--md-on-primary)] text-[var(--md-primary)]'
              : 'translate-x-1 w-4 h-4 bg-[var(--md-outline)] text-transparent'
          }`}
        >
          {checked && <Check className="w-3.5 h-3.5 stroke-[3]" />}
        </motion.span>
      </motion.div>
      {label && <span className="text-sm font-semibold text-[var(--md-on-surface)]">{label}</span>}
    </label>
  )
}
