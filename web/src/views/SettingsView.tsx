import React, { useState, useEffect } from 'react'
import {
  Settings,
  Shield,
  Trash2,
  CheckCircle2,
  AlertCircle,
  Server,
  Zap,
  Globe,
  Activity,
  Save,
} from 'lucide-react'
import { api } from '../api'
import type { SystemConfig, SystemStatus, MirrorTestResult } from '../types'

interface SettingsViewProps {
  config: SystemConfig | null
  status: SystemStatus | null
  onRefresh: () => void
}

const BGM_PRESETS = [
  { name: '官方直连 (默认)', url: 'https://api.bgm.tv', desc: '官方原站 API' },
  { name: 'Rin Cat 镜像', url: 'https://mirror.bgm.rin.cat', desc: '国内高速反代节点' },
  { name: 'Chii 镜像', url: 'https://chii.ai', desc: '高可用公共反代' },
]

const DANDAN_PRESETS = [
  { name: '官方节点', url: 'https://api.dandanplay.net' },
  { name: '免签高可用节点', url: 'https://ddplay.retr0.xyz' },
]

export const SettingsView: React.FC<SettingsViewProps> = ({ config, status, onRefresh }) => {
  // Config form states
  const [bangumiHost, setBangumiHost] = useState(config?.bangumiHost || 'https://api.bgm.tv')
  const [dandanHost, setDanDanHost] = useState(config?.dandanHost || 'https://api.dandanplay.net')
  const [serverName, setServerName] = useState(config?.serverName || 'Akari Media')
  const [customProxy, setCustomProxy] = useState(config?.customProxy || '')

  // Sync with prop updates
  useEffect(() => {
    if (config) {
      setBangumiHost(config.bangumiHost || 'https://api.bgm.tv')
      setDanDanHost(config.dandanHost || 'https://api.dandanplay.net')
      setServerName(config.serverName || 'Akari Media')
      setCustomProxy(config.customProxy || '')
    }
  }, [config])

  // Config save state
  const [saveLoading, setSaveLoading] = useState(false)
  const [saveSuccess, setSaveSuccess] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)

  // Mirror testing state
  const [testingBgm, setTestingBgm] = useState(false)
  const [bgmTestResult, setBgmTestResult] = useState<MirrorTestResult | null>(null)

  // Password update states
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwdLoading, setPwdLoading] = useState(false)
  const [pwdSuccess, setPwdSuccess] = useState<string | null>(null)
  const [pwdError, setPwdError] = useState<string | null>(null)

  // Cache cleaning state
  const [cleaning, setCleaning] = useState(false)
  const [cleanMsg, setCleanMsg] = useState<string | null>(null)

  // Test Bangumi endpoint latency
  const handleTestBangumi = async (testUrl?: string) => {
    const target = (testUrl || bangumiHost).trim()
    if (!target) return
    setTestingBgm(true)
    setBgmTestResult(null)
    try {
      const res = await api.testBangumiMirror(target)
      setBgmTestResult(res)
    } catch (err: any) {
      setBgmTestResult({
        success: false,
        error: err.message || '测试失败',
      })
    } finally {
      setTestingBgm(false)
    }
  }

  // Save network and server config
  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaveLoading(true)
    setSaveSuccess(null)
    setSaveError(null)

    try {
      await api.updateConfig({
        bangumiHost: bangumiHost.trim(),
        dandanHost: dandanHost.trim(),
        serverName: serverName.trim(),
        customProxy: customProxy.trim(),
      })
      setSaveSuccess('服务参数与 Bangumi 镜像源已保存并实时生效！')
      setTimeout(() => setSaveSuccess(null), 4000)
      onRefresh()
    } catch (err: any) {
      setSaveError(err.message || '保存配置失败')
    } finally {
      setSaveLoading(false)
    }
  }

  // Update password
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

  // Clean cache
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

  const isBgmPreset = BGM_PRESETS.some((p) => p.url === bangumiHost.trim())

  return (
    <div className="space-y-6 animate-fade-in max-w-4xl">
      {/* Header */}
      <div>
        <h2 className="text-xl font-bold text-white tracking-tight flex items-center space-x-2">
          <Settings className="w-5 h-5 text-slate-400" />
          <span>系统设置与网络配置</span>
        </h2>
        <p className="text-xs text-slate-400 mt-0.5">
          配置 Bangumi API 镜像源、DanDanPlay 弹幕端点、管理员密码与系统缓存维护
        </p>
      </div>

      {/* Bangumi Mirror Configuration Card */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-3xl p-6 shadow-xl space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-2.5 text-white font-bold text-base">
            <div className="p-1.5 bg-pink-500/10 rounded-lg text-pink-400 border border-pink-500/20">
              <Globe className="w-4 h-4" />
            </div>
            <span>Bangumi 番组计划 API 镜像源设置</span>
          </div>
          <span className="text-[11px] px-2.5 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-slate-700">
            热更新 · 无需重启
          </span>
        </div>

        <p className="text-xs text-slate-400 leading-relaxed">
          当直连官方主站出现网络波动或 DNS 污染时，可无缝切换至国内镜像节点或自建 Cloudflare Worker 反代，以提升番剧索引、每日放送和元数据的加载速度。
        </p>

        {/* Presets Selector */}
        <div className="space-y-2 pt-1">
          <label className="block text-xs font-semibold text-slate-300">常用预设镜像节点</label>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
            {BGM_PRESETS.map((preset) => {
              const active = bangumiHost.trim() === preset.url
              return (
                <button
                  key={preset.url}
                  type="button"
                  onClick={() => {
                    setBangumiHost(preset.url)
                    handleTestBangumi(preset.url)
                  }}
                  className={`p-3 rounded-2xl text-left border transition-all cursor-pointer flex flex-col justify-between ${
                    active
                      ? 'bg-pink-500/10 border-pink-500/40 text-pink-200 shadow-sm shadow-pink-500/10 ring-1 ring-pink-500/30'
                      : 'bg-slate-950/60 border-slate-800/80 text-slate-300 hover:border-slate-700 hover:bg-slate-950'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-semibold text-xs text-white">{preset.name}</span>
                    {active && <span className="w-2 h-2 rounded-full bg-pink-400 animate-pulse" />}
                  </div>
                  <div className="font-mono text-[11px] text-slate-400 truncate mt-1">{preset.url}</div>
                  <div className="text-[10px] text-slate-500 mt-1">{preset.desc}</div>
                </button>
              )
            })}
          </div>
        </div>

        {/* Custom Input & Test Bar */}
        <div className="space-y-2 pt-2">
          <div className="flex items-center justify-between">
            <label className="block text-xs font-semibold text-slate-300">
              API 端点 URL {!isBgmPreset && <span className="text-pink-400 font-normal">(自定义节点)</span>}
            </label>
            {bgmTestResult && (
              <div
                className={`text-xs flex items-center space-x-1.5 font-medium px-2 py-0.5 rounded-lg border ${
                  bgmTestResult.success
                    ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20'
                    : 'text-red-400 bg-red-500/10 border-red-500/20'
                }`}
              >
                {bgmTestResult.success ? (
                  <>
                    <CheckCircle2 className="w-3.5 h-3.5" />
                    <span>
                      {bgmTestResult.message || '连接正常'} · 延迟 {bgmTestResult.latencyMs}ms
                    </span>
                  </>
                ) : (
                  <>
                    <AlertCircle className="w-3.5 h-3.5" />
                    <span className="truncate max-w-[260px]">{bgmTestResult.error || '连接失败'}</span>
                  </>
                )}
              </div>
            )}
          </div>

          <div className="flex flex-col sm:flex-row gap-2">
            <input
              type="text"
              value={bangumiHost}
              onChange={(e) => setBangumiHost(e.target.value)}
              placeholder="https://api.bgm.tv"
              className="flex-1 px-4 py-2.5 bg-slate-950 border border-slate-700 rounded-xl text-xs font-mono text-slate-100 focus:outline-none focus:border-pink-500"
            />
            <button
              type="button"
              onClick={() => handleTestBangumi()}
              disabled={testingBgm || !bangumiHost.trim()}
              className="px-4 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center justify-center space-x-1.5 transition-colors cursor-pointer disabled:opacity-50 shrink-0"
            >
              <Activity className={`w-3.5 h-3.5 ${testingBgm ? 'animate-spin text-pink-400' : 'text-slate-400'}`} />
              <span>{testingBgm ? '测速中...' : '测试连通性'}</span>
            </button>
          </div>
        </div>
      </div>

      {/* General Network & Server Parameters */}
      <form onSubmit={handleSaveConfig} className="bg-slate-900/70 border border-slate-800 rounded-3xl p-6 shadow-xl space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-2 text-white font-bold text-base">
            <Server className="w-5 h-5 text-indigo-400" />
            <span>核心服务与弹幕网络源</span>
          </div>
        </div>

        {saveError && (
          <div className="p-3 bg-red-500/10 border border-red-500/20 text-red-400 text-xs rounded-xl flex items-center space-x-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{saveError}</span>
          </div>
        )}

        {saveSuccess && (
          <div className="p-3 bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs rounded-xl flex items-center space-x-2">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{saveSuccess}</span>
          </div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">服务器显示名称</label>
            <input
              type="text"
              value={serverName}
              onChange={(e) => setServerName(e.target.value)}
              placeholder="Akari Media"
              className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-xl text-xs text-slate-100 focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">DanDanPlay 弹幕 API 端点</label>
            <div className="space-y-1.5">
              <input
                type="text"
                value={dandanHost}
                onChange={(e) => setDanDanHost(e.target.value)}
                placeholder="https://api.dandanplay.net"
                className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-xl text-xs font-mono text-slate-100 focus:outline-none focus:border-indigo-500"
              />
              <div className="flex gap-1.5">
                {DANDAN_PRESETS.map((dp) => (
                  <button
                    key={dp.url}
                    type="button"
                    onClick={() => setDanDanHost(dp.url)}
                    className={`text-[10px] px-2 py-0.5 rounded-lg border transition-colors cursor-pointer ${
                      dandanHost === dp.url
                        ? 'bg-indigo-500/20 text-indigo-300 border-indigo-500/30 font-medium'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:text-slate-200'
                    }`}
                  >
                    {dp.name}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">自定义系统网络代理 (HTTP/HTTPS)</label>
            <input
              type="text"
              value={customProxy}
              onChange={(e) => setCustomProxy(e.target.value)}
              placeholder="例如 http://127.0.0.1:7890 (留空为系统默认)"
              className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-xl text-xs font-mono text-slate-100 focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div className="p-3 bg-slate-950/80 rounded-xl border border-slate-800/80">
            <div className="text-slate-500">运行平台与架构</div>
            <div className="text-slate-300 mt-0.5">
              {status?.os} / {status?.arch} ({status?.goVersion})
            </div>
          </div>
        </div>

        {/* Readonly info pills */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 pt-2 text-[11px]">
          <div className="p-2.5 bg-slate-950/50 rounded-xl border border-slate-800/60">
            <div className="text-slate-500">HTTP 端口</div>
            <div className="font-semibold text-slate-200 mt-0.5">{config?.httpPort}</div>
          </div>
          <div className="p-2.5 bg-slate-950/50 rounded-xl border border-slate-800/60">
            <div className="text-slate-500">UDP 发现端口</div>
            <div className="font-semibold text-slate-200 mt-0.5">{config?.udpPort}</div>
          </div>
          <div className="p-2.5 bg-slate-950/50 rounded-xl border border-slate-800/60 col-span-2">
            <div className="text-slate-500">存储数据路径</div>
            <div className="font-mono text-slate-300 mt-0.5 truncate">{config?.dataDir}</div>
          </div>
        </div>

        <div className="flex justify-end pt-2">
          <button
            type="submit"
            disabled={saveLoading}
            className="px-5 py-2.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl shadow-md shadow-indigo-600/20 transition-all cursor-pointer disabled:opacity-50 flex items-center space-x-1.5"
          >
            <Save className="w-3.5 h-3.5" />
            <span>{saveLoading ? '保存中...' : '保存网络与服务设置'}</span>
          </button>
        </div>
      </form>

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
