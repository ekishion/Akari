import React, { useState, useEffect } from 'react'
import { motion, AnimatePresence, type Variants } from 'framer-motion'
import {
  Radio,
  Plus,
  Play,
  Trash2,
  Edit3,
  RefreshCw,
  ExternalLink,
  Search,
  Layers,
  RotateCcw,
  Sparkles,
  Tv,
  QrCode,
} from 'lucide-react'
import { api } from '../api'
import type { RulePlugin, GlobalSynonym, SubjectAlias, BilibiliStatus } from '../types'
import { MdCard } from '../components/md3/MdCard'
import { MdButton } from '../components/md3/MdButton'
import { MdTextField } from '../components/md3/MdTextField'
import { MdSwitch } from '../components/md3/MdSwitch'
import { MdDialog } from '../components/md3/MdDialog'
import { RuleTestModal } from '../components/RuleTestModal'
import { AddRuleModal } from '../components/AddRuleModal'
import { EditRuleModal } from '../components/EditRuleModal'
import { BilibiliQrModal } from '../components/BilibiliQrModal'

interface RulesViewProps {
  rules: RulePlugin[]
  onRefresh: () => void
}

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05,
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

export const RulesView: React.FC<RulesViewProps> = ({ rules, onRefresh }) => {
  const [activeTab, setActiveTab] = useState<'plugins' | 'aliases'>('plugins')
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedRuleForTest, setSelectedRuleForTest] = useState<RulePlugin | null>(null)
  const [selectedRuleForEdit, setSelectedRuleForEdit] = useState<RulePlugin | null>(null)
  const [isAddOpen, setIsAddOpen] = useState(false)
  const [togglingRule, setTogglingRule] = useState<string | null>(null)
  const [updatingAllRules, setUpdatingAllRules] = useState(false)

  // Aliases & Synonyms state
  const [synonyms, setSynonyms] = useState<GlobalSynonym[]>([])
  const [subjectAliases, setSubjectAliases] = useState<SubjectAlias[]>([])

  // Modals for Synonyms/Aliases
  const [isAddSynonymOpen, setIsAddSynonymOpen] = useState(false)
  const [synPattern, setSynPattern] = useState('')
  const [synReplacement, setSynReplacement] = useState('')

  const [isAddSubjectAliasOpen, setIsAddSubjectAliasOpen] = useState(false)
  const [subSubjectId, setSubSubjectId] = useState('')
  const [subTitle, setSubTitle] = useState('')
  const [subAliasesText, setSubAliasesText] = useState('')
  const [aliasSearch, setAliasSearch] = useState('')

  // Bilibili Primary Source state
  const [biliStatus, setBiliStatus] = useState<BilibiliStatus | null>(null)
  const [isBiliQrModalOpen, setIsBiliQrModalOpen] = useState(false)
  const [biliToggling, setBiliToggling] = useState(false)

  const fetchBiliStatus = async () => {
    try {
      const st = await api.getBilibiliStatus()
      setBiliStatus(st)
    } catch (err) {
      console.error('Failed to load Bilibili status:', err)
    }
  }

  useEffect(() => {
    fetchBiliStatus()
  }, [])

  const handleToggleBilibili = async (enabled: boolean) => {
    setBiliToggling(true)
    try {
      await api.updateBilibiliConfig({ enabled })
      await fetchBiliStatus()
    } catch (err: any) {
      alert(err.message || '切换哔哩哔哩源状态失败')
    } finally {
      setBiliToggling(false)
    }
  }

  const handleToggleBiliPrefer = async (prefer: boolean) => {
    try {
      await api.updateBilibiliConfig({ prefer_bilibili: prefer })
      await fetchBiliStatus()
    } catch (err: any) {
      alert(err.message || '修改优先规则失败')
    }
  }

  const handleUpdateBiliQuality = async (maxQuality: number) => {
    try {
      await api.updateBilibiliConfig({ max_quality: maxQuality })
      await fetchBiliStatus()
    } catch (err: any) {
      alert(err.message || '修改画质限制失败')
    }
  }

  const handleUpdateBiliStreamMode = async (mode: string) => {
    try {
      await api.updateBilibiliConfig({ stream_mode: mode })
      await fetchBiliStatus()
    } catch (err: any) {
      alert(err.message || '切换流模式失败')
    }
  }

  const fetchAliasesData = async () => {
    try {
      const [syns, subs] = await Promise.all([
        api.listGlobalSynonyms(),
        api.listSubjectAliases(),
      ])
      setSynonyms(syns || [])
      setSubjectAliases(subs || [])
    } catch (err: any) {
      console.error('Failed to load aliases data:', err)
    }
  }

  useEffect(() => {
    if (activeTab === 'aliases') {
      fetchAliasesData()
    }
  }, [activeTab])

  const handleToggle = async (rule: RulePlugin) => {
    setTogglingRule(rule.name || rule.id)
    try {
      await api.toggleRule(rule.name || rule.id, !rule.enabled)
      onRefresh()
    } catch (err: any) {
      alert(err.message || '切换规则状态失败')
    } finally {
      setTogglingRule(null)
    }
  }

  const handleDelete = async (rule: RulePlugin) => {
    if (!confirm(`确定要删除解析规则 "${rule.name}" 吗？`)) return
    try {
      await api.deleteRule(rule.name || rule.id)
      onRefresh()
    } catch (err: any) {
      alert(err.message || '删除失败')
    }
  }

  const handleUpdateAllRules = async () => {
    if (!confirm('确定要从官方规则源一键更新覆盖所有规则吗？现有规则的启用/禁用状态将被保留。')) return
    setUpdatingAllRules(true)
    try {
      const res = await api.updateAllRules()
      alert(`规则同步更新完成！\n共处理 ${res.importedCount} 条规则（新增 ${res.addedCount} 条，覆盖更新 ${res.updatedCount} 条）`)
      onRefresh()
    } catch (err: any) {
      alert(err.message || '更新规则失败')
    } finally {
      setUpdatingAllRules(false)
    }
  }

  // Synonym actions
  const handleSaveSynonym = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!synPattern.trim()) return
    try {
      await api.upsertGlobalSynonym(synPattern.trim(), synReplacement.trim(), true)
      setSynPattern('')
      setSynReplacement('')
      setIsAddSynonymOpen(false)
      fetchAliasesData()
    } catch (err: any) {
      alert(err.message || '保存同义词失败')
    }
  }

  const handleDeleteSynonym = async (pat: string) => {
    if (!confirm(`确定要删除同义词规则 "${pat}" 吗？`)) return
    try {
      await api.deleteGlobalSynonym(pat)
      fetchAliasesData()
    } catch (err: any) {
      alert(err.message || '删除同义词失败')
    }
  }

  const handleResetSynonyms = async () => {
    if (!confirm('确定要重置全局同义词为默认推荐库吗？现有同义词将被替换。')) return
    try {
      await api.resetGlobalSynonyms()
      fetchAliasesData()
    } catch (err: any) {
      alert(err.message || '重置失败')
    }
  }

  // Subject alias actions
  const handleSaveSubjectAlias = async (e: React.FormEvent) => {
    e.preventDefault()
    const id = parseInt(subSubjectId.trim(), 10)
    if (isNaN(id) || id <= 0) {
      alert('请输入有效的 Bangumi Subject ID (纯数字)')
      return
    }
    const aliases = subAliasesText
      .split(/[,，\n]/)
      .map((s) => s.trim())
      .filter(Boolean)

    try {
      await api.upsertSubjectAliases(id, subTitle.trim(), aliases)
      setSubSubjectId('')
      setSubTitle('')
      setSubAliasesText('')
      setIsAddSubjectAliasOpen(false)
      fetchAliasesData()
    } catch (err: any) {
      alert(err.message || '保存番剧别名失败')
    }
  }

  const handleDeleteSubjectAlias = async (subjectId: number) => {
    if (!confirm(`确定要删除条目 ${subjectId} 的所有自定义别名吗？`)) return
    try {
      await api.deleteSubjectAliases(subjectId)
      fetchAliasesData()
    } catch (err: any) {
      alert(err.message || '删除番剧别名失败')
    }
  }

  const filteredRules = rules.filter(
    (r) =>
      r.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      r.baseURL.toLowerCase().includes(searchTerm.toLowerCase())
  )

  const filteredSubjectAliases = subjectAliases.filter(
    (s) =>
      s.title.toLowerCase().includes(aliasSearch.toLowerCase()) ||
      s.subjectId.toString().includes(aliasSearch) ||
      s.aliases.some((a) => a.toLowerCase().includes(aliasSearch.toLowerCase()))
  )

  return (
    <div className="flex flex-col gap-6 animate-fade-in pb-12">
      {/* Tab Switcher & Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h3 className="text-lg font-bold text-[var(--md-on-surface)] flex items-center gap-2">
            <Radio className="w-5 h-5 text-[var(--md-primary)]" />
            <span>规则插件与同义词库</span>
          </h3>
          <p className="text-xs text-[var(--md-on-surface-variant)] mt-0.5">
            配置 Kazumi 视频解析规则与全局/条目别名库，支持毫秒级精准聚合匹配
          </p>
        </div>

        {/* Tab Pills with Framer Motion Sliding Pill */}
        <div className="flex items-center p-1 bg-[var(--md-surface-container)] rounded-full border border-[var(--md-outline-variant)]/60 gap-1 self-start sm:self-auto">
          <button
            onClick={() => setActiveTab('plugins')}
            className={`relative px-4 py-2 text-xs font-semibold rounded-full transition-colors cursor-pointer flex items-center gap-1.5 select-none ${
              activeTab === 'plugins'
                ? 'text-[var(--md-on-primary)]'
                : 'text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-surface)]'
            }`}
          >
            {activeTab === 'plugins' && (
              <motion.div
                layoutId="rulesTabActivePill"
                transition={{ type: 'spring', stiffness: 450, damping: 35 }}
                className="absolute inset-0 bg-[var(--md-primary)] rounded-full shadow-sm"
              />
            )}
            <Radio className="w-3.5 h-3.5 relative z-10" />
            <span className="relative z-10">聚合源插件 ({rules.length})</span>
          </button>
          <button
            onClick={() => setActiveTab('aliases')}
            className={`relative px-4 py-2 text-xs font-semibold rounded-full transition-colors cursor-pointer flex items-center gap-1.5 select-none ${
              activeTab === 'aliases'
                ? 'text-[var(--md-on-primary)]'
                : 'text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-surface)]'
            }`}
          >
            {activeTab === 'aliases' && (
              <motion.div
                layoutId="rulesTabActivePill"
                transition={{ type: 'spring', stiffness: 450, damping: 35 }}
                className="absolute inset-0 bg-[var(--md-primary)] rounded-full shadow-sm"
              />
            )}
            <Sparkles className="w-3.5 h-3.5 relative z-10" />
            <span className="relative z-10">别名与同义词 ({synonyms.length + subjectAliases.length})</span>
          </button>
        </div>
      </div>

      <AnimatePresence mode="wait">
        {activeTab === 'plugins' ? (
          <motion.div
            key="plugins-tab"
            variants={containerVariants}
            initial="hidden"
            animate="show"
            exit={{ opacity: 0 }}
            className="flex flex-col gap-6"
          >
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div className="relative flex-1">
                <MdTextField
                  placeholder="搜索解析规则名称或域名..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  leadingIcon={<Search className="w-4 h-4" />}
                />
              </div>
              <div className="flex items-center gap-2 self-end sm:self-auto shrink-0">
                <MdButton
                  variant="tonal"
                  size="md"
                  onClick={handleUpdateAllRules}
                  disabled={updatingAllRules}
                  loading={updatingAllRules}
                  icon={<RefreshCw className={`w-4 h-4 ${updatingAllRules ? 'animate-spin' : ''}`} />}
                >
                  一键更新规则
                </MdButton>
                <MdButton
                  variant="filled"
                  size="md"
                  onClick={() => setIsAddOpen(true)}
                  icon={<Plus className="w-4 h-4" />}
                >
                  添加 / 导入规则
                </MdButton>
              </div>
            </div>

            {/* Bilibili Primary Source Hero Card */}
            <motion.div variants={itemVariants}>
              <MdCard
                variant="filled"
                className="p-5 sm:p-6 border border-pink-500/30 bg-gradient-to-br from-pink-500/5 via-[var(--md-surface-container)] to-[var(--md-surface-container)] relative overflow-hidden shadow-xs"
              >
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-[var(--md-outline-variant)]/60">
                  <div className="flex items-center gap-3.5">
                    <div className="p-3 rounded-2xl bg-pink-500/10 text-pink-600 dark:text-pink-400 shrink-0">
                      <Tv className="w-6 h-6" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-bold text-base text-[var(--md-on-surface)]">哔哩哔哩 (Bilibili) 官方源</span>
                        <span className="px-2.5 py-0.5 text-[11px] font-bold bg-pink-500/15 text-pink-600 dark:text-pink-400 rounded-full border border-pink-500/30 flex items-center gap-1">
                          🌟 第一首要源
                        </span>
                        {biliStatus?.is_login && (
                          <span
                            className={`text-[10px] font-bold px-2 py-0.5 rounded-full ${
                              biliStatus.is_vip
                                ? 'bg-gradient-to-r from-pink-500 to-rose-500 text-white'
                                : 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20'
                            }`}
                          >
                            {biliStatus.is_vip ? '大会员 VIP' : '普通登录'}
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-[var(--md-on-surface-variant)] mt-1">
                        优先级最高：播放时优先秒级直连 B 站官方超清流；B 站未收录或无该集时，自动降级请求下方动漫站规则源。
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 self-end sm:self-auto shrink-0">
                    <MdButton
                      type="button"
                      variant="tonal"
                      size="sm"
                      onClick={() => setIsBiliQrModalOpen(true)}
                      icon={<QrCode className="w-4 h-4" />}
                    >
                      {biliStatus?.is_login ? '重新扫码' : '扫码授权登录'}
                    </MdButton>
                    <MdSwitch
                      checked={biliStatus?.enabled ?? true}
                      disabled={biliToggling}
                      onChange={() => handleToggleBilibili(!(biliStatus?.enabled ?? true))}
                    />
                  </div>
                </div>

                {/* Sub-controls: Priority, Quality & Stream Mode Selector */}
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 pt-4 text-xs">
                  <div className="flex items-center justify-between p-3 rounded-xl bg-[var(--md-surface-container-high)]/60 border border-[var(--md-outline-variant)]/40">
                    <div>
                      <span className="font-semibold text-[var(--md-on-surface)]">优先匹配返回</span>
                      <p className="text-[11px] text-[var(--md-on-surface-variant)]">命中后直接秒级直出</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={biliStatus?.prefer ?? true}
                      onChange={(e) => handleToggleBiliPrefer(e.target.checked)}
                      className="w-4 h-4 rounded accent-[var(--md-primary)] cursor-pointer"
                    />
                  </div>

                  <div className="flex items-center justify-between p-3 rounded-xl bg-[var(--md-surface-container-high)]/60 border border-[var(--md-outline-variant)]/40">
                    <div>
                      <span className="font-semibold text-[var(--md-on-surface)]">最高画质上限</span>
                      <p className="text-[11px] text-[var(--md-on-surface-variant)]">
                        最高可用: <strong className="text-[var(--md-primary)]">{biliStatus?.quality_desc || '480P 清晰'}</strong>
                      </p>
                    </div>
                    <select
                      value={biliStatus?.max_quality || 120}
                      onChange={(e) => handleUpdateBiliQuality(Number(e.target.value))}
                      className="px-3 py-1.5 rounded-lg bg-[var(--md-surface-container)] text-[var(--md-on-surface)] text-xs border border-[var(--md-outline-variant)] focus:border-[var(--md-primary)] focus:outline-hidden cursor-pointer"
                    >
                      <option value={120}>4K 超清 / 杜比 (最高)</option>
                      <option value={116}>1080P 60帧 (大会员)</option>
                      <option value={112}>1080P 高码率 (大会员)</option>
                      <option value={80}>1080P 高清 (需登录)</option>
                      <option value={64}>720P 高清</option>
                      <option value={32}>480P 清晰 (免登)</option>
                    </select>
                  </div>

                  <div className="flex items-center justify-between p-3 rounded-xl bg-[var(--md-surface-container-high)]/60 border border-[var(--md-outline-variant)]/40">
                    <div>
                      <span className="font-semibold text-[var(--md-on-surface)]">串流传输模式</span>
                      <p className="text-[11px] text-[var(--md-on-surface-variant)]">
                        {biliStatus?.stream_mode === 'dash' ? 'DASH多路混流 (需ffmpeg)' : 'MP4单流直链 (免ffmpeg)'}
                      </p>
                    </div>
                    <select
                      value={biliStatus?.stream_mode || 'direct'}
                      onChange={(e) => handleUpdateBiliStreamMode(e.target.value)}
                      className="px-3 py-1.5 rounded-lg bg-[var(--md-surface-container)] text-[var(--md-on-surface)] text-xs border border-[var(--md-outline-variant)] focus:border-[var(--md-primary)] focus:outline-hidden cursor-pointer"
                    >
                      <option value="direct">🚀 MP4 单流直链 (免 ffmpeg / 极速稳定)</option>
                      <option value="dash">🎬 DASH 混流模式 (需 ffmpeg / 支持4K)</option>
                    </select>
                  </div>
                </div>
              </MdCard>
            </motion.div>

            {/* Rules Cards Grid */}
            <motion.div variants={containerVariants} className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {filteredRules.length === 0 ? (
                <div className="col-span-full text-center py-16 bg-[var(--md-surface-container-low)] border border-[var(--md-outline-variant)] rounded-[32px] text-[var(--md-on-surface-variant)]">
                  <Radio className="w-10 h-10 mx-auto mb-2 opacity-30" />
                  <p className="text-sm font-medium">未找到匹配的解析规则</p>
                </div>
              ) : (
                filteredRules.map((rule) => (
                  <motion.div key={rule.id || rule.name} variants={itemVariants}>
                    <MdCard
                      variant="filled"
                      className={`p-6 flex flex-col justify-between h-full ${
                        rule.enabled ? 'border-[var(--md-outline-variant)]' : 'opacity-60 bg-[var(--md-surface-container-lowest)]'
                      }`}
                    >
                    <div>
                      {/* Top Row: Name & Switch */}
                      <div className="flex items-start justify-between gap-2">
                        <div className="overflow-hidden">
                          <div className="flex items-center gap-2">
                            <span className="font-bold text-base text-[var(--md-on-surface)] truncate">{rule.name}</span>
                            <span className="px-2 py-0.5 text-[10px] font-mono bg-[var(--md-surface-container-highest)] text-[var(--md-on-surface)] rounded-full">
                              v{rule.version || '1.0'}
                            </span>
                          </div>
                          <a
                            href={rule.baseURL}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs text-[var(--md-on-surface-variant)] hover:text-[var(--md-primary)] flex items-center gap-1 mt-1 truncate"
                          >
                            <span className="truncate">{rule.baseURL}</span>
                            <ExternalLink className="w-3 h-3 shrink-0" />
                          </a>
                        </div>

                        <MdSwitch
                          checked={rule.enabled}
                          disabled={togglingRule === (rule.name || rule.id)}
                          onChange={() => handleToggle(rule)}
                        />
                      </div>

                      {/* Badges */}
                      <div className="flex flex-wrap gap-1.5 mt-4">
                        <span className="px-2.5 py-0.5 text-[10px] font-medium bg-[var(--md-surface-container)] text-[var(--md-on-surface-variant)] rounded-lg">
                          模式: {rule.searchMode || 'xpath'}
                        </span>
                        {rule.multiSources && (
                          <span className="px-2.5 py-0.5 text-[10px] font-medium bg-purple-500/10 text-purple-600 dark:text-purple-300 rounded-lg">
                            多线路
                          </span>
                        )}
                        {rule.adBlocker && (
                          <span className="px-2.5 py-0.5 text-[10px] font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-300 rounded-lg">
                            去广告
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Bottom Actions */}
                    <div className="mt-5 pt-3 border-t border-[var(--md-outline-variant)]/60 flex items-center justify-between">
                      <MdButton
                        variant="tonal"
                        size="sm"
                        onClick={() => setSelectedRuleForTest(rule)}
                        icon={<Play className="w-3 h-3 fill-current" />}
                      >
                        在线沙盒测试
                      </MdButton>

                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => setSelectedRuleForEdit(rule)}
                          className="p-2 text-[var(--md-on-surface-variant)] hover:text-[var(--md-primary)] hover:bg-[var(--md-primary-container)]/30 rounded-full transition-colors cursor-pointer"
                          title="编辑修改规则"
                        >
                          <Edit3 className="w-4 h-4" />
                        </button>

                        <button
                          onClick={() => handleDelete(rule)}
                          className="p-2 text-[var(--md-on-surface-variant)] hover:text-[var(--md-danger)] hover:bg-[var(--md-danger-container)]/30 rounded-full transition-colors cursor-pointer"
                          title="删除规则"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  </MdCard>
                </motion.div>
              )))
            }</motion.div>
          </motion.div>
        ) : (
          <motion.div
            key="aliases-tab"
            variants={containerVariants}
            initial="hidden"
            animate="show"
            exit={{ opacity: 0 }}
            className="flex flex-col gap-8"
          >
            {/* Section 1: Global Synonyms */}
          <MdCard variant="filled" className="p-6 sm:p-8">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-[var(--md-outline-variant)]/60">
              <div>
                <h4 className="text-base font-bold text-[var(--md-on-surface)] flex items-center gap-2">
                  <Layers className="w-4 h-4 text-[var(--md-primary)]" />
                  <span>全局词法同义词替换 (Global Synonyms)</span>
                </h4>
                <p className="text-xs text-[var(--md-on-surface-variant)] mt-0.5">
                  对所有番剧名称进行双向同义衍生与标准化（如「超时空」↔「超」、「已经死了」↔「已死」）
                </p>
              </div>

              <div className="flex items-center gap-2">
                <MdButton
                  variant="outlined"
                  size="sm"
                  onClick={handleResetSynonyms}
                  icon={<RotateCcw className="w-3.5 h-3.5" />}
                >
                  重置推荐词库
                </MdButton>
                <MdButton
                  variant="filled"
                  size="sm"
                  onClick={() => setIsAddSynonymOpen(true)}
                  icon={<Plus className="w-3.5 h-3.5" />}
                >
                  添加同义词
                </MdButton>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3 mt-4">
              {synonyms.length === 0 ? (
                <div className="col-span-full py-6 text-center text-xs text-[var(--md-on-surface-variant)]">
                  暂无全局同义词记录
                </div>
              ) : (
                synonyms.map((s) => (
                  <div
                    key={s.pattern}
                    className="flex items-center justify-between px-3.5 py-2 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)] text-xs"
                  >
                    <div className="flex items-center gap-2 font-mono">
                      <span className="font-bold text-[var(--md-primary)]">{s.pattern}</span>
                      <span className="text-[var(--md-on-surface-variant)]">↔</span>
                      <span className="text-[var(--md-on-surface)]">{s.replacement}</span>
                    </div>
                    <button
                      onClick={() => handleDeleteSynonym(s.pattern)}
                      className="text-[var(--md-on-surface-variant)] hover:text-[var(--md-danger)] p-1 rounded-full cursor-pointer"
                      title="删除"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))
              )}
            </div>
          </MdCard>

          {/* Section 2: Per-Subject Custom Aliases */}
          <MdCard variant="filled" className="p-6 sm:p-8">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-[var(--md-outline-variant)]/60">
              <div>
                <h4 className="text-base font-bold text-[var(--md-on-surface)] flex items-center gap-2">
                  <Sparkles className="w-4 h-4 text-pink-500" />
                  <span>特定番剧专属别名库 (Subject Custom Aliases)</span>
                </h4>
                <p className="text-xs text-[var(--md-on-surface-variant)] mt-0.5">
                  为特定 Bangumi 条目 ID 绑定自定义搜索关键词与民间译名
                </p>
              </div>

              <div className="flex items-center gap-2">
                <div className="w-48 sm:w-64">
                  <MdTextField
                    placeholder="按名称或 ID 筛选..."
                    value={aliasSearch}
                    onChange={(e) => setAliasSearch(e.target.value)}
                  />
                </div>
                <MdButton
                  variant="filled"
                  size="sm"
                  onClick={() => setIsAddSubjectAliasOpen(true)}
                  icon={<Plus className="w-3.5 h-3.5" />}
                  className="shrink-0"
                >
                  添加条目别名
                </MdButton>
              </div>
            </div>

            <div className="flex flex-col gap-3 mt-4">
              {filteredSubjectAliases.length === 0 ? (
                <div className="py-8 text-center text-xs text-[var(--md-on-surface-variant)]">
                  暂无番剧条目专属别名映射
                </div>
              ) : (
                filteredSubjectAliases.map((sa) => (
                  <div
                    key={sa.subjectId}
                    className="p-4 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)] flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                  >
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-bold text-sm text-[var(--md-on-surface)]">{sa.title || '未命名条目'}</span>
                        <span className="px-2 py-0.5 rounded-full text-[10px] font-mono bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)]">
                          ID: {sa.subjectId}
                        </span>
                      </div>
                      <div className="flex flex-wrap items-center gap-1.5 mt-2">
                        {sa.aliases.map((al) => (
                          <span
                            key={al}
                            className="px-2.5 py-0.5 rounded-full text-xs bg-[var(--md-surface-container-high)] text-[var(--md-on-surface)] font-medium"
                          >
                            {al}
                          </span>
                        ))}
                      </div>
                    </div>

                    <MdButton
                      variant="text"
                      size="sm"
                      onClick={() => handleDeleteSubjectAlias(sa.subjectId)}
                      icon={<Trash2 className="w-4 h-4 text-[var(--md-danger)]" />}
                    >
                      删除
                    </MdButton>
                  </div>
                ))
              )}
            </div>
          </MdCard>
        </motion.div>
      )}
      </AnimatePresence>

      {/* Add Synonym Modal */}
      <MdDialog
        open={isAddSynonymOpen}
        onClose={() => setIsAddSynonymOpen(false)}
        title="添加全局同义词"
        subtitle="将常见翻译差异或缩写规范化为源站通用词。"
        icon={<Layers className="w-5 h-5 text-[var(--md-primary)]" />}
      >
        <form onSubmit={handleSaveSynonym} className="flex flex-col gap-4">
          <MdTextField
            label="原词 (Pattern)"
            placeholder="例如: 已经死了"
            value={synPattern}
            onChange={(e) => setSynPattern(e.target.value)}
            required
          />
          <MdTextField
            label="替换词 / 简称 (Replacement)"
            placeholder="例如: 已死"
            value={synReplacement}
            onChange={(e) => setSynReplacement(e.target.value)}
            required
          />
          <div className="flex justify-end gap-2 pt-3 border-t border-[var(--md-outline-variant)]/60">
            <MdButton type="button" variant="text" onClick={() => setIsAddSynonymOpen(false)}>
              取消
            </MdButton>
            <MdButton type="submit" variant="filled">
              保存
            </MdButton>
          </div>
        </form>
      </MdDialog>

      {/* Add Subject Alias Modal */}
      <MdDialog
        open={isAddSubjectAliasOpen}
        onClose={() => setIsAddSubjectAliasOpen(false)}
        title="添加番剧专属别名"
        subtitle="为特定 Bangumi 条目绑定多重中文、日文、罗马音别名以提升匹配率。"
        icon={<Sparkles className="w-5 h-5 text-pink-500" />}
      >
        <form onSubmit={handleSaveSubjectAlias} className="flex flex-col gap-4">
          <MdTextField
            label="Bangumi Subject ID"
            placeholder="例如: 159725"
            value={subSubjectId}
            onChange={(e) => setSubSubjectId(e.target.value)}
            helperText="纯数字 ID，可在 bgm.tv 网址中查看"
            required
          />
          <MdTextField
            label="条目名称 (用于展示参考)"
            placeholder="例如: 罗小黑战记"
            value={subTitle}
            onChange={(e) => setSubTitle(e.target.value)}
          />
          <div className="flex flex-col gap-1.5">
            <label className="text-xs font-medium text-[var(--md-on-surface-variant)] px-1">
              别名列表 (多个别名用逗号或换行分隔)
            </label>
            <textarea
              rows={3}
              value={subAliasesText}
              onChange={(e) => setSubAliasesText(e.target.value)}
              placeholder="罗小黑战记大电影, 罗小黑电影版"
              className="w-full bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] rounded-2xl border border-[var(--md-outline-variant)] px-4 py-2.5 text-sm text-[var(--md-on-surface)] outline-none focus:ring-2 focus:ring-[var(--md-primary)]"
              required
            />
          </div>
          <div className="flex justify-end gap-2 pt-3 border-t border-[var(--md-outline-variant)]/60">
            <MdButton type="button" variant="text" onClick={() => setIsAddSubjectAliasOpen(false)}>
              取消
            </MdButton>
            <MdButton type="submit" variant="filled">
              保存
            </MdButton>
          </div>
        </form>
      </MdDialog>

      {/* Add Rule / Sandbox Modals */}
      <AddRuleModal
        isOpen={isAddOpen}
        onClose={() => setIsAddOpen(false)}
        onSuccess={() => {
          setIsAddOpen(false)
          onRefresh()
        }}
      />

      <EditRuleModal
        rule={selectedRuleForEdit}
        isOpen={!!selectedRuleForEdit}
        onClose={() => setSelectedRuleForEdit(null)}
        onSuccess={onRefresh}
      />

      <RuleTestModal
        rule={selectedRuleForTest}
        isOpen={!!selectedRuleForTest}
        onClose={() => setSelectedRuleForTest(null)}
      />

      <BilibiliQrModal
        isOpen={isBiliQrModalOpen}
        onClose={() => setIsBiliQrModalOpen(false)}
        onSuccess={() => {
          fetchBiliStatus()
          onRefresh()
        }}
      />
    </div>
  )
}
