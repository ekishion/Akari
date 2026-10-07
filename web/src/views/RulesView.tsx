import React, { useState } from 'react'
import {
  Radio,
  Plus,
  Play,
  Trash2,
  ExternalLink,
  Search,
} from 'lucide-react'
import { api } from '../api'
import type { RulePlugin } from '../types'
import { RuleTestModal } from '../components/RuleTestModal'
import { AddRuleModal } from '../components/AddRuleModal'

interface RulesViewProps {
  rules: RulePlugin[]
  onRefresh: () => void
}

export const RulesView: React.FC<RulesViewProps> = ({ rules, onRefresh }) => {
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedRuleForTest, setSelectedRuleForTest] = useState<RulePlugin | null>(null)
  const [isAddOpen, setIsAddOpen] = useState(false)
  const [togglingRule, setTogglingRule] = useState<string | null>(null)

  const filteredRules = rules.filter(
    (r) =>
      r.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      r.baseURL.toLowerCase().includes(searchTerm.toLowerCase())
  )

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

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header & Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-white tracking-tight flex items-center space-x-2">
            <Radio className="w-5 h-5 text-pink-400" />
            <span>动漫解析源与规则管理</span>
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            管理 Kazumi 视频解析规则，支持 XPath 与 API 模式，可在客户端点播时按优先级自动匹配聚合
          </p>
        </div>

        <div className="flex items-center space-x-3">
          <button
            onClick={() => setIsAddOpen(true)}
            className="px-4 py-2 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-xs font-semibold rounded-xl flex items-center space-x-1.5 shadow-md shadow-pink-500/20 transition-all cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            <span>添加 / 导入规则</span>
          </button>
        </div>
      </div>

      {/* Filter Bar */}
      <div className="relative">
        <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
        <input
          type="text"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          placeholder="搜索解析源名称或域名..."
          className="w-full pl-10 pr-4 py-2.5 bg-slate-900/80 border border-slate-800 rounded-xl text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-pink-500 transition-colors"
        />
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
