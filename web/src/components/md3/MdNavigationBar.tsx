import React from 'react'
import { motion } from 'framer-motion'
import {
  LayoutDashboard,
  Layers,
  Users,
  ShieldCheck,
  Settings,
} from 'lucide-react'
import type { TabKey } from './MdNavigationRail'

interface MdNavigationBarProps {
  currentTab: TabKey
  onSelectTab: (tab: TabKey) => void
}

export const MdNavigationBar: React.FC<MdNavigationBarProps> = ({
  currentTab,
  onSelectTab,
}) => {
  const navItems: Array<{
    key: TabKey
    label: string
    icon: React.ComponentType<{ className?: string }>
  }> = [
    { key: 'dashboard', label: '概览', icon: LayoutDashboard },
    { key: 'rules', label: '规则', icon: Layers },
    { key: 'users', label: '用户', icon: Users },
    { key: 'security', label: '安全', icon: ShieldCheck },
    { key: 'settings', label: '设置', icon: Settings },
  ]

  return (
    <nav className="md:hidden fixed bottom-0 left-0 right-0 h-18 md-bottom-bar border-t border-[var(--md-outline-variant)]/60 px-2 flex items-center justify-around z-40 select-none shadow-lg">
      {navItems.map((item) => {
        const active = currentTab === item.key
        const Icon = item.icon
        return (
          <motion.button
            key={item.key}
            onClick={() => onSelectTab(item.key)}
            whileTap={{ scale: 0.92 }}
            className="relative flex flex-col items-center justify-center flex-1 py-1 gap-1 cursor-pointer select-none"
          >
            <div className="relative flex items-center justify-center w-14 h-8 rounded-full">
              {active && (
                <motion.div
                  layoutId="navBarActivePill"
                  transition={{ type: 'spring', stiffness: 450, damping: 35 }}
                  className="absolute inset-0 bg-[var(--md-primary-container)] rounded-full shadow-sm"
                />
              )}
              <Icon
                className={`w-5 h-5 relative z-10 transition-colors duration-200 ${
                  active
                    ? 'text-[var(--md-on-primary-container)]'
                    : 'text-[var(--md-on-surface-variant)]'
                }`}
              />
            </div>
            <span
              className={`text-[10px] tracking-tight transition-colors duration-200 ${
                active
                  ? 'font-bold text-[var(--md-on-surface)]'
                  : 'text-[var(--md-on-surface-variant)]'
              }`}
            >
              {item.label}
            </span>
          </motion.button>
        )
      })}
    </nav>
  )
}
