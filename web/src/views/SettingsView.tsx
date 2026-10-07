import React, { useState } from 'react'
import {
  Settings,
  Shield,
  Trash2,
  CheckCircle2,
  AlertCircle,
  Server,
  Zap,
} from 'lucide-react'
import { api } from '../api'
import type { SystemConfig, SystemStatus } from '../types'

interface SettingsViewProps {
  config: SystemConfig | null
  status: SystemStatus | null
  onRefresh: () => void
}

export const SettingsView: React.FC<SettingsViewProps> = ({ config, status, onRefresh }) => {
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwdLoading, setPwdLoading] = useState(false)
  const [pwdSuccess, setPwdSuccess] = useState<string | null>(null)
  const [pwdError, setPwdError] = useState<string | null>(null)

  const [cleaning, setCleaning] = useState(false)
  const [cleanMsg, setCleanMsg] = useState<string | null>(null)

  const handleUpdatePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      setPwdError('两次输入的密码不一致')
      return
    }

    setPwdLoading(true)
    setPwdError(null)
    setPwdSuccess(null)

    try {
      await api.changePassword('admin', newPassword.trim())
      setPwdSuccess(
        newPassword.trim() === ''
          ? '密码已清除，已切换为单用户免密模式'
          : '管理员密码已更新！'
      )
      setNewPassword('')
      setConfirmPassword('')
      onRefresh()
    } catch (err: any) {
      setPwdError(err.message || '更新密码失败')
    } finally {
      setPwdLoading(false)
    }
  }

  const handleCleanCache = async () => {
    setCleaning(true)
    setCleanMsg(null)
    try {
      const res = await api.cleanCache()
      setCleanMsg(res.message || '缓存已清空')
      setTimeout(() => setCleanMsg(null), 3000)
    } catch (err: any) {
      alert(err.message || '清理缓存失败')
    } finally {
      setCleaning(false)
    }
  }

  return (
    <div className="space-y-6 animate-fade-in max-w-4xl">
      {/* Header */}
      <div>
        <h2 className="text-xl font-bold text-white tracking-tight flex items-center space-x-2">
          <Settings className="w-5 h-5 text-slate-400" />
          <span>系统设置与运维</span>
        </h2>
        <p className="text-xs text-slate-400 mt-0.5">
          配置 Akari Media 核心参数、密码保护、网络代理与缓存维护
        </p>
      </div>

      {/* Admin Security Card */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-3xl p-6 shadow-xl space-y-4">
        <div className="flex items-center space-x-2 text-white font-bold text-base">
          <Shield className="w-5 h-5 text-pink-400" />
          <span>安全与管理员密码</span>
        </div>
        <p className="text-xs text-slate-400">
          当设置管理员密码后，系统会自动开启严格认证模式；若留空则运行在零配置免密单用户模式。
        </p>

        {pwdError && (
          <div className="p-3 bg-red-500/10 border border-red-500/20 text-red-400 text-xs rounded-xl flex items-center space-x-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{pwdError}</span>
          </div>
        )}

        {pwdSuccess && (
          <div className="p-3 bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs rounded-xl flex items-center space-x-2">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{pwdSuccess}</span>
          </div>
        )}

        <form onSubmit={handleUpdatePassword} className="space-y-3 pt-2">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">新密码</label>
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="输入新密码 (留空清除)"
                className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 focus:outline-none focus:border-pink-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">确认新密码</label>
              <input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="再次输入新密码"
                className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 focus:outline-none focus:border-pink-500"
              />
            </div>
          </div>

          <div className="flex justify-end pt-2">
            <button
              type="submit"
              disabled={pwdLoading}
              className="px-5 py-2 bg-pink-500 hover:bg-pink-600 text-white text-xs font-semibold rounded-xl shadow-md shadow-pink-500/20 transition-all cursor-pointer disabled:opacity-50"
            >
              {pwdLoading ? '保存中...' : '更新密码设置'}
            </button>
          </div>
        </form>
      </div>

      {/* Server Properties */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-3xl p-6 shadow-xl space-y-4">
        <div className="flex items-center space-x-2 text-white font-bold text-base">
          <Server className="w-5 h-5 text-indigo-400" />
          <span>服务参数信息</span>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">服务器名称 (Server Name)</div>
            <div className="font-semibold text-slate-200 mt-0.5">{config?.serverName}</div>
          </div>
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">Server ID</div>
            <div className="font-mono text-slate-300 mt-0.5">{config?.serverId}</div>
          </div>
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">HTTP 监听端口</div>
            <div className="font-semibold text-slate-200 mt-0.5">{config?.httpPort}</div>
          </div>
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">UDP 广播发现端口</div>
            <div className="font-semibold text-slate-200 mt-0.5">{config?.udpPort}</div>
          </div>
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">DanDanPlay 弹幕 API</div>
            <div className="font-mono text-slate-300 mt-0.5">{config?.dandanHost}</div>
          </div>
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">Bangumi 番组计划 API</div>
            <div className="font-mono text-slate-300 mt-0.5">{config?.bangumiHost}</div>
          </div>
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">运行平台与架构</div>
            <div className="text-slate-300 mt-0.5">
              {status?.os} / {status?.arch} ({status?.goVersion})
            </div>
          </div>
          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">存储数据路径</div>
            <div className="font-mono text-slate-300 mt-0.5">{config?.dataDir}</div>
          </div>
        </div>
      </div>

      {/* Maintenance & Cache */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-3xl p-6 shadow-xl flex items-center justify-between">
        <div>
          <div className="text-sm font-bold text-white flex items-center space-x-2">
            <Zap className="w-4 h-4 text-amber-400" />
            <span>内存与流嗅探缓存清理</span>
          </div>
          <p className="text-xs text-slate-400 mt-1">
            清空 30 分钟串流临时解析结果与弹幕缓存，释放内存占用
          </p>
          {cleanMsg && (
            <div className="text-xs text-emerald-400 mt-1 font-medium">{cleanMsg}</div>
          )}
        </div>

        <button
          onClick={handleCleanCache}
          disabled={cleaning}
          className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center space-x-1.5 transition-colors cursor-pointer shrink-0"
        >
          <Trash2 className="w-3.5 h-3.5" />
          <span>{cleaning ? '正在清理...' : '清空缓存'}</span>
        </button>
      </div>
    </div>
  )
}
