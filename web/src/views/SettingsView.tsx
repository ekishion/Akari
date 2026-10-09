import React, { useState, useEffect } from 'react'
import { motion, type Variants } from 'framer-motion'
import {
  Lock,
  Trash2,
  CheckCircle2,
  AlertCircle,
  Server,
  Activity,
  Save,
  Key,
} from 'lucide-react'
import { api } from '../api'
import type { SystemConfig, SystemStatus, MirrorTestResult } from '../types'
import { MdCard } from '../components/md3/MdCard'
import { MdButton } from '../components/md3/MdButton'
import { MdTextField } from '../components/md3/MdTextField'
import { MdChip } from '../components/md3/MdChip'

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: {
      staggerChildren: 0.08,
    },
  },
}

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 15 },
  show: {
    opacity: 1,
    y: 0,
    transition: { type: 'spring', stiffness: 350, damping: 25 },
  },
}

interface SettingsViewProps {
  config: SystemConfig | null
  status?: SystemStatus | null
  onRefresh: () => void
}

const BGM_PRESETS = [
  { name: '官方直连 (默认)', url: 'https://api.bgm.tv' },
  { name: 'Rin Cat 镜像', url: 'https://mirror.bgm.rin.cat' },
  { name: 'Chii 镜像', url: 'https://chii.ai' },
]

const DANDAN_PRESETS = [
  { name: '官方节点', url: 'https://api.dandanplay.net' },
  { name: '免签高可用节点', url: 'https://ddplay.retr0.xyz' },
]

