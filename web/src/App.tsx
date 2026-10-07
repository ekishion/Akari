import React, { useState, useEffect } from 'react'
import { api } from './api'
import type { SystemStatus, SystemConfig, UserView, RulePlugin, PlaybackHistory } from './types'
import { Navbar } from './components/Navbar'
import { DashboardView } from './views/DashboardView'
import { RulesView } from './views/RulesView'
import { UsersView } from './views/UsersView'
import { HistoryView } from './views/HistoryView'
import { SettingsView } from './views/SettingsView'
import { LogIn, AlertCircle } from 'lucide-react'

export function App() {
  const [currentTab, setCurrentTab] = useState('dashboard')
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [config, setConfig] = useState<SystemConfig | null>(null)
  const [users, setUsers] = useState<UserView[]>([])
  const [rules, setRules] = useState<RulePlugin[]>([])
  const [history, setHistory] = useState<PlaybackHistory[]>([])
  const [loading, setLoading] = useState(true)

  // Auth State
  const [currentUser, setCurrentUser] = useState<any | null>(null)
  const [authRequired, setAuthRequired] = useState(false)
  const [loginUsername, setLoginUsername] = useState('admin')
  const [loginPassword, setLoginPassword] = useState('')
  const [loginError, setLoginError] = useState<string | null>(null)
  const [loginLoading, setLoginLoading] = useState(false)

  const loadData = async () => {
    setLoading(true)
    try {
      const [statusRes, configRes, usersRes, rulesRes, historyRes] = await Promise.all([
        api.getStatus().catch(() => null),
        api.getConfig().catch(() => null),
        api.listUsers().catch(() => []),
        api.listRules().catch(() => []),
        api.listHistory().catch(() => []),
      ])

      if (statusRes) setStatus(statusRes)
      if (configRes) setConfig(configRes)
      if (usersRes) setUsers(usersRes)
      if (rulesRes) setRules(rulesRes)
      if (historyRes) setHistory(historyRes)

      // Try checking if user is logged in
      try {
        const me = await api.getMe()
        if (me && !me.guest) {
          setCurrentUser(me)
          setAuthRequired(false)
        } else if (configRes?.hasPassword) {
          setAuthRequired(true)
        } else {
          // Zero-config passwordless mode, auto login as admin
          setCurrentUser({ name: 'Admin', isAdmin: true })
          setAuthRequired(false)
        }
      } catch {
        if (configRes?.hasPassword) {
          setAuthRequired(true)
        }
      }
    } catch (err) {
      console.error('Failed to load system data', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoginLoading(true)
    setLoginError(null)

    try {
      const res = await api.login(loginUsername, loginPassword)
      localStorage.setItem('akari_token', res.token)
      setCurrentUser(res.user)
      setAuthRequired(false)
      await loadData()
    } catch (err: any) {
      setLoginError(err.message || '登录失败，请检查账号密码')
    } finally {
      setLoginLoading(false)
    }
  }

  const handleLogout = () => {
    localStorage.removeItem('akari_token')
    setCurrentUser(null)
    setAuthRequired(true)
  }

  if (authRequired) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4 bg-slate-950 relative overflow-hidden">
        {/* Decorative background glows */}
        <div className="absolute -top-40 -left-40 w-96 h-96 bg-pink-500/10 rounded-full blur-3xl" />
        <div className="absolute -bottom-40 -right-40 w-96 h-96 bg-purple-500/10 rounded-full blur-3xl" />

        <div className="bg-slate-900/90 border border-slate-800 backdrop-blur-xl w-full max-w-md rounded-3xl p-8 shadow-2xl relative z-10 animate-fade-in">
          <div className="text-center mb-8">
            <div className="w-14 h-14 rounded-2xl bg-gradient-to-tr from-pink-500 via-purple-600 to-indigo-500 flex items-center justify-center shadow-lg shadow-pink-500/20 text-white font-bold text-2xl mx-auto mb-4">
              🌸
            </div>
            <h1 className="text-2xl font-bold text-white tracking-tight">Akari Media</h1>
            <p className="text-xs text-slate-400 mt-1">请输入管理员账号密码以进入控制面板</p>
          </div>

          {loginError && (
            <div className="mb-4 p-3.5 bg-red-500/10 border border-red-500/20 text-red-400 text-xs rounded-xl flex items-center space-x-2">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{loginError}</span>
            </div>
          )}

          <form onSubmit={handleLogin} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                用户名 (Username)
              </label>
              <input
                type="text"
                required
                value={loginUsername}
                onChange={(e) => setLoginUsername(e.target.value)}
                className="w-full px-4 py-2.5 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 focus:outline-none focus:border-pink-500 transition-colors"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                密码 (Password)
              </label>
              <input
                type="password"
                required
                value={loginPassword}
                onChange={(e) => setLoginPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full px-4 py-2.5 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 focus:outline-none focus:border-pink-500 transition-colors"
              />
            </div>

            <button
              type="submit"
              disabled={loginLoading}
              className="w-full py-3 bg-gradient-to-r from-pink-500 via-purple-600 to-indigo-600 hover:opacity-90 text-white font-semibold text-sm rounded-xl transition-all shadow-lg shadow-pink-500/25 flex items-center justify-center space-x-2 cursor-pointer disabled:opacity-50 mt-2"
            >
              <LogIn className="w-4 h-4" />
              <span>{loginLoading ? '登录验证中...' : '登录控制台'}</span>
            </button>
          </form>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex flex-col bg-slate-950 text-slate-100">
      <Navbar
        currentTab={currentTab}
        setCurrentTab={setCurrentTab}
        user={currentUser}
        onLogout={config?.hasPassword ? handleLogout : undefined}
        onRefresh={loadData}
        loading={loading}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 sm:py-8">
        {currentTab === 'dashboard' && (
          <DashboardView
            status={status}
            history={history}
            rules={rules}
            onOpenRules={() => setCurrentTab('rules')}
            onOpenUsers={() => setCurrentTab('users')}
          />
        )}

        {currentTab === 'rules' && <RulesView rules={rules} onRefresh={loadData} />}

        {currentTab === 'users' && <UsersView users={users} onRefresh={loadData} />}

        {currentTab === 'history' && <HistoryView history={history} />}

        {currentTab === 'settings' && (
          <SettingsView config={config} status={status} onRefresh={loadData} />
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-900 py-6 text-center text-xs text-slate-500">
        <div className="max-w-7xl mx-auto px-4 flex flex-col sm:flex-row items-center justify-between gap-2">
          <div>Akari Media &copy; {new Date().getFullYear()} - 优雅的分布式动漫串流引擎</div>
          <div className="flex items-center space-x-4 text-slate-400">
            <span>Go {status?.goVersion || '1.24+'}</span>
            <span>•</span>
            <span>SQLite Embedded</span>
            <span>•</span>
            <span>Emby Protocol Standard</span>
          </div>
        </div>
      </footer>
    </div>
  )
}
export default App
