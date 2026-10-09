import React from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  Sun,
  Moon,
  LogOut,
  Radio,
  UserCheck,
} from 'lucide-react'
import { useTheme } from '../../context/ThemeContext'
import { useAuth } from '../../context/AuthContext'
import type { TabKey } from './MdNavigationRail'

interface MdTopAppBarProps {
  currentTab: TabKey
  connected?: boolean
}

export const MdTopAppBar: React.FC<MdTopAppBarProps> = ({
  currentTab,
  connected = true,
}) => {
  const { isDark, toggleTheme } = useTheme()
  const { username, logout } = useAuth()

  const tabTitles: Record<TabKey, { title: string; subtitle: string }> = {
    dashboard: { title: '概览仪表盘', subtitle: '服务实时指标、活跃会话与流代理流量' },
    rules: { title: '规则与别名', subtitle: '番剧源规则、测试沙盒与同义词映射' },
    users: { title: '用户与客户端', subtitle: 'Emby 访问令牌、客户端凭证与会话' },
    security: { title: '安全与审计', subtitle: '管理员鉴权、操作审计日志与 IP 封禁策略' },
    settings: { title: '系统设置', subtitle: '服务端参数、Bangumi 镜像与密码管理' },
  }

  const current = tabTitles[currentTab] || { title: '控制台', subtitle: 'Akari Media Emby Bridge' }

  return (
    <header className="sticky top-0 z-40 flex items-center justify-between px-6 py-4 md-top-app-bar border-b border-[var(--md-outline-variant)]/40 transition-colors">
      {/* Title & Subtitle with AnimatePresence */}
      <div className="min-w-0">
        <AnimatePresence mode="wait">
          <motion.div
            key={currentTab}
            initial={{ opacity: 0, y: -6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 6 }}
            transition={{ duration: 0.2, ease: [0.2, 0, 0, 1] }}
          >
            <h2 className="text-xl sm:text-2xl font-bold text-[var(--md-on-surface)] tracking-tight truncate">
              {current.title}
            </h2>
            <p className="text-xs text-[var(--md-on-surface-variant)] hidden sm:block truncate mt-0.5">
              {current.subtitle}
            </p>
          </motion.div>
        </AnimatePresence>
      </div>

      {/* Right Controls */}
      <div className="flex items-center gap-2.5">
        {/* SSE Connection Pill */}
        <div
          className={`hidden sm:flex items-center gap-2 px-3.5 py-1.5 rounded-full text-xs font-semibold border transition-colors ${
            connected
              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
              : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
          }`}
        >
          <div className="relative flex items-center justify-center">
            <Radio className="w-3.5 h-3.5 relative z-10" />
            {connected && (
              <span className="absolute w-3.5 h-3.5 rounded-full bg-emerald-500/30 animate-ping" />
            )}
          </div>
          <span>{connected ? 'SSE 实时遥测' : '正在重连...'}</span>
        </div>

        {/* Theme Toggle Button */}
        <motion.button
          onClick={toggleTheme}
          whileHover={{ scale: 1.06 }}
          whileTap={{ scale: 0.92 }}
          aria-label="Toggle Theme"
          className="flex items-center justify-center w-10 h-10 rounded-full bg-[var(--md-surface-container)] hover:bg-[var(--md-surface-container-high)] text-[var(--md-on-surface)] border border-[var(--md-outline-variant)]/50 transition-colors cursor-pointer shadow-sm"
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={isDark ? 'dark' : 'light'}
              initial={{ rotate: -90, scale: 0.6, opacity: 0 }}
              animate={{ rotate: 0, scale: 1, opacity: 1 }}
              exit={{ rotate: 90, scale: 0.6, opacity: 0 }}
              transition={{ duration: 0.2 }}
            >
              {isDark ? (
                <Sun className="w-4.5 h-4.5 text-amber-400" />
              ) : (
                <Moon className="w-4.5 h-4.5 text-[var(--md-primary)]" />
              )}
            </motion.div>
          </AnimatePresence>
        </motion.button>

        {/* Admin Profile Pill & Logout */}
        <div className="flex items-center gap-2 pl-2 border-l border-[var(--md-outline-variant)]/60">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 text-xs font-medium text-[var(--md-on-surface)] shadow-xs">
            <UserCheck className="w-4 h-4 text-[var(--md-primary)]" />
            <span className="font-semibold">{username}</span>
          </div>

          <motion.button
            onClick={() => logout()}
            whileHover={{ scale: 1.06 }}
            whileTap={{ scale: 0.92 }}
            title="退出登录"
            className="flex items-center justify-center w-10 h-10 rounded-full bg-[var(--md-surface-container-low)] hover:bg-[var(--md-danger-container)] text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-danger-container)] border border-[var(--md-outline-variant)]/50 transition-colors cursor-pointer"
          >
            <LogOut className="w-4 h-4" />
          </motion.button>
        </div>
      </div>
    </header>
  )
}
