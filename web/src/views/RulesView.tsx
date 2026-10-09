import React, { useState, useEffect } from 'react'
import {
  Radio,
  Plus,
  Play,
  Trash2,
  ExternalLink,
  Search,
  Layers,
  RotateCcw,
  Tag,
  Sparkles,
  X,
} from 'lucide-react'
import { api } from '../api'
import type { RulePlugin, GlobalSynonym, SubjectAlias } from '../types'
import { RuleTestModal } from '../components/RuleTestModal'
import { AddRuleModal } from '../components/AddRuleModal'

interface RulesViewProps {
  rules: RulePlugin[]
  onRefresh: () => void
}

export const RulesView: React.FC<RulesViewProps> = ({ rules, onRefresh }) => {
  const [activeTab, setActiveTab] = useState<'plugins' | 'aliases'>('plugins')
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedRuleForTest, setSelectedRuleForTest] = useState<RulePlugin | null>(null)
  const [isAddOpen, setIsAddOpen] = useState(false)
  const [togglingRule, setTogglingRule] = useState<string | null>(null)

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
    setTogglingRule(rule.name)
    try {
      await api.toggleRule(rule.name, !rule.enabled)
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
      await api.deleteRule(rule.name)
      onRefresh()
    } catch (err: any) {
      alert(err.message || '删除失败')
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
    <div className="space-y-6 animate-fade-in">
      {/* Header & Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-white tracking-tight flex items-center space-x-2">
            <Radio className="w-5 h-5 text-pink-400" />
            <span>规则与别名匹配管理</span>
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            配置 Kazumi 视频解析源规则，以及全局同义词与番剧专属别名库，实现毫秒级精准聚合匹配
          </p>
        </div>

        {/* Tab Switcher */}
        <div className="flex items-center p-1 bg-slate-900/90 border border-slate-800 rounded-xl space-x-1 self-start sm:self-auto">
          <button
            onClick={() => setActiveTab('plugins')}
            className={`px-3 py-1.5 text-xs font-semibold rounded-lg flex items-center space-x-1.5 transition-all cursor-pointer ${
              activeTab === 'plugins'
                ? 'bg-gradient-to-r from-pink-500 to-purple-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Radio className="w-3.5 h-3.5" />
            <span>聚合源插件 ({rules.length})</span>
          </button>
          <button
            onClick={() => setActiveTab('aliases')}
            className={`px-3 py-1.5 text-xs font-semibold rounded-lg flex items-center space-x-1.5 transition-all cursor-pointer ${
              activeTab === 'aliases'
                ? 'bg-gradient-to-r from-pink-500 to-purple-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Sparkles className="w-3.5 h-3.5" />
            <span>别名与同义词 ({synonyms.length + subjectAliases.length})</span>
          </button>
        </div>
      </div>

      {activeTab === 'plugins' && (
        <>
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="relative flex-1">
              <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="搜索解析源名称或域名..."
                className="w-full pl-10 pr-4 py-2.5 bg-slate-900/80 border border-slate-800 rounded-xl text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-pink-500 transition-colors"
              />
            </div>
            <button
              onClick={() => setIsAddOpen(true)}
              className="px-4 py-2 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-xs font-semibold rounded-xl flex items-center space-x-1.5 shadow-md shadow-pink-500/20 transition-all cursor-pointer self-end sm:self-auto shrink-0"
            >
              <Plus className="w-4 h-4" />
              <span>添加 / 导入规则</span>
            </button>
          </div>

          {/* Rules Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filteredRules.length === 0 ? (
              <div className="col-span-full text-center py-16 bg-slate-900/40 border border-slate-800/80 rounded-2xl text-slate-500">
                <Radio className="w-10 h-10 mx-auto mb-2 opacity-30" />
                <p className="text-sm">未找到匹配的解析规则</p>
              </div>
            ) : (
              filteredRules.map((rule) => {
                const isToggling = togglingRule === rule.name
                return (
                  <div
                    key={rule.id || rule.name}
                    className={`p-5 rounded-2xl border transition-all flex flex-col justify-between ${
                      rule.enabled
                        ? 'bg-slate-900/70 border-slate-800 hover:border-pink-500/40 shadow-lg'
                        : 'bg-slate-950/40 border-slate-900 opacity-60'
                    }`}
                  >
                    <div>
                      {/* Top Row: Name & Switch */}
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <div className="flex items-center space-x-2">
                            <span className="font-bold text-base text-white">{rule.name}</span>
                            <span className="px-2 py-0.5 text-[10px] font-mono bg-slate-800 text-slate-300 rounded-full border border-slate-700">
                              v{rule.version || '1.0'}
                            </span>
                          </div>
                          <a
                            href={rule.baseURL}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs text-slate-400 hover:text-pink-400 flex items-center space-x-1 mt-1 truncate"
                          >
                            <span className="truncate">{rule.baseURL}</span>
                            <ExternalLink className="w-3 h-3 shrink-0" />
                          </a>
                        </div>

                        {/* Enable Toggle Switch */}
                        <button
                          onClick={() => handleToggle(rule)}
                          disabled={isToggling}
                          className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                            rule.enabled ? 'bg-pink-500' : 'bg-slate-700'
                          }`}
                        >
                          <span
                            className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                              rule.enabled ? 'translate-x-5' : 'translate-x-0'
                            }`}
                          />
                        </button>
                      </div>

                      {/* Badges */}
                      <div className="flex flex-wrap gap-1.5 mt-3">
                        <span className="px-2 py-0.5 text-[10px] bg-slate-800 text-slate-300 rounded-lg">
                          模式: {rule.searchMode || 'xpath'}
                        </span>
                        {rule.multiSources && (
                          <span className="px-2 py-0.5 text-[10px] bg-purple-500/10 text-purple-300 border border-purple-500/20 rounded-lg">
                            多线路
                          </span>
                        )}
                        {rule.adBlocker && (
                          <span className="px-2 py-0.5 text-[10px] bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 rounded-lg">
                            去广告
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Bottom Actions */}
                    <div className="mt-5 pt-3 border-t border-slate-800/80 flex items-center justify-between">
                      <button
                        onClick={() => setSelectedRuleForTest(rule)}
                        className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-pink-300 hover:text-pink-200 text-xs font-medium rounded-xl flex items-center space-x-1 transition-colors cursor-pointer"
                      >
                        <Play className="w-3 h-3 fill-current" />
                        <span>在线测试</span>
                      </button>

                      <button
                        onClick={() => handleDelete(rule)}
                        className="p-1.5 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-colors cursor-pointer"
                        title="删除规则"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </>
      )}

      {activeTab === 'aliases' && (
        <div className="space-y-8">
          {/* Section 1: Global Synonyms */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-2xl p-5 space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
              <div>
                <h3 className="text-base font-bold text-white flex items-center space-x-2">
                  <Layers className="w-4 h-4 text-purple-400" />
                  <span>全局词法同义词替换 (Global Synonyms)</span>
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  对所有番剧名称进行双向同义衍生与标准化，如「超时空」↔「超」、「已经死了」↔「已死」
                </p>
              </div>

              <div className="flex items-center space-x-2">
                <button
                  onClick={handleResetSynonyms}
                  className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium rounded-xl flex items-center space-x-1 transition-colors cursor-pointer"
                  title="重置为默认同义词"
                >
                  <RotateCcw className="w-3 h-3" />
                  <span>恢复默认</span>
                </button>
                <button
                  onClick={() => setIsAddSynonymOpen(true)}
                  className="px-3 py-1.5 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-xs font-semibold rounded-xl flex items-center space-x-1 shadow transition-all cursor-pointer"
                >
                  <Plus className="w-3.5 h-3.5" />
                  <span>新增同义词</span>
                </button>
              </div>
            </div>

            {/* Synonym list */}
            {synonyms.length === 0 ? (
              <div className="text-center py-8 text-slate-500 text-xs">
                暂无全局同义词规则，可点击右上角「恢复默认」加载推荐词库。
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                {synonyms.map((syn) => (
                  <div
                    key={syn.id || syn.pattern}
                    className="flex items-center justify-between p-3 bg-slate-950/60 border border-slate-800/80 rounded-xl"
                  >
                    <div className="flex items-center space-x-2 text-sm">
                      <span className="font-medium text-pink-300">{syn.pattern}</span>
                      <span className="text-slate-500 font-mono">↔</span>
                      <span className="font-medium text-purple-300">{syn.replacement || '（删除）'}</span>
                    </div>
                    <button
                      onClick={() => handleDeleteSynonym(syn.pattern)}
                      className="p-1 text-slate-500 hover:text-red-400 rounded transition-colors cursor-pointer"
                      title="删除"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Section 2: Subject Custom Aliases */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-2xl p-5 space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
              <div>
                <h3 className="text-base font-bold text-white flex items-center space-x-2">
                  <Tag className="w-4 h-4 text-pink-400" />
                  <span>番剧专属自定义别名 (Subject Aliases)</span>
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  为特定 Bangumi 条目指定额外的聚合搜索词（系统已默认自动从 Bangumi Infobox 与 Tags 提取别名）
                </p>
              </div>

              <button
                onClick={() => setIsAddSubjectAliasOpen(true)}
                className="px-3 py-1.5 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-xs font-semibold rounded-xl flex items-center space-x-1 shadow transition-all cursor-pointer self-start sm:self-auto"
              >
                <Plus className="w-3.5 h-3.5" />
                <span>新增番剧别名</span>
              </button>
            </div>

            {/* Search */}
            <div className="relative">
              <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                value={aliasSearch}
                onChange={(e) => setAliasSearch(e.target.value)}
                placeholder="搜索番剧名称、Bangumi ID 或别名..."
                className="w-full pl-9 pr-3 py-2 bg-slate-950/60 border border-slate-800 rounded-xl text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-pink-500"
              />
            </div>

            {/* List */}
            {filteredSubjectAliases.length === 0 ? (
              <div className="text-center py-10 text-slate-500 text-xs">
                {subjectAliases.length === 0
                  ? '暂无自定义番剧别名。遇到聚合源特殊命名的冷门番剧时，可在此针对性添加。'
                  : '未找到符合条件的番剧别名记录。'}
              </div>
            ) : (
              <div className="divide-y divide-slate-800/60 border border-slate-800/80 rounded-xl overflow-hidden bg-slate-950/40">
                {filteredSubjectAliases.map((sub) => (
                  <div key={sub.subjectId} className="p-3.5 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                    <div className="space-y-1">
                      <div className="flex items-center space-x-2">
                        <span className="text-xs font-bold text-white">{sub.title || `Bangumi #${sub.subjectId}`}</span>
                        <span className="px-1.5 py-0.5 text-[10px] font-mono bg-slate-800 text-slate-400 rounded">
                          ID: {sub.subjectId}
                        </span>
                      </div>
                      <div className="flex flex-wrap gap-1.5 pt-0.5">
                        {sub.aliases.map((al, idx) => (
                          <span
                            key={idx}
                            className="px-2 py-0.5 bg-pink-500/10 text-pink-300 border border-pink-500/20 rounded-md text-[11px]"
                          >
                            {al}
                          </span>
                        ))}
                      </div>
                    </div>

                    <button
                      onClick={() => handleDeleteSubjectAlias(sub.subjectId)}
                      className="p-1.5 text-slate-500 hover:text-red-400 rounded transition-colors cursor-pointer self-end sm:self-auto"
                      title="删除此番剧的所有自定义别名"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Add Synonym Modal */}
      {isAddSynonymOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <h3 className="text-base font-bold text-white flex items-center space-x-2">
                <Layers className="w-4 h-4 text-purple-400" />
                <span>新增全局同义词</span>
              </h3>
              <button
                onClick={() => setIsAddSynonymOpen(false)}
                className="text-slate-400 hover:text-white"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleSaveSynonym} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  原文词 / 变体词 (Pattern)
                </label>
                <input
                  type="text"
                  required
                  placeholder="例如: 超时空 或 已经死了"
                  value={synPattern}
                  onChange={(e) => setSynPattern(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-sm text-white focus:outline-none focus:border-pink-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  替换词 / 标准词 (Replacement)
                </label>
                <input
                  type="text"
                  placeholder="例如: 超 或 已死 (留空则代表删除该词)"
                  value={synReplacement}
                  onChange={(e) => setSynReplacement(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-sm text-white focus:outline-none focus:border-pink-500"
                />
              </div>

              <div className="flex justify-end space-x-2 pt-2">
                <button
                  type="button"
                  onClick={() => setIsAddSynonymOpen(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl cursor-pointer"
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-xs font-semibold rounded-xl cursor-pointer"
                >
                  保存同义词
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add Subject Alias Modal */}
      {isAddSubjectAliasOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <h3 className="text-base font-bold text-white flex items-center space-x-2">
                <Tag className="w-4 h-4 text-pink-400" />
                <span>新增番剧专属别名</span>
              </h3>
              <button
                onClick={() => setIsAddSubjectAliasOpen(false)}
                className="text-slate-400 hover:text-white"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleSaveSubjectAlias} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  Bangumi Subject ID <span className="text-pink-400">*</span>
                </label>
                <input
                  type="number"
                  required
                  placeholder="例如: 604826"
                  value={subSubjectId}
                  onChange={(e) => setSubSubjectId(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-sm text-white focus:outline-none focus:border-pink-500 font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  番剧主标题 (选填)
                </label>
                <input
                  type="text"
                  placeholder="例如: 超辉夜姬！"
                  value={subTitle}
                  onChange={(e) => setSubTitle(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-sm text-white focus:outline-none focus:border-pink-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  自定义别名列表 <span className="text-slate-500">(逗号或换行分隔)</span>
                </label>
                <textarea
                  rows={3}
                  required
                  placeholder="超时空辉夜姬, 超辉夜姬剧场版, Chou Kaguya Hime"
                  value={subAliasesText}
                  onChange={(e) => setSubAliasesText(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-sm text-white focus:outline-none focus:border-pink-500"
                />
              </div>

              <div className="flex justify-end space-x-2 pt-2">
                <button
                  type="button"
                  onClick={() => setIsAddSubjectAliasOpen(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl cursor-pointer"
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-xs font-semibold rounded-xl cursor-pointer"
                >
                  保存别名
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modals */}
      <RuleTestModal
        rule={selectedRuleForTest}
        isOpen={!!selectedRuleForTest}
        onClose={() => setSelectedRuleForTest(null)}
      />

      <AddRuleModal
        isOpen={isAddOpen}
        onClose={() => setIsAddOpen(false)}
        onSuccess={onRefresh}
      />
    </div>
  )
}