export const SettingsView: React.FC<SettingsViewProps> = ({ config, onRefresh }) => {
  const [bangumiHost, setBangumiHost] = useState(config?.bangumiHost || 'https://api.bgm.tv')
  const [dandanHost, setDanDanHost] = useState(config?.dandanHost || 'https://api.dandanplay.net')
  const [serverName, setServerName] = useState(config?.serverName || 'Akari Media')
  const [customProxy, setCustomProxy] = useState(config?.customProxy || '')

  useEffect(() => {
    if (config) {
      setBangumiHost(config.bangumiHost || 'https://api.bgm.tv')
      setDanDanHost(config.dandanHost || 'https://api.dandanplay.net')
      setServerName(config.serverName || 'Akari Media')
      setCustomProxy(config.customProxy || '')
    }
  }, [config])

  const [saveLoading, setSaveLoading] = useState(false)
  const [saveSuccess, setSaveSuccess] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)

  const [testingBgm, setTestingBgm] = useState(false)
  const [bgmTestResult, setBgmTestResult] = useState<MirrorTestResult | null>(null)

  // Password update states
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwdLoading, setPwdLoading] = useState(false)
  const [pwdSuccess, setPwdSuccess] = useState<string | null>(null)
  const [pwdError, setPwdError] = useState<string | null>(null)

  const [cleaning, setCleaning] = useState(false)
  const [cleanMsg, setCleanMsg] = useState<string | null>(null)

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
        error: err.message || '网络连接测试失败',
      })
    } finally {
      setTestingBgm(false)
    }
  }

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
      setSaveSuccess('服务端配置已成功保存并实时生效！')
      onRefresh()
      setTimeout(() => setSaveSuccess(null), 3000)
    } catch (err: any) {
      setSaveError(err.message || '保存配置失败')
    } finally {
      setSaveLoading(false)
    }
  }

  const handleUpdatePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setPwdSuccess(null)
    setPwdError(null)

    if (!oldPassword) {
      setPwdError('请输入当前管理员旧密码')
      return
    }

    if (newPassword.length < 6) {
      setPwdError('新密码长度不能少于 6 位')
      return
    }

    if (newPassword !== confirmPassword) {
      setPwdError('两次输入的新密码不一致')
      return
    }

    setPwdLoading(true)
    try {
      await api.adminChangePassword(oldPassword, newPassword)
      setPwdSuccess('管理员密码已成功更新！')
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setTimeout(() => setPwdSuccess(null), 3000)
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
      setCleanMsg(res.message || '缓存清理成功')
      onRefresh()
    } catch (err: any) {
      setCleanMsg('清理失败: ' + err.message)
    } finally {
      setCleaning(false)
      setTimeout(() => setCleanMsg(null), 3000)
    }
  }

  return (
    <motion.div
      variants={containerVariants}
      initial="hidden"
      animate="show"
      className="flex flex-col gap-6 pb-12"
    >
      {/* 1. Basic Server Profile */}
      <motion.div variants={itemVariants}>
        <MdCard variant="filled" className="p-6 sm:p-8">
        <div className="flex items-center gap-3 mb-6 pb-4 border-b border-[var(--md-outline-variant)]/60">
          <div className="p-2.5 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)]">
            <Server className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-[var(--md-on-surface)]">服务端核心参数</h3>
            <p className="text-xs text-[var(--md-on-surface-variant)]">配置服务实例展示名称与全局代理节点</p>
          </div>
        </div>

        <form onSubmit={handleSaveConfig} className="flex flex-col gap-5">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            <MdTextField
              label="服务器展示名称 (Server Name)"
              placeholder="例如: Akari Media"
              value={serverName}
              onChange={(e) => setServerName(e.target.value)}
              helperText="展示在 Emby 客户端连接列表中"
              required
            />

            <MdTextField
              label="自定义全局 HTTP 代理 (可选)"
              placeholder="http://127.0.0.1:7890"
              value={customProxy}
              onChange={(e) => setCustomProxy(e.target.value)}
              helperText="留空则自动检测并使用系统代理"
            />
          </div>

          {/* Bangumi Mirrors */}
          <div className="flex flex-col gap-3 pt-3 border-t border-[var(--md-outline-variant)]/40">
            <div className="flex items-center justify-between">
              <label className="text-xs font-semibold text-[var(--md-on-surface)]">
                Bangumi 元数据 API 镜像源
              </label>
              <MdButton
                type="button"
                variant="outlined"
                size="sm"
                loading={testingBgm}
                onClick={() => handleTestBangumi()}
                icon={<Activity className="w-3.5 h-3.5" />}
              >
                测速连通性
              </MdButton>
            </div>

            <MdTextField
              placeholder="https://api.bgm.tv"
              value={bangumiHost}
              onChange={(e) => setBangumiHost(e.target.value)}
            />

            {/* Presets */}
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs text-[var(--md-on-surface-variant)]">快捷预设:</span>
              {BGM_PRESETS.map((p) => (
                <MdChip
                  key={p.url}
                  label={p.name}
                  selected={bangumiHost === p.url}
                  onClick={() => {
                    setBangumiHost(p.url)
                    handleTestBangumi(p.url)
                  }}
                />
              ))}
            </div>

            {bgmTestResult && (
              <div
                className={`p-3 rounded-2xl text-xs flex items-center gap-2 border ${
                  bgmTestResult.success
                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                    : 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20'
                }`}
              >
                {bgmTestResult.success ? (
                  <>
                    <CheckCircle2 className="w-4 h-4 shrink-0" />
                    <span>节点测试成功 · 延迟: {bgmTestResult.latencyMs}ms ({bgmTestResult.message || 'OK'})</span>
                  </>
                ) : (
                  <>
                    <AlertCircle className="w-4 h-4 shrink-0" />
                    <span>节点测试失败: {bgmTestResult.error}</span>
                  </>
                )}
              </div>
            )}
          </div>

          {/* DanDanPlay Mirrors */}
          <div className="flex flex-col gap-3 pt-3 border-t border-[var(--md-outline-variant)]/40">
            <label className="text-xs font-semibold text-[var(--md-on-surface)]">
              弹弹play 弹幕 API 端点
            </label>

            <MdTextField
              placeholder="https://api.dandanplay.net"
              value={dandanHost}
              onChange={(e) => setDanDanHost(e.target.value)}
            />

            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs text-[var(--md-on-surface-variant)]">快捷预设:</span>
              {DANDAN_PRESETS.map((p) => (
                <MdChip
                  key={p.url}
                  label={p.name}
                  selected={dandanHost === p.url}
                  onClick={() => setDanDanHost(p.url)}
                />
              ))}
            </div>
          </div>

          {/* Feedback messages */}
          {saveSuccess && (
            <div className="p-3.5 rounded-2xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs font-semibold border border-emerald-500/20">
              {saveSuccess}
            </div>
          )}
          {saveError && (
            <div className="p-3.5 rounded-2xl bg-rose-500/10 text-rose-600 dark:text-rose-400 text-xs font-semibold border border-rose-500/20">
              {saveError}
            </div>
          )}

          <div className="flex justify-end pt-2">
            <MdButton
              type="submit"
              variant="filled"
              size="md"
              loading={saveLoading}
              icon={<Save className="w-4 h-4" />}
            >
              保存配置
            </MdButton>
          </div>
        </form>
      </MdCard>
      </motion.div>

      {/* 2. Admin Password Change */}
      <motion.div variants={itemVariants}>
        <MdCard variant="filled" className="p-6 sm:p-8">
          <div className="flex items-center gap-3 mb-6 pb-4 border-b border-[var(--md-outline-variant)]/60">
            <div className="p-2.5 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)]">
              <Key className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-[var(--md-on-surface)]">修改管理员密码</h3>
              <p className="text-xs text-[var(--md-on-surface-variant)]">更新 Web 控制台登录凭据（bcrypt 安全哈希加密）</p>
            </div>
          </div>

          <form onSubmit={handleUpdatePassword} className="flex flex-col gap-4 max-w-lg">
            <MdTextField
              label="当前旧密码"
              type="password"
              placeholder="请输入当前密码 (默认: admin123)"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              required
            />

            <MdTextField
              label="新密码 (不少于 6 位)"
              type="password"
              placeholder="请输入新密码"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
            />

            <MdTextField
              label="确认新密码"
              type="password"
              placeholder="请再次输入新密码"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
            />

            {pwdSuccess && (
              <div className="p-3.5 rounded-2xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs font-semibold border border-emerald-500/20">
                {pwdSuccess}
              </div>
            )}
            {pwdError && (
              <div className="p-3.5 rounded-2xl bg-rose-500/10 text-rose-600 dark:text-rose-400 text-xs font-semibold border border-rose-500/20">
                {pwdError}
              </div>
            )}

            <div className="flex justify-start pt-2">
              <MdButton
                type="submit"
                variant="tonal"
                size="md"
                loading={pwdLoading}
                icon={<Lock className="w-4 h-4" />}
              >
                更新管理员密码
              </MdButton>
            </div>
          </form>
        </MdCard>
      </motion.div>

      {/* 3. Cache & Maintenance */}
      <motion.div variants={itemVariants}>
        <MdCard variant="filled" className="p-6 sm:p-8">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2.5 rounded-2xl bg-amber-500/10 text-amber-600 dark:text-amber-400">
              <Trash2 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-lg font-bold text-[var(--md-on-surface)]">系统缓存与维护</h3>
              <p className="text-xs text-[var(--md-on-surface-variant)]">一键清空直链播放嗅探缓存与未持久化的临时元数据</p>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mt-4 p-4 rounded-2xl bg-[var(--md-surface-container)]/70 border border-[var(--md-outline-variant)]/50">
            <div>
              <span className="text-sm font-semibold text-[var(--md-on-surface)]">重置并清理流嗅探缓存</span>
              <p className="text-xs text-[var(--md-on-surface-variant)]">用于强制重新抓取最新视频流与排查失效源</p>
            </div>

            <MdButton
              variant="tonal"
              size="md"
              loading={cleaning}
              onClick={handleCleanCache}
              icon={<Trash2 className="w-4 h-4" />}
            >
              立即清理
            </MdButton>
          </div>

          {cleanMsg && (
            <div className="mt-3 p-3 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] text-xs font-semibold">
              {cleanMsg}
            </div>
          )}
        </MdCard>
      </motion.div>
    </motion.div>
  )
}
