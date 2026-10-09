import React, { useEffect, useState } from 'react'
import { motion, AnimatePresence, type Variants } from 'framer-motion'
import {
  ShieldCheck,
  ShieldAlert,
  Unlock,
  Trash2,
  RefreshCw,
  Key,
  Globe,
  Radio,
} from 'lucide-react'
import { api } from '../api'
import type { AuditLog, IPBan } from '../types'
import { MdCard } from '../components/md3/MdCard'
import { MdButton } from '../components/md3/MdButton'
import { MdDialog } from '../components/md3/MdDialog'

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: {
      staggerChildren: 0.06,
    },
  },
}

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 12 },
  show: {
    opacity: 1,
    y: 0,
    transition: { type: 'spring', stiffness: 350, damping: 25 },
  },
}

export const SecurityView: React.FC = () => {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [totalLogs, setTotalLogs] = useState(0)
  const [bans, setBans] = useState<IPBan[]>([])
  const [loading, setLoading] = useState(true)
  const [clearing, setClearing] = useState(false)
  const [unbanningIP, setUnbanningIP] = useState<string | null>(null)
  const [confirmClearOpen, setConfirmClearOpen] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  const fetchData = async () => {
    setLoading(true)
    try {
      const [logsRes, bansRes] = await Promise.all([
        api.getAuditLogs(100, 0),
        api.getBannedIPs(),
      ])
      setLogs(logsRes.items || [])
      setTotalLogs(logsRes.total || 0)
      setBans(bansRes.items || [])
    } catch (err: any) {
      console.error('Failed to load security data:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const handleClearLogs = async () => {
    setClearing(true)
    try {
      await api.clearAuditLogs()
      setLogs([])
      setTotalLogs(0)
      setConfirmClearOpen(false)
      setNotice('审计日志已全部清空')
    } catch (err: any) {
      setNotice('清空失败: ' + err.message)
    } finally {
      setClearing(false)
      setTimeout(() => setNotice(null), 3000)
    }
  }

  const handleUnban = async (ip: string) => {
    setUnbanningIP(ip)
    try {
      await api.unbanIP(ip)
      setBans((prev) => prev.filter((b) => b.ip !== ip))
      setNotice(`IP ${ip} 已解除封禁`)
      fetchData()
    } catch (err: any) {
      setNotice('解封失败: ' + err.message)
    } finally {
      setUnbanningIP(null)
      setTimeout(() => setNotice(null), 3000)
    }
  }

  const getEventBadge = (eventType: string) => {
    switch (eventType) {
      case 'login_success':
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">登录成功</span>
      case 'login_failed':
      case 'login_blocked':
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20">登录失败</span>
      case 'ip_banned':
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-600 text-white shadow-sm">IP 自动封禁</span>
      case 'ip_unbanned':
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border border-indigo-500/20">手动解封</span>
      case 'password_changed':
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20">修改密码</span>
      case 'rule_updated':
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">规则更新</span>
      default:
        return <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-gray-500/10 text-gray-600 dark:text-gray-400 border border-gray-500/20">{eventType}</span>
    }
  }

  return (
    <motion.div
      variants={containerVariants}
      initial="hidden"
      animate="show"
      className="flex flex-col gap-6 pb-12"
    >
      <AnimatePresence>
        {notice && (
          <motion.div
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            className="p-4 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] text-sm font-semibold shadow-md"
          >
            {notice}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Security Health Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <motion.div variants={itemVariants}>
          <MdCard variant="elevated" interactive className="p-6 flex items-center gap-4">
            <div className="flex items-center justify-center w-14 h-14 rounded-2xl bg-emerald-500/15 text-emerald-600 dark:text-emerald-400">
              <ShieldCheck className="w-7 h-7" />
            </div>
            <div>
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">安全防护等级</span>
              <div className="flex items-baseline gap-2 mt-0.5">
                <span className="text-2xl font-bold text-emerald-500">MAX</span>
                <span className="text-xs text-[var(--md-on-surface-variant)]">4 重安全防线激活</span>
              </div>
            </div>
          </MdCard>
        </motion.div>

        <motion.div variants={itemVariants}>
          <MdCard variant="elevated" interactive className="p-6 flex items-center gap-4">
            <div className="flex items-center justify-center w-14 h-14 rounded-2xl bg-[var(--md-primary-container)] text-[var(--md-primary)]">
              <Key className="w-7 h-7" />
            </div>
            <div>
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">鉴权与防盗链</span>
              <div className="text-sm font-bold text-[var(--md-on-surface)] mt-0.5">
                JWT 7天免登 + HMAC 签名
              </div>
            </div>
          </MdCard>
        </motion.div>

        <motion.div variants={itemVariants}>
          <MdCard variant="elevated" interactive className="p-6 flex items-center gap-4">
            <div className="flex items-center justify-center w-14 h-14 rounded-2xl bg-rose-500/15 text-rose-600 dark:text-rose-400">
              <Globe className="w-7 h-7" />
            </div>
            <div>
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">当前受限 / 封禁 IP</span>
              <div className="flex items-baseline gap-2 mt-0.5">
                <span className="text-2xl font-bold text-[var(--md-on-surface)]">{bans.length}</span>
                <span className="text-xs text-rose-500 font-semibold">防爆破拦截中</span>
              </div>
            </div>
          </MdCard>
        </motion.div>
      </div>

      {/* Banned IPs Section */}
      {bans.length > 0 && (
        <motion.div variants={itemVariants}>
          <MdCard variant="filled" className="p-6 border border-rose-500/30">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2 text-rose-500 font-bold text-base">
                <ShieldAlert className="w-5 h-5" />
                <span>已被系统锁定的 IP 列表 (已触发布控规则)</span>
              </div>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-[var(--md-outline-variant)] text-xs text-[var(--md-on-surface-variant)]">
                    <th className="pb-3 px-3">IP 地址</th>
                    <th className="pb-3 px-3">锁定原因</th>
                    <th className="pb-3 px-3">连续失误</th>
                    <th className="pb-3 px-3">解封时间</th>
                    <th className="pb-3 px-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--md-outline-variant)]/40">
                  {bans.map((ban) => (
                    <tr key={ban.ip} className="hover:bg-[var(--md-surface-container)] transition-colors">
                      <td className="py-3 px-3 font-mono font-bold text-rose-500">{ban.ip}</td>
                      <td className="py-3 px-3 text-xs text-[var(--md-on-surface)]">{ban.reason}</td>
                      <td className="py-3 px-3 text-xs">{ban.failedAttempts} 次</td>
                      <td className="py-3 px-3 text-xs text-[var(--md-on-surface-variant)]">
                        {ban.bannedUntil ? new Date(ban.bannedUntil).toLocaleString() : '永久封禁'}
                      </td>
                      <td className="py-3 px-3 text-right">
                        <MdButton
                          variant="tonal"
                          size="sm"
                          loading={unbanningIP === ban.ip}
                          onClick={() => handleUnban(ban.ip)}
                          icon={<Unlock className="w-3.5 h-3.5" />}
                        >
                          立即解封
                        </MdButton>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </MdCard>
        </motion.div>
      )}

      {/* Audit Logs Section */}
      <motion.div variants={itemVariants}>
        <MdCard variant="filled" className="p-6 sm:p-8">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6 pb-4 border-b border-[var(--md-outline-variant)]/60">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-xl bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)]">
                <Radio className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-[var(--md-on-surface)]">
                  安全与敏感操作审计流水
                </h3>
                <span className="text-xs text-[var(--md-on-surface-variant)]">
                  共记录 {totalLogs} 条关键安全事件 (SQLite 持久化)
                </span>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <MdButton
                variant="outlined"
                size="sm"
                onClick={fetchData}
                icon={<RefreshCw className="w-4 h-4" />}
              >
                刷新
              </MdButton>
              <MdButton
                variant="tonal"
                size="sm"
                disabled={logs.length === 0}
                onClick={() => setConfirmClearOpen(true)}
                icon={<Trash2 className="w-4 h-4 text-[var(--md-danger)]" />}
              >
                清空日志
              </MdButton>
            </div>
          </div>

          {/* Logs Table */}
          {loading ? (
            <div className="py-12 text-center text-sm text-[var(--md-on-surface-variant)]">
              加载审计日志中...
            </div>
          ) : logs.length === 0 ? (
            <div className="py-12 text-center text-sm text-[var(--md-on-surface-variant)]">
              暂无安全审计日志记录
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-[var(--md-outline-variant)] text-xs text-[var(--md-on-surface-variant)]">
                    <th className="pb-3 px-3">时间</th>
                    <th className="pb-3 px-3">事件类型</th>
                    <th className="pb-3 px-3">来源 IP</th>
                    <th className="pb-3 px-3">详细描述</th>
                    <th className="pb-3 px-3">客户端 / UA</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--md-outline-variant)]/40">
                  {logs.map((item) => (
                    <tr key={item.id} className="hover:bg-[var(--md-surface-container)] transition-colors">
                      <td className="py-3 px-3 text-xs text-[var(--md-on-surface-variant)] whitespace-nowrap">
                        {new Date(item.createdAt).toLocaleString()}
                      </td>
                      <td className="py-3 px-3">{getEventBadge(item.eventType)}</td>
                      <td className="py-3 px-3 font-mono text-xs text-[var(--md-on-surface)]">
                        {item.ipAddress || '-'}
                      </td>
                      <td className="py-3 px-3 text-xs font-medium text-[var(--md-on-surface)]">
                        {item.details}
                      </td>
                      <td className="py-3 px-3 text-xs text-[var(--md-on-surface-variant)] max-w-xs truncate" title={item.userAgent}>
                        {item.userAgent || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </MdCard>
      </motion.div>

      {/* Confirm Clear Modal */}
      <MdDialog
        open={confirmClearOpen}
        onClose={() => setConfirmClearOpen(false)}
        title="确认清空审计日志？"
        icon={<Trash2 className="w-5 h-5 text-[var(--md-danger)]" />}
        actions={
          <>
            <MdButton variant="text" onClick={() => setConfirmClearOpen(false)}>
              取消
            </MdButton>
            <MdButton variant="danger" loading={clearing} onClick={handleClearLogs}>
              确认清空
            </MdButton>
          </>
        }
      >
        清空后所有历史登录、失败拦截与配置变更的审计记录将被永久删除。
      </MdDialog>
    </motion.div>
  )
}
