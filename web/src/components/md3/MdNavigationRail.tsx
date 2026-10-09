import React from 'react'
import { motion } from 'framer-motion'
import {
  LayoutDashboard,
  Layers,
  Users,
  ShieldCheck,
  Settings,
  Activity,
} from 'lucide-react'
import { AkariLogo } from './AkariLogo'

export type TabKey = 'dashboard' | 'rules' | 'users' | 'security' | 'settings'

interface NavItem {
  key: TabKey
  label: string
  icon: React.ComponentType<{ className?: string }>
  badge?: number | string
}

interface MdNavigationRailProps {
  currentTab: TabKey
  onSelectTab: (tab: TabKey) => void
}

export const MdNavigationRail: React.FC<MdNavigationRailProps> = ({
  currentTab,
  onSelectTab,
}) => {
  const navItems: NavItem[] = [
    { key: 'dashboard', label: '概览仪表盘', icon: LayoutDashboard },
    { key: 'rules', label: '规则与别名', icon: Layers },
    { key: 'users', label: '用户与会话', icon: Users },
    { key: 'security', label: '安全与审计', icon: ShieldCheck },
    { key: 'settings', label: '系统设置', icon: Settings },
  ]

  return (
    <aside className="hidden md:flex flex-col w-64 h-screen sticky top-0 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container-lowest)] border-r border-[var(--md-outline-variant)]/60 px-3.5 py-6 justify-between shrink-0 select-none z-30">
      {/* Brand Header */}
      <div className="flex flex-col gap-6">
        <div className="flex items-center gap-3.5 px-3 py-1">
          <AkariLogo size={44} variant="badge" interactive={true} className="shadow-lg shadow-[var(--md-primary)]/20 rounded-2xl" />
          <div>
            <h1 className="text-base font-bold text-[var(--md-on-surface)] tracking-tight leading-tight">
              Akari Bridge
            </h1>
            <span className="text-xs font-semibold text-[var(--md-primary)] flex items-center gap-1 mt-0.5">
              <span className="w-1.5 h-1.5 rounded-full bg-[var(--md-primary)] inline-block" />
              MD3 Expressive
            </span>
          </div>
        </div>

        {/* Navigation Items */}
        <nav className="flex flex-col gap-1.5 mt-2">
          {navItems.map((item) => {
            const active = currentTab === item.key
            const Icon = item.icon
            return (
              <motion.button
                key={item.key}
                onClick={() => onSelectTab(item.key)}
                whileTap={{ scale: 0.96 }}
                className="group relative flex items-center gap-3.5 w-full h-14 px-4 rounded-full transition-colors cursor-pointer select-none"
              >
                {/* Continuous Sliding Active Pill Background */}
                {active && (
                  <motion.div
                    layoutId="navRailActivePill"
                    transition={{ type: 'spring', stiffness: 450, damping: 35 }}
                    className="absolute inset-0 bg-[var(--md-primary-container)] rounded-full shadow-sm"
                  />
                )}

                {/* Icon Container with active transition */}
                <div className="relative z-10 flex items-center justify-center w-10 h-8 rounded-full">
                  {active && (
                    <motion.div
                      layoutId="navRailIconBadge"
                      transition={{ type: 'spring', stiffness: 450, damping: 35 }}
                      className="absolute inset-0 bg-[var(--md-primary)] rounded-full shadow-sm"
                    />
                  )}
                  <Icon
                    className={`w-5 h-5 relative z-10 transition-colors duration-200 ${
                      active
                        ? 'text-[var(--md-on-primary)]'
                        : 'text-[var(--md-on-surface-variant)] group-hover:text-[var(--md-on-surface)]'
                    }`}
                  />
                </div>

                <span
                  className={`relative z-10 text-sm tracking-wide flex-1 text-left transition-colors duration-200 ${
                    active
                      ? 'font-bold text-[var(--md-on-primary-container)]'
                      : 'font-medium text-[var(--md-on-surface-variant)] group-hover:text-[var(--md-on-surface)]'
                  }`}
                >
                  {item.label}
                </span>

                {item.badge && (
                  <span className="relative z-10 px-2 py-0.5 text-xs font-bold rounded-full bg-[var(--md-primary)] text-white">
                    {item.badge}
                  </span>
                )}
              </motion.button>
            )
          })}
        </nav>
      </div>

      {/* Footer System Status Badge */}
      <motion.div
        whileHover={{ scale: 1.02 }}
        className="px-3.5 py-3 rounded-2xl bg-[var(--md-surface-container)]/80 border border-[var(--md-outline-variant)]/40 text-xs text-[var(--md-on-surface-variant)] flex items-center gap-3 shadow-sm"
      >
        <div className="relative flex items-center justify-center">
          <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 shrink-0" />
          <span className="absolute w-4 h-4 rounded-full bg-emerald-500/40 animate-ping" />
        </div>
        <div className="flex flex-col min-w-0 flex-1">
          <span className="font-bold text-[var(--md-on-surface)] truncate flex items-center gap-1">
            <Activity className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
            <span>网关网桥已就绪</span>
          </span>
          <span className="text-[10px] opacity-75 truncate mt-0.5">JWT & Stream Guarded</span>
        </div>
      </motion.div>
    </aside>
  )
}
