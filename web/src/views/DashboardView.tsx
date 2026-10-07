import React, { useState } from 'react'
import {
  Radio,
  Users,
  Database,
  Cpu,
  Tv,
  Copy,
  Check,
  PlayCircle,
  Clock,
  Sparkles,
  ChevronRight,
  ShieldCheck,
} from 'lucide-react'
import type { SystemStatus, PlaybackHistory, RulePlugin } from '../types'

interface DashboardViewProps {
  status: SystemStatus | null
  history: PlaybackHistory[]
  rules: RulePlugin[]
  onOpenRules: () => void
  onOpenUsers: () => void
}

export const DashboardView: React.FC<DashboardViewProps> = ({
  status,
  history,
  rules,
  onOpenRules,
  onOpenUsers,
}) => {
  const [copiedUrl, setCopiedUrl] = useState(false)
  const [copiedUser, setCopiedUser] = useState(false)

  const serverUrl = status?.serverUrl || `http://${window.location.hostname}:8096`
  const activeRulesCount = rules.filter((r) => r.enabled).length

  const handleCopy = (text: string, type: 'url' | 'user') => {
    navigator.clipboard.writeText(text)
    if (type === 'url') {
      setCopiedUrl(true)
      setTimeout(() => setCopiedUrl(false), 2000)
    } else {
      setCopiedUser(true)
      setTimeout(() => setCopiedUser(false), 2000)
    }
  }

  const formatUptime = (seconds: number) => {
    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    const mins = Math.floor((seconds % 3600) / 60)
    if (days > 0) return `${days}天 ${hours}小时 ${mins}分`
    if (hours > 0) return `${hours}小时 ${mins}分`
    return `${mins}分钟`
  }

  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 KB'
    const k = 1024
    const sizes = ['Bytes', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Top Banner */}
      <div className="relative overflow-hidden rounded-3xl bg-gradient-to-r from-pink-900/40 via-purple-900/30 to-slate-900 border border-pink-500/20 p-6 sm:p-8 shadow-2xl">
        <div className="absolute -right-10 -top-10 w-64 h-64 bg-pink-500/10 rounded-full blur-3xl pointer-events-none" />
        <div className="relative z-10 flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
          <div>
            <div className="inline-flex items-center space-x-2 px-3 py-1 rounded-full bg-pink-500/10 border border-pink-500/20 text-pink-400 text-xs font-semibold mb-3">
              <Sparkles className="w-3.5 h-3.5" />
              <span>Akari Media 在线运行中</span>
            </div>
            <h1 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight">
              {status?.serverName || 'Akari Media Server'}
            </h1>
            <p className="text-slate-300 text-sm mt-1 max-w-xl">
              无需下载本地视频，直连解析全网动漫源并通过 Emby 协议实时串流推送到 Infuse、Apple TV、VidHub 和播放器。
            </p>
          </div>

          {/* Quick Connect Badge */}
          <div className="bg-slate-950/80 border border-slate-800 backdrop-blur-md rounded-2xl p-4 w-full md:w-auto shrink-0 shadow-lg">
            <div className="text-xs font-semibold text-slate-400 mb-2 flex items-center justify-between">
              <span>Emby 串流地址</span>
              <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full border border-emerald-500/20">
                端口 {status?.httpPort || 8096}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <code className="bg-slate-900 px-3 py-1.5 rounded-xl text-xs font-mono text-pink-300 border border-slate-800">
                {serverUrl}
              </code>
              <button
                onClick={() => handleCopy(serverUrl, 'url')}
                className="p-2 bg-pink-500 hover:bg-pink-600 text-white rounded-xl transition-colors cursor-pointer shadow-md shadow-pink-500/20"
                title="复制服务器地址"
              >
                {copiedUrl ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Stats Cards Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 sm:gap-6">
        {/* Active Rules Card */}
        <div
          onClick={onOpenRules}
          className="group p-5 bg-slate-900/60 hover:bg-slate-900 border border-slate-800 hover:border-pink-500/40 rounded-2xl transition-all cursor-pointer shadow-lg"
        >
          <div className="flex items-center justify-between text-slate-400 mb-3">
            <span className="text-xs font-semibold uppercase tracking-wider">动漫解析源</span>
            <div className="p-2 rounded-xl bg-pink-500/10 text-pink-400 group-hover:bg-pink-500/20 transition-colors">
              <Radio className="w-4 h-4" />
            </div>
          </div>
          <div className="text-2xl font-bold text-white tracking-tight">
            {activeRulesCount} <span className="text-xs text-slate-400 font-normal">/ {rules.length} 启用</span>
          </div>
          <div className="text-xs text-pink-400 flex items-center space-x-1 mt-2">
            <span>管理与调试源</span>
            <ChevronRight className="w-3 h-3 group-hover:translate-x-0.5 transition-transform" />
          </div>
        </div>

        {/* Users Card */}
        <div
          onClick={onOpenUsers}
          className="group p-5 bg-slate-900/60 hover:bg-slate-900 border border-slate-800 hover:border-purple-500/40 rounded-2xl transition-all cursor-pointer shadow-lg"
        >
          <div className="flex items-center justify-between text-slate-400 mb-3">
            <span className="text-xs font-semibold uppercase tracking-wider">系统用户</span>
            <div className="p-2 rounded-xl bg-purple-500/10 text-purple-400 group-hover:bg-purple-500/20 transition-colors">
              <Users className="w-4 h-4" />
            </div>
          </div>
          <div className="text-2xl font-bold text-white tracking-tight">
            {status?.dbStats?.userCount || 1} <span className="text-xs text-slate-400 font-normal">个账号</span>
          </div>
          <div className="text-xs text-purple-400 flex items-center space-x-1 mt-2">
            <span>{status?.singleUserMode ? '单用户免密模式' : '多住户隔离模式'}</span>
            <ChevronRight className="w-3 h-3 group-hover:translate-x-0.5 transition-transform" />
          </div>
        </div>

        {/* History / DB Size */}
        <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-2xl shadow-lg">
          <div className="flex items-center justify-between text-slate-400 mb-3">
            <span className="text-xs font-semibold uppercase tracking-wider">播放记录 & 存储</span>
            <div className="p-2 rounded-xl bg-indigo-500/10 text-indigo-400">
              <Database className="w-4 h-4" />
            </div>
          </div>
          <div className="text-2xl font-bold text-white tracking-tight">
            {status?.dbStats?.historyCount || history.length}{' '}
            <span className="text-xs text-slate-400 font-normal">条播放点</span>
          </div>
          <div className="text-xs text-slate-400 mt-2">
            SQLite 数据库: {formatBytes(status?.dbStats?.dbSizeBytes || 0)}
          </div>
        </div>

        {/* Uptime & Memory */}
        <div className="p-5 bg-slate-900/60 border border-slate-800 rounded-2xl shadow-lg">
          <div className="flex items-center justify-between text-slate-400 mb-3">
            <span className="text-xs font-semibold uppercase tracking-wider">系统运行</span>
            <div className="p-2 rounded-xl bg-emerald-500/10 text-emerald-400">
              <Cpu className="w-4 h-4" />
            </div>
          </div>
          <div className="text-2xl font-bold text-white tracking-tight">
            {status?.memoryAllocMB || 0} <span className="text-xs text-slate-400 font-normal">MB 内存</span>
          </div>
          <div className="text-xs text-slate-400 mt-2 flex items-center space-x-1">
            <Clock className="w-3 h-3" />
            <span>已运行 {formatUptime(status?.uptimeSeconds || 0)}</span>
          </div>
        </div>
      </div>

      {/* Main Two Columns: Client Connect Guide & Recent Watch Activities */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Infuse / Emby Connection Guide Card */}
        <div className="lg:col-span-1 bg-slate-900/60 border border-slate-800 rounded-3xl p-6 shadow-xl flex flex-col justify-between">
          <div>
            <div className="flex items-center space-x-2 text-white font-bold text-base mb-4">
              <Tv className="w-5 h-5 text-pink-400" />
              <span>客户端连接配置 (Infuse / VidHub)</span>
            </div>

            <div className="space-y-3 text-xs">
              <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
                <div className="text-slate-400 mb-1">1. 添加媒体库服务类型</div>
                <div className="font-semibold text-slate-200">选择 Emby / Jellyfin</div>
              </div>

              <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80 flex items-center justify-between">
                <div>
                  <div className="text-slate-400 mb-0.5">2. 服务器地址 (URL)</div>
                  <div className="font-mono text-pink-300 break-all">{serverUrl}</div>
                </div>
                <button
                  onClick={() => handleCopy(serverUrl, 'url')}
                  className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
                >
                  {copiedUrl ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                </button>
              </div>

              <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80 flex items-center justify-between">
                <div>
                  <div className="text-slate-400 mb-0.5">3. 用户名 (User ID)</div>
                  <div className="font-mono font-bold text-slate-200">admin</div>
                </div>
                <button
                  onClick={() => handleCopy('admin', 'user')}
                  className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
                >
                  {copiedUser ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                </button>
              </div>

              <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
                <div className="text-slate-400 mb-1">4. 密码 (Password)</div>
                <div className="text-slate-300">
                  {status?.singleUserMode ? '留空（单用户模式下无需密码）' : '填写您设置的账户密码'}
                </div>
              </div>
            </div>
          </div>

          <div className="mt-6 pt-4 border-t border-slate-800 text-[11px] text-slate-400 flex items-center space-x-1.5">
            <ShieldCheck className="w-4 h-4 text-emerald-400 shrink-0" />
            <span>自动局域网 UDP 7359 广播已开启，支持客户端局域网自动发现</span>
          </div>
        </div>

        {/* Recent Playback History */}
        <div className="lg:col-span-2 bg-slate-900/60 border border-slate-800 rounded-3xl p-6 shadow-xl flex flex-col">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center space-x-2 text-white font-bold text-base">
              <PlayCircle className="w-5 h-5 text-purple-400" />
              <span>近期播放活动</span>
            </div>
            <span className="text-xs text-slate-500">共 {history.length} 条播放记录</span>
          </div>

          <div className="flex-1 overflow-y-auto space-y-3 max-h-[380px] pr-1">
            {history.length === 0 ? (
              <div className="text-center py-16 text-slate-500">
                <PlayCircle className="w-10 h-10 mx-auto mb-2 opacity-30" />
                <p className="text-sm">暂无播放记录，在 Infuse 中点播任意动漫即可自动同步记录进度</p>
              </div>
            ) : (
              history.slice(0, 8).map((h, idx) => {
                const pct =
                  h.totalTicks > 0 ? Math.min(100, Math.round((h.positionTicks / h.totalTicks) * 100)) : 0
                return (
                  <div
                    key={idx}
                    className="p-3.5 bg-slate-950/80 border border-slate-800/80 rounded-2xl flex items-center justify-between gap-4 hover:border-slate-700 transition-colors"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center space-x-2">
                        <span className="font-semibold text-sm text-slate-200 truncate">
                          条目: {h.itemId}
                        </span>
                        {h.played && (
                          <span className="px-2 py-0.5 text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 rounded-full font-medium">
                            已看完
                          </span>
                        )}
                      </div>

                      {/* Progress Bar */}
                      <div className="mt-2 flex items-center space-x-3">
                        <div className="flex-1 h-1.5 bg-slate-800 rounded-full overflow-hidden">
                          <div
                            className="h-full bg-gradient-to-r from-pink-500 to-purple-500 rounded-full"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                        <span className="text-[11px] font-mono text-slate-400 shrink-0">{pct}%</span>
                      </div>
                    </div>

                    <div className="text-right shrink-0">
                      <div className="text-xs font-medium text-slate-300">用户: {h.userId}</div>
                      <div className="text-[10px] text-slate-500 mt-1">
                        {h.lastPlayedDate ? new Date(h.lastPlayedDate).toLocaleString() : '最近'}
                      </div>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
