import React, { useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X } from 'lucide-react'

interface MdDialogProps {
  open: boolean
  onClose: () => void
  title?: string
  subtitle?: string
  icon?: React.ReactNode
  children: React.ReactNode
  actions?: React.ReactNode
  maxWidth?: 'sm' | 'md' | 'lg' | 'xl' | '2xl'
}

export const MdDialog: React.FC<MdDialogProps> = ({
  open,
  onClose,
  title,
  subtitle,
  icon,
  children,
  actions,
  maxWidth = 'md',
}) => {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && open) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

  let maxWidthClass = 'max-w-md'
  if (maxWidth === 'sm') maxWidthClass = 'max-w-sm'
  if (maxWidth === 'lg') maxWidthClass = 'max-w-lg'
  if (maxWidth === 'xl') maxWidthClass = 'max-w-xl'
  if (maxWidth === '2xl') maxWidthClass = 'max-w-2xl'

  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6 overflow-y-auto">
          {/* Scrim / Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            onClick={onClose}
            className="fixed inset-0 bg-black/60 md-glass"
          />

          {/* Dialog Surface Card with M3 Container Transform */}
          <motion.div
            initial={{ opacity: 0, scale: 0.92, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.94, y: 10, transition: { duration: 0.15 } }}
            transition={{ type: 'spring', stiffness: 350, damping: 28 }}
            className={`relative w-full ${maxWidthClass} bg-[var(--md-surface-container-high)] border border-[var(--md-outline-variant)] rounded-[32px] p-6 sm:p-8 shadow-2xl z-10`}
          >
            {/* Close Button Top Right */}
            <button
              onClick={onClose}
              className="absolute top-5 right-5 p-2 rounded-full text-[var(--md-on-surface-variant)] hover:bg-[var(--md-surface-container-highest)] hover:text-[var(--md-on-surface)] transition-colors cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>

            {(title || icon) && (
              <div className="flex items-start gap-3.5 mb-5 pr-8">
                {icon && (
                  <div className="p-2.5 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] shrink-0 mt-0.5 shadow-xs">
                    {icon}
                  </div>
                )}
                <div className="min-w-0 flex-1">
                  {title && (
                    <h3 className="text-xl font-bold text-[var(--md-on-surface)] tracking-tight">
                      {title}
                    </h3>
                  )}
                  {subtitle && (
                    <p className="text-xs text-[var(--md-on-surface-variant)] mt-1 leading-relaxed">
                      {subtitle}
                    </p>
                  )}
                </div>
              </div>
            )}

            <div className="text-sm text-[var(--md-on-surface-variant)] mb-6 overflow-y-auto max-h-[70vh] p-0.5">
              {children}
            </div>

            {actions && (
              <div className="flex items-center justify-end gap-3 pt-3 border-t border-[var(--md-outline-variant)]/50">
                {actions}
              </div>
            )}
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  )
}
