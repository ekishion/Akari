import { LayoutDashboard, Radio, Users, History, Settings, LogOut, RefreshCw } from 'lucide-react'

interface NavbarProps {
  currentTab: string
  setCurrentTab: (tab: string) => void
  user: any
  onLogout?: () => void
  onRefresh?: () => void
  loading?: boolean
}

export const Navbar: React.FC<NavbarProps> = ({
  currentTab,
  setCurrentTab,
  user,
  onLogout,
  onRefresh,
  loading,
}) => {
  const navItems = [
    { id: 'dashboard', label: '概览仪表盘', icon: LayoutDashboard },
    { id: 'rules', label: '动漫解析源', icon: Radio },
    { id: 'users', label: '用户与令牌', icon: Users },
    { id: 'history', label: '播放历史', icon: History },
    { id: 'settings', label: '系统设置', icon: Settings },
  ]

  return (
    <header className="sticky top-0 z-40 backdrop-blur-xl bg-slate-950/80 border-b border-slate-800/80">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo & Brand */}
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-pink-500 via-purple-600 to-indigo-500 flex items-center justify-center shadow-lg shadow-pink-500/20 text-white font-bold text-xl">
              🌸
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <span className="font-bold text-lg bg-gradient-to-r from-pink-400 via-purple-300 to-indigo-300 bg-clip-text text-transparent tracking-tight">
                  Akari Media
                </span>
                <span className="px-2 py-0.5 text-xs font-semibold rounded-full bg-pink-500/10 text-pink-400 border border-pink-500/20">
                  Emby Core
                </span>
              </div>
              <p className="text-xs text-slate-400">分布式追番 & 直连代理网关</p>
            </div>
          </div>

          {/* Navigation Links */}
          <nav className="hidden md:flex items-center space-x-1">
            {navItems.map((item) => {
              const Icon = item.icon
              const isActive = currentTab === item.id
              return (
                <button
                  key={item.id}
                  onClick={() => setCurrentTab(item.id)}
                  className={`flex items-center space-x-2 px-3.5 py-2 rounded-xl text-sm font-medium transition-all duration-200 ${
                    isActive
                      ? 'bg-pink-500/15 text-pink-400 border border-pink-500/30 shadow-sm shadow-pink-500/10'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
                  }`}
                >
                  <Icon className={`w-4 h-4 ${isActive ? 'text-pink-400' : 'text-slate-400'}`} />
                  <span>{item.label}</span>
                </button>
              )
            })}
          </nav>

          {/* Actions & User */}
          <div className="flex items-center space-x-3">
            {onRefresh && (
              <button
                onClick={onRefresh}
                title="刷新数据"
                disabled={loading}
                className="p-2 text-slate-400 hover:text-slate-200 hover:bg-slate-900 rounded-lg transition-colors"
              >
                <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin text-pink-400' : ''}`} />
              </button>
            )}

            {user && (
              <div className="flex items-center space-x-2 pl-2 border-l border-slate-800">
                <div className="w-8 h-8 rounded-full bg-slate-800 border border-slate-700 flex items-center justify-center text-xs font-semibold text-pink-300">
                  {user.name ? user.name.slice(0, 1).toUpperCase() : 'A'}
                </div>
                <div className="hidden sm:block text-left">
                  <div className="text-xs font-medium text-slate-200">{user.name || 'Admin'}</div>
                  <div className="text-[10px] text-pink-400/80">
                    {user.isAdmin ? '超级管理员' : '标准用户'}
                  </div>
                </div>
                {onLogout && (
                  <button
                    onClick={onLogout}
                    title="退出登录"
                    className="p-1.5 text-slate-400 hover:text-red-400 rounded-lg transition-colors ml-1"
                  >
                    <LogOut className="w-4 h-4" />
                  </button>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Mobile Navigation */}
        <div className="flex md:hidden overflow-x-auto py-2 space-x-2 border-t border-slate-900">
          {navItems.map((item) => {
            const Icon = item.icon
            const isActive = currentTab === item.id
            return (
              <button
                key={item.id}
                onClick={() => setCurrentTab(item.id)}
                className={`flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap ${
                  isActive
                    ? 'bg-pink-500/20 text-pink-300 border border-pink-500/30'
                    : 'text-slate-400 hover:bg-slate-900'
                }`}
              >
                <Icon className="w-3.5 h-3.5" />
                <span>{item.label}</span>
              </button>
            )
          })}
        </div>
      </div>
    </header>
  )
}
