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
  Tv,
  QrCode,
  LogOut,
  RefreshCw,
} from 'lucide-react'
import { api } from '../api'
import type { SystemConfig, SystemStatus, MirrorTestResult, BilibiliStatus } from '../types'
import { MdCard } from '../components/md3/MdCard'
import { MdButton } from '../components/md3/MdButton'
import { MdTextField } from '../components/md3/MdTextField'
import { MdChip } from '../components/md3/MdChip'
import { BilibiliQrModal } from '../components/BilibiliQrModal'

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
  { name: '官方 Next 源', url: 'https://next.bgm.tv' },
]

const BGM_IMAGE_PRESETS = [
  { name: '官方图片源 (默认)', url: 'https://lain.bgm.tv' },
]

const DANDAN_PRESETS = [
  { name: '官方节点', url: 'https://api.dandanplay.net' },
  { name: '免签高可用节点', url: 'https://ddplay.retr0.xyz' },
]

export const SettingsView: React.FC<SettingsViewProps> = ({ config, onRefresh }) => {
  const [bangumiHost, setBangumiHost] = useState(config?.bangumiHost || 'https://api.bgm.tv')
  const [bangumiImageHost, setBangumiImageHost] = useState(config?.bangumiImageHost || 'https://lain.bgm.tv')
  const [enableECH, setEnableECH] = useState<boolean>(config?.enableECH !== false)
  const [dandanHost, setDanDanHost] = useState(config?.dandanHost || 'https://api.dandanplay.net')
  const [serverName, setServerName] = useState(config?.serverName || 'Akari Media')
  const [customProxy, setCustomProxy] = useState(config?.customProxy || '')

  useEffect(() => {
    if (config) {
      setBangumiHost(config.bangumiHost || 'https://api.bgm.tv')
      setBangumiImageHost(config.bangumiImageHost || 'https://lain.bgm.tv')
      setEnableECH(config.enableECH !== false)
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

  const [testingBgmImg, setTestingBgmImg] = useState(false)
  const [bgmImgTestResult, setBgmImgTestResult] = useState<MirrorTestResult | null>(null)

  // Bilibili States
  const [biliStatus, setBiliStatus] = useState<BilibiliStatus | null>(null)
  const [biliLoading, setBiliLoading] = useState(false)
  const [biliSaveLoading, setBiliSaveLoading] = useState(false)
  const [biliSessdata, setBiliSessdata] = useState('')
  const [biliBuvid3, setBiliBuvid3] = useState('')
  const [biliEnabled, setBiliEnabled] = useState(true)
  const [biliPrefer, setBiliPrefer] = useState(true)
  const [biliMaxQuality, setBiliMaxQuality] = useState(120)
  const [biliSuccess, setBiliSuccess] = useState<string | null>(null)
  const [biliError, setBiliError] = useState<string | null>(null)
  const [isQrModalOpen, setIsQrModalOpen] = useState(false)

  const fetchBilibiliStatus = async () => {
    setBiliLoading(true)
    try {
      const st = await api.getBilibiliStatus()
      setBiliStatus(st)
      if (st) {
        setBiliEnabled(st.enabled)
        setBiliPrefer(st.prefer)
        if (st.max_quality) setBiliMaxQuality(st.max_quality)
      }
    } catch (err) {
      console.error('Failed to load bilibili status:', err)
    } finally {
      setBiliLoading(false)
    }
  }

  useEffect(() => {
    fetchBilibiliStatus()
  }, [])

  const handleSaveBilibiliConfig = async (e: React.FormEvent) => {
    e.preventDefault()
    setBiliSaveLoading(true)
    setBiliSuccess(null)
    setBiliError(null)

    try {
      const payload: any = {
        enabled: biliEnabled,
        prefer_bilibili: biliPrefer,
        max_quality: biliMaxQuality,
      }
      if (biliSessdata.trim()) {
        payload.sessdata = biliSessdata.trim()
      }
      if (biliBuvid3.trim()) {
        payload.buvid3 = biliBuvid3.trim()
      }

      const res = await api.updateBilibiliConfig(payload)
      if (res && res.status) {
        setBiliStatus(res.status)
        setBiliSessdata('')
        setBiliBuvid3('')
        setBiliSuccess('哔哩哔哩配置已成功保存并同步！')
        setTimeout(() => setBiliSuccess(null), 3000)
      }
    } catch (err: any) {
      setBiliError(err.message || '保存哔哩哔哩配置失败')
    } finally {
      setBiliSaveLoading(false)
    }
  }

  const handleLogoutBilibili = async () => {
    if (!confirm('确定要解绑当前哔哩哔哩账号并清除凭据吗？')) return
    try {
      await api.logoutBilibili()
      setBiliSuccess('已成功退出哔哩哔哩登录')
      fetchBilibiliStatus()
      setTimeout(() => setBiliSuccess(null), 3000)
    } catch (err: any) {
      setBiliError(err.message || '退出登录失败')
    }
  }

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

  const handleTestBangumiImage = async (testUrl?: string) => {
    const target = (testUrl || bangumiImageHost).trim()
    if (!target) return
    setTestingBgmImg(true)
    setBgmImgTestResult(null)
    try {
      const res = await api.testBangumiImageMirror(target)
      setBgmImgTestResult(res)
    } catch (err: any) {
      setBgmImgTestResult({
        success: false,
        error: err.message || '图片源连接测试失败',
      })
    } finally {
      setTestingBgmImg(false)
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
        bangumiImageHost: bangumiImageHost.trim(),
        enableECH: enableECH,
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

          {/* Bangumi API & Mirror */}
          <div className="flex flex-col gap-3 pt-3 border-t border-[var(--md-outline-variant)]/40">
            <div className="flex items-center justify-between">
              <div>
                <label className="text-xs font-semibold text-[var(--md-on-surface)]">
                  Bangumi 元数据 API 节点
                </label>
                <p className="text-[11px] text-[var(--md-on-surface-variant)]">
                  用于刮削番剧日历、剧集信息、演职员与评分（默认官方源，支持配置自定义镜像站）
                </p>
              </div>
              <MdButton
                type="button"
                variant="outlined"
                size="sm"
                loading={testingBgm}
                onClick={() => handleTestBangumi()}
                icon={<Activity className="w-3.5 h-3.5" />}
              >
                测速 API
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
                    <span>API 节点测试成功 · 延迟: {bgmTestResult.latencyMs}ms ({bgmTestResult.message || 'OK'})</span>
                  </>
                ) : (
                  <>
                    <AlertCircle className="w-4 h-4 shrink-0" />
                    <span>API 节点测试失败: {bgmTestResult.error}</span>
                  </>
                )}
              </div>
            )}
          </div>

          {/* Bangumi Image Mirror */}
          <div className="flex flex-col gap-3 pt-3 border-t border-[var(--md-outline-variant)]/40">
            <div className="flex items-center justify-between">
              <div>
                <label className="text-xs font-semibold text-[var(--md-on-surface)]">
                  Bangumi 封面与图片资源节点
                </label>
                <p className="text-[11px] text-[var(--md-on-surface-variant)]">
                  用于拉取封面图片资源（默认官方源 lain.bgm.tv，支持配置自定义镜像站）
                </p>
              </div>
              <MdButton
                type="button"
                variant="outlined"
                size="sm"
                loading={testingBgmImg}
                onClick={() => handleTestBangumiImage()}
                icon={<Activity className="w-3.5 h-3.5" />}
              >
                测速图片源
              </MdButton>
            </div>

            <MdTextField
              placeholder="https://lain.bgm.tv"
              value={bangumiImageHost}
              onChange={(e) => setBangumiImageHost(e.target.value)}
            />

            {/* Presets */}
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs text-[var(--md-on-surface-variant)]">快捷预设:</span>
              {BGM_IMAGE_PRESETS.map((p) => (
                <MdChip
                  key={p.url}
                  label={p.name}
                  selected={bangumiImageHost === p.url}
                  onClick={() => {
                    setBangumiImageHost(p.url)
                    handleTestBangumiImage(p.url)
                  }}
                />
              ))}
            </div>

            {bgmImgTestResult && (
              <div
                className={`p-3 rounded-2xl text-xs flex items-center gap-2 border ${
                  bgmImgTestResult.success
                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                    : 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20'
                }`}
              >
                {bgmImgTestResult.success ? (
                  <>
                    <CheckCircle2 className="w-4 h-4 shrink-0" />
                    <span>图片节点测试成功 · 延迟: {bgmImgTestResult.latencyMs}ms ({bgmImgTestResult.message || 'OK'})</span>
                  </>
                ) : (
                  <>
                    <AlertCircle className="w-4 h-4 shrink-0" />
                    <span>图片节点测试失败: {bgmImgTestResult.error}</span>
                  </>
                )}
              </div>
            )}
          </div>

          {/* ECH (Encrypted Client Hello) TLS 1.3 Security & Anti-blocking */}
          <div className="flex flex-col gap-2 pt-3 border-t border-[var(--md-outline-variant)]/40">
            <div className="flex items-center justify-between">
              <div>
                <label className="text-xs font-semibold text-[var(--md-on-surface)]">
                  启用 ECH (Encrypted Client Hello) 握手加密
                </label>
                <p className="text-[11px] text-[var(--md-on-surface-variant)]">
                  基于 TLS 1.3 与 DNS HTTPS (Type 65) 动态公钥，隐藏请求 SNI，有效突破 DNS 污染和 GFW/运营商 TLS 阻断
                </p>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={enableECH}
                  onChange={(e) => setEnableECH(e.target.checked)}
                  className="sr-only peer"
                />
                <div className="w-11 h-6 bg-slate-200 peer-focus:outline-hidden rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[var(--md-primary)]"></div>
              </label>
            </div>
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

      {/* 2. Bilibili Integration & Authentication */}
      <motion.div variants={itemVariants}>
        <MdCard variant="filled" className="p-6 sm:p-8">
          <div className="flex items-center justify-between gap-3 mb-6 pb-4 border-b border-[var(--md-outline-variant)]/60">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-2xl bg-pink-500/10 text-pink-600 dark:text-pink-400">
                <Tv className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-[var(--md-on-surface)]">哔哩哔哩 (Bilibili) 接入与画质</h3>
                <p className="text-xs text-[var(--md-on-surface-variant)]">
                  毫秒级官方 API 直连解析与实时 DASH 流式合流，支持 4K/1080P60 大会员高码率
                </p>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <MdButton
                type="button"
                variant="tonal"
                size="sm"
                onClick={() => setIsQrModalOpen(true)}
                icon={<QrCode className="w-4 h-4" />}
              >
                扫码授权登录
              </MdButton>
            </div>
          </div>

          {/* Account Status Card */}
          <div className="p-4 rounded-2xl bg-[var(--md-surface-container)]/70 border border-[var(--md-outline-variant)]/50 mb-6 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div className="flex items-center gap-3.5">
              {biliStatus?.is_login && biliStatus.face ? (
                <img
                  src={biliStatus.face}
                  alt={biliStatus.uname}
                  referrerPolicy="no-referrer"
                  crossOrigin="anonymous"
                  onError={(e) => {
                    const target = e.currentTarget
                    if (!target.dataset.retried && biliStatus?.face) {
                      target.dataset.retried = 'true'
                      target.src = `/items/images/proxy?url=${encodeURIComponent(biliStatus.face)}`
                    }
                  }}
                  className="w-12 h-12 rounded-full border-2 border-pink-500/30 object-cover shadow-xs"
                />
              ) : (
                <div className="w-12 h-12 rounded-full bg-slate-200 dark:bg-slate-700 flex items-center justify-center text-slate-500">
                  <Tv className="w-6 h-6" />
                </div>
              )}

              <div>
                <div className="flex items-center gap-2">
                  <span className="font-bold text-sm text-[var(--md-on-surface)]">
                    {biliStatus?.is_login ? biliStatus.uname : '未登录 (公开免登模式)'}
                  </span>
                  {biliStatus?.is_login && (
                    <span
                      className={`text-[10px] font-bold px-2 py-0.5 rounded-full ${
                        biliStatus.is_vip
                          ? 'bg-gradient-to-r from-pink-500 to-rose-500 text-white shadow-xs'
                          : 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20'
                      }`}
                    >
                      {biliStatus.is_vip ? '大会员 VIP' : '普通用户'}
                    </span>
                  )}
                </div>
                <p className="text-xs text-[var(--md-on-surface-variant)] mt-0.5">
                  当前支持画质: <span className="font-semibold text-[var(--md-primary)]">{biliStatus?.quality_desc || '480P 清晰'}</span>
                  {biliStatus?.is_login && ` · UID: ${biliStatus.mid}`}
                </p>
              </div>
            </div>

            <div className="flex items-center gap-2 self-end sm:self-auto">
              <MdButton
                type="button"
                variant="outlined"
                size="sm"
                loading={biliLoading}
                onClick={fetchBilibiliStatus}
                icon={<RefreshCw className="w-3.5 h-3.5" />}
              >
                刷新状态
              </MdButton>
              {biliStatus?.is_login && (
                <MdButton
                  type="button"
                  variant="text"
                  size="sm"
                  onClick={handleLogoutBilibili}
                  icon={<LogOut className="w-3.5 h-3.5 text-rose-500" />}
                >
                  退出账号
                </MdButton>
              )}
            </div>
          </div>

          {/* Configuration Form */}
          <form onSubmit={handleSaveBilibiliConfig} className="flex flex-col gap-5">
            {/* Toggles & Options */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 p-4 rounded-2xl bg-[var(--md-surface-container-high)]/40 border border-[var(--md-outline-variant)]/40">
              <label className="flex items-center justify-between cursor-pointer">
                <div>
                  <span className="text-sm font-semibold text-[var(--md-on-surface)]">启用哔哩哔哩官方源</span>
                  <p className="text-xs text-[var(--md-on-surface-variant)]">允许解析并播放 B 站正版番剧与高清流</p>
                </div>
                <input
                  type="checkbox"
                  checked={biliEnabled}
                  onChange={(e) => setBiliEnabled(e.target.checked)}
                  className="w-5 h-5 rounded-md accent-[var(--md-primary)] cursor-pointer"
                />
              </label>

              <label className="flex items-center justify-between cursor-pointer">
                <div>
                  <span className="text-sm font-semibold text-[var(--md-on-surface)]">优先推荐 B 站片源</span>
                  <p className="text-xs text-[var(--md-on-surface-variant)]">匹配命中时置于播放源首位提供高画质</p>
                </div>
                <input
                  type="checkbox"
                  checked={biliPrefer}
                  onChange={(e) => setBiliPrefer(e.target.checked)}
                  className="w-5 h-5 rounded-md accent-[var(--md-primary)] cursor-pointer"
                />
              </label>
            </div>

            {/* Quality & Manual Input */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div>
                <label className="block text-xs font-semibold text-[var(--md-on-surface)] mb-2">
                  最大解析画质限制
                </label>
                <select
                  value={biliMaxQuality}
                  onChange={(e) => setBiliMaxQuality(Number(e.target.value))}
                  className="w-full px-4 py-3 rounded-2xl bg-[var(--md-surface-container)] text-[var(--md-on-surface)] text-sm border border-[var(--md-outline-variant)] focus:border-[var(--md-primary)] focus:outline-hidden transition-colors"
                >
                  <option value={120}>4K 超清 / 杜比视界 (需大会员)</option>
                  <option value={116}>1080P 60帧 (需大会员)</option>
                  <option value={80}>1080P 高清 (需普通登录)</option>
                  <option value={64}>720P 高清</option>
                  <option value={32}>480P 清晰 (免登录)</option>
                </select>
              </div>

              <MdTextField
                label="手动设置 SESSDATA (可选)"
                placeholder="留空则保持当前或使用扫码登录"
                type="password"
                value={biliSessdata}
                onChange={(e) => setBiliSessdata(e.target.value)}
                helperText="填入后将覆盖当前 Cookie"
              />
            </div>

            {biliSuccess && (
              <div className="p-3.5 rounded-2xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs font-semibold border border-emerald-500/20">
                {biliSuccess}
              </div>
            )}
            {biliError && (
              <div className="p-3.5 rounded-2xl bg-rose-500/10 text-rose-600 dark:text-rose-400 text-xs font-semibold border border-rose-500/20">
                {biliError}
              </div>
            )}

            <div className="flex justify-end pt-2">
              <MdButton
                type="submit"
                variant="filled"
                size="md"
                loading={biliSaveLoading}
                icon={<Save className="w-4 h-4" />}
              >
                保存哔哩哔哩设置
              </MdButton>
            </div>
          </form>
        </MdCard>
      </motion.div>

      {/* 3. Admin Password Change */}
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

      {/* Bilibili QR Login Modal */}
      <BilibiliQrModal
        isOpen={isQrModalOpen}
        onClose={() => setIsQrModalOpen(false)}
        onSuccess={fetchBilibiliStatus}
      />
    </motion.div>
  )
}
