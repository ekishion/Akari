import React, { useEffect, useState } from 'react'
import { motion, type Variants } from 'framer-motion'
import {
  Activity,
  HardDrive,
  Layers,
  Clock,
  Zap,
  Server,
  RefreshCw,
  Sparkles,
  ShieldCheck,
  Film,
  Cpu,
  Database,
  Radio,
} from 'lucide-react'
import { api } from '../api'
import type { SystemStatus, TelemetryEvent } from '../types'
import { MdCard } from '../components/md3/MdCard'
import { MdButton } from '../components/md3/MdButton'

interface DashboardViewProps {
  onNavigateTab?: (tab: 'rules' | 'users' | 'security' | 'settings') => void
}

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: {
      staggerChildren: 0.07,
    },
  },
}

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 14 },
  show: {
    opacity: 1,
    y: 0,
    transition: { type: 'spring', stiffness: 350, damping: 26 },
  },
}

export const DashboardView: React.FC<DashboardViewProps> = ({ onNavigateTab }) => {
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [telemetry, setTelemetry] = useState<TelemetryEvent | null>(null)
  const [cacheCleaning, setCacheCleaning] = useState(false)
  const [cacheResult, setCacheResult] = useState<string | null>(null)

  const fetchInitialStatus = async () => {
    try {
      const data = await api.getStatus()
      setStatus(data)
    } catch (err) {
      console.error('Failed to load system status:', err)
    }
  }

  useEffect(() => {
    fetchInitialStatus()

    // Subscribe to SSE telemetry stream
    const unsubscribe = api.subscribeTelemetry(
      (data) => {
        setTelemetry(data)
      },
      (err) => {
        console.warn('Telemetry SSE connection error:', err)
      }
    )

    return () => unsubscribe()
  }, [])

  const handleCleanCache = async () => {
    setCacheCleaning(true)
    setCacheResult(null)
    try {
      const res = await api.cleanCache()
      setCacheResult(res.message || '缓存清理完成')
    } catch (err: any) {
      setCacheResult('清理失败: ' + err.message)
    } finally {
      setCacheCleaning(false)
      setTimeout(() => setCacheResult(null), 3000)
    }
  }

  const formatUptime = (totalSeconds: number) => {
    const days = Math.floor(totalSeconds / 86400)
    const hours = Math.floor((totalSeconds % 86400) / 3600)
    const minutes = Math.floor((totalSeconds % 3600) / 60)
    const seconds = totalSeconds % 60
    if (days > 0) return `${days}天 ${hours}小时 ${minutes}分`
    if (hours > 0) return `${hours}小时 ${minutes}分 ${seconds}秒`
    return `${minutes}分 ${seconds}秒`
  }

  const uptime = telemetry?.uptimeSeconds ?? status?.uptimeSeconds ?? 0
  const allocMB = telemetry?.memoryAllocMB ?? status?.memoryAllocMB ?? 0
  const sysMB = telemetry?.memorySysMB ?? status?.memorySysMB ?? 0
  const goroutines = telemetry?.goroutines ?? status?.numGoroutines ?? 0
  const activeSessions = telemetry?.activeSessions ?? 0
  const enabledRules = telemetry?.enabledRules ?? status?.activeRules ?? 0
  const totalRules = telemetry?.totalRules ?? status?.totalRules ?? 0

  return (
    <motion.div
      variants={containerVariants}
      initial="hidden"
      animate="show"
      className="flex flex-col gap-6 pb-12"
    >
      {/* Hero Telemetry Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 sm:gap-6">
        {/* Active Streams */}
        <motion.div variants={itemVariants}>
          <MdCard
            variant="elevated"
            interactive
            className="flex items-center gap-4 p-5 sm:p-6"
          >
            <div className="flex items-center justify-center w-14 h-14 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-primary)] shadow-xs">
              <Film className="w-7 h-7" />
            </div>
            <div>
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">
                活跃媒体流
              </span>
              <div className="flex items-baseline gap-2 mt-0.5">
                <span className="text-2xl font-bold text-[var(--md-on-surface)]">
                  {activeSessions}
                </span>
                <span className="text-xs text-emerald-600 dark:text-emerald-400 font-semibold flex items-center gap-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-ping inline-block" />
                  实时推流中
                </span>
              </div>
            </div>
          </MdCard>
        </motion.div>

        {/* Enabled Rules */}
        <motion.div variants={itemVariants}>
          <MdCard
            variant="elevated"
            interactive
            className="flex items-center gap-4 p-5 sm:p-6"
          >
            <div className="flex items-center justify-center w-14 h-14 rounded-2xl bg-[var(--md-tertiary-container)] text-[var(--md-tertiary)] shadow-xs">
              <Layers className="w-7 h-7" />
            </div>
            <div>
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">
                番剧解析规则
              </span>
              <div className="flex items-baseline gap-2 mt-0.5">
                <span className="text-2xl font-bold text-[var(--md-on-surface)]">
                  {enabledRules}
                </span>
                <span className="text-xs font-medium text-[var(--md-on-surface-variant)]">
                  / {totalRules} 已就绪
                </span>
              </div>
            </div>
          </MdCard>
        </motion.div>

        {/* Memory Allocation */}
        <motion.div variants={itemVariants}>
          <MdCard
            variant="elevated"
            interactive
            className="flex items-center gap-4 p-5 sm:p-6"
          >
            <div className="flex items-center justify-center w-14 h-14 rounded-2xl bg-[var(--md-secondary-container)] text-[var(--md-on-secondary-container)] shadow-xs">
              <HardDrive className="w-7 h-7" />
            </div>
            <div>
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">
                内存占用 (Alloc)
              </span>
              <div className="flex items-baseline gap-2 mt-0.5">
                <span className="text-2xl font-bold text-[var(--md-on-surface)]">
                  {allocMB.toFixed(1)}
                </span>
                <span className="text-xs font-medium text-[var(--md-on-surface-variant)]">
                  MB (Sys: {sysMB.toFixed(0)}MB)
                </span>
              </div>
            </div>
          </MdCard>
        </motion.div>

        {/* System Uptime */}
        <motion.div variants={itemVariants}>
          <MdCard
            variant="elevated"
            interactive
            className="flex items-center gap-4 p-5 sm:p-6"
          >
            <div className="flex items-center justify-center w-14 h-14 rounded-2xl bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 shadow-xs">
              <Clock className="w-7 h-7" />
            </div>
            <div>
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">
                连续运行时间
              </span>
              <div className="text-lg font-bold text-[var(--md-on-surface)] mt-0.5 truncate">
                {formatUptime(uptime)}
              </div>
            </div>
          </MdCard>
        </motion.div>
      </div>

      {/* Main Info Columns */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left 2 Cols: Server Profile & Realtime Telemetry */}
        <div className="lg:col-span-2 flex flex-col gap-6">
          <motion.div variants={itemVariants}>
            <MdCard variant="filled" className="p-6 sm:p-8">
              <div className="flex items-center justify-between mb-6 pb-4 border-b border-[var(--md-outline-variant)]/60">
                <div className="flex items-center gap-3">
                  <div className="p-2.5 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] shadow-xs">
                    <Server className="w-5 h-5" />
                  </div>
                  <div>
                    <h3 className="text-lg font-bold text-[var(--md-on-surface)]">
                      {status?.serverName || 'Akari Media Bridge'}
                    </h3>
                    <span className="text-xs font-mono text-[var(--md-on-surface-variant)]">
                      Server ID: {status?.serverId || 'N/A'}
                    </span>
                  </div>
                </div>

                <span className="px-3.5 py-1 rounded-full text-xs font-bold bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] shadow-xs">
                  v{status?.version || '2.0.0'}
                </span>
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-3 gap-3.5 text-sm">
                <div className="p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 flex flex-col gap-1">
                  <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">HTTP 监听端口</span>
                  <span className="font-bold text-[var(--md-on-surface)]">{status?.httpPort || 8096}</span>
                </div>
                <div className="p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 flex flex-col gap-1">
                  <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">UDP 发现广播</span>
                  <span className="font-bold text-[var(--md-on-surface)]">{status?.udpPort || 7359}</span>
                </div>
                <div className="p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 flex flex-col gap-1">
                  <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">Goroutines 协程</span>
                  <span className="font-bold text-[var(--md-on-surface)] flex items-center gap-1.5">
                    <Cpu className="w-3.5 h-3.5 text-[var(--md-primary)]" />
                    {goroutines}
                  </span>
                </div>
                <div className="p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 flex flex-col gap-1">
                  <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">运行架构 / OS</span>
                  <span className="font-bold text-[var(--md-on-surface)]">{status?.os} / {status?.arch}</span>
                </div>
                <div className="p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 flex flex-col gap-1">
                  <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">SQLite 数据库</span>
                  <span className="font-bold text-[var(--md-on-surface)] flex items-center gap-1.5">
                    <Database className="w-3.5 h-3.5 text-pink-500" />
                    {status?.dbStats ? (status.dbStats.dbSizeBytes / 1024).toFixed(1) + ' KB' : 'WAL Mode'}
                  </span>
                </div>
                <div className="p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/40 flex flex-col gap-1">
                  <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">Go 运行时</span>
                  <span className="font-bold text-[var(--md-on-surface)]">{status?.goVersion || 'Go 1.26'}</span>
                </div>
              </div>
            </MdCard>
          </motion.div>

          {/* Realtime Telemetry Pulse Card */}
          <motion.div variants={itemVariants}>
            <MdCard variant="filled" className="p-6 sm:p-8">
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2.5">
                  <Activity className="w-5 h-5 text-[var(--md-primary)]" />
                  <h4 className="text-base font-bold text-[var(--md-on-surface)]">
                    实时遥测长连接 (SSE Telemetry Stream)
                  </h4>
                </div>
                <span className="text-xs font-mono text-[var(--md-on-surface-variant)]">
                  {telemetry?.timestamp || 'Waiting for heartbeat...'}
                </span>
              </div>

              <p className="text-xs text-[var(--md-on-surface-variant)] leading-relaxed mb-4">
                当前控制台已建立与服务端的单向 SSE 长连接，实时保持核心指标、活跃会话与安全日志的零延迟同步。
              </p>

              <div className="flex items-center gap-4 p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)]/50">
                <div className="relative flex items-center justify-center">
                  <Radio className="w-4 h-4 text-emerald-500 relative z-10" />
                  <span className="absolute w-4 h-4 rounded-full bg-emerald-500/30 animate-ping" />
                </div>
                <div className="text-xs text-[var(--md-on-surface)] flex-1 font-semibold">
                  流媒体代理防盗链 HMAC 动态签名有效 · SQLite WAL 并发事务已就绪
                </div>
              </div>
            </MdCard>
          </motion.div>
        </div>

        {/* Right 1 Col: Quick Actions & Navigation Shortcuts */}
        <div className="flex flex-col gap-6">
          <motion.div variants={itemVariants}>
            <MdCard variant="filled" className="p-6 flex flex-col gap-4">
              <div className="flex items-center gap-2 text-sm font-bold text-[var(--md-on-surface)]">
                <Zap className="w-4 h-4 text-[var(--md-primary)]" />
                <span>快捷操作中心</span>
              </div>

              <MdButton
                variant="tonal"
                size="md"
                loading={cacheCleaning}
                onClick={handleCleanCache}
                icon={<RefreshCw className="w-4 h-4" />}
                className="w-full justify-start"
              >
                清理流与元数据缓存
              </MdButton>

              {cacheResult && (
                <div className="p-3 rounded-2xl text-xs bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] font-medium animate-fade-in">
                  {cacheResult}
                </div>
              )}

              {onNavigateTab && (
                <>
                  <MdButton
                    variant="outlined"
                    size="md"
                    onClick={() => onNavigateTab('rules')}
                    icon={<Layers className="w-4 h-4" />}
                    className="w-full justify-start"
                  >
                    管理规则与同义词
                  </MdButton>

                  <MdButton
                    variant="outlined"
                    size="md"
                    onClick={() => onNavigateTab('security')}
                    icon={<ShieldCheck className="w-4 h-4" />}
                    className="w-full justify-start"
                  >
                    查看安全与审计日志
                  </MdButton>
                </>
              )}
            </MdCard>
          </motion.div>

          {/* Emby Quick Connect Info */}
          <motion.div variants={itemVariants}>
            <MdCard variant="elevated" className="p-6">
              <div className="flex items-center gap-2 mb-3">
                <Sparkles className="w-4 h-4 text-[var(--md-primary)]" />
                <span className="text-sm font-bold text-[var(--md-on-surface)]">
                  Emby 客户端直连地址
                </span>
              </div>
              <p className="text-xs text-[var(--md-on-surface-variant)] mb-3 leading-relaxed">
                在任何 Emby / VidHub / SenPlayer 客户端中直接输入以下主机地址即可连接：
              </p>
              <div className="p-3.5 rounded-2xl bg-[var(--md-surface-container)] font-mono text-xs font-semibold text-[var(--md-primary)] break-all border border-[var(--md-outline-variant)]/60 shadow-inner">
                {status?.serverUrl || 'http://127.0.0.1:8096'}
              </div>
            </MdCard>
          </motion.div>
        </div>
      </div>
    </motion.div>
  )
}
