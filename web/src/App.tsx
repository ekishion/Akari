import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { AuthProvider, useAuth } from './context/AuthContext'
import { ThemeProvider } from './context/ThemeContext'
import { LoginView } from './views/LoginView'
import { DashboardView } from './views/DashboardView'
import { RulesView } from './views/RulesView'
import { UsersView } from './views/UsersView'
import { SecurityView } from './views/SecurityView'
import { SettingsView } from './views/SettingsView'
import { MdNavigationRail, type TabKey } from './components/md3/MdNavigationRail'
import { MdNavigationBar } from './components/md3/MdNavigationBar'
import { MdTopAppBar } from './components/md3/MdTopAppBar'
import { api } from './api'
import type { SystemConfig, SystemStatus, UserView, RulePlugin } from './types'

function MainContent() {
  const { isAuthenticated, loading: authLoading } = useAuth()
  const [currentTab, setCurrentTab] = useState<TabKey>('dashboard')
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [config, setConfig] = useState<SystemConfig | null>(null)
  const [users, setUsers] = useState<UserView[]>([])
  const [rules, setRules] = useState<RulePlugin[]>([])
  const [sseConnected, setSseConnected] = useState(true)

  const loadData = async () => {
    if (!isAuthenticated) return
    try {
      const [statusRes, configRes, usersRes, rulesRes] = await Promise.all([
        api.getStatus().catch(() => null),
        api.getConfig().catch(() => null),
        api.listUsers().catch(() => []),
        api.listRules().catch(() => []),
      ])

      if (statusRes) setStatus(statusRes)
      if (configRes) setConfig(configRes)
      if (usersRes) setUsers(usersRes)
      if (rulesRes) setRules(rulesRes)
    } catch (err) {
      console.error('Failed to load application data:', err)
    }
  }

  useEffect(() => {
    if (isAuthenticated) {
      loadData()
      // Setup SSE connection listener
      const unsubscribe = api.subscribeTelemetry(
        () => {
          setSseConnected(true)
        },
        () => {
          setSseConnected(false)
        }
      )
      return () => unsubscribe()
    }
  }, [isAuthenticated])

  if (authLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[var(--md-surface)] text-[var(--md-on-surface)]">
        <div className="flex flex-col items-center gap-3">
          <div className="w-8 h-8 rounded-full border-4 border-[var(--md-primary)] border-t-transparent animate-spin" />
          <span className="text-xs font-medium text-[var(--md-on-surface-variant)]">
            加载安全控制台中...
          </span>
        </div>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <LoginView />
  }

  return (
    <div className="flex min-h-screen bg-[var(--md-surface)] text-[var(--md-on-surface)]">
      {/* 1. Desktop Navigation Rail */}
      <MdNavigationRail currentTab={currentTab} onSelectTab={setCurrentTab} />

      {/* 2. Main Content Canvas */}
      <div className="flex-1 flex flex-col min-w-0 pb-20 md:pb-6">
        {/* Top App Bar */}
        <MdTopAppBar currentTab={currentTab} connected={sseConnected} />

        {/* View Canvas with Continuous Motion */}
        <main className="flex-1 px-4 sm:px-8 py-6 max-w-7xl w-full mx-auto">
          <AnimatePresence mode="wait">
            <motion.div
              key={currentTab}
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.25, ease: [0.2, 0, 0, 1] }}
              className="w-full"
            >
              {currentTab === 'dashboard' && (
                <DashboardView onNavigateTab={(tab) => setCurrentTab(tab)} />
              )}
              {currentTab === 'rules' && (
                <RulesView rules={rules} onRefresh={loadData} />
              )}
              {currentTab === 'users' && (
                <UsersView users={users} onRefresh={loadData} />
              )}
              {currentTab === 'security' && <SecurityView />}
              {currentTab === 'settings' && (
                <SettingsView config={config} status={status} onRefresh={loadData} />
              )}
            </motion.div>
          </AnimatePresence>
        </main>
      </div>

      {/* 3. Mobile Navigation Bar */}
      <MdNavigationBar currentTab={currentTab} onSelectTab={setCurrentTab} />
    </div>
  )
}

export function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <MainContent />
      </AuthProvider>
    </ThemeProvider>
  )
}

export default App

