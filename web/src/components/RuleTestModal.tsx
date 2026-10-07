import React, { useState } from 'react'
import { X, Search, Play, CheckCircle2, AlertCircle, Loader2, ExternalLink } from 'lucide-react'
import { api } from '../api'
import type { RulePlugin, TestResult } from '../types'

interface RuleTestModalProps {
  rule: RulePlugin | null
  isOpen: boolean
  onClose: () => void
}

export const RuleTestModal: React.FC<RuleTestModalProps> = ({ rule, isOpen, onClose }) => {
  const [keyword, setKeyword] = useState('葬送的芙莉莲')
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<TestResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  if (!isOpen || !rule) return null

  const handleTest = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    if (!keyword.trim()) return

    setLoading(true)
    setError(null)
    setResult(null)

    try {
      const res = await api.testRule({
        ruleName: rule.name,
        keyword: keyword.trim(),
      })
      setResult(res)
    } catch (err: any) {
      setError(err.message || '测试失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 w-full max-w-2xl rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/50">
          <div className="flex items-center space-x-2">
            <div className="w-8 h-8 rounded-lg bg-pink-500/20 text-pink-400 flex items-center justify-center font-bold">
              ⚡
            </div>
            <div>
              <h3 className="text-base font-semibold text-white">测试解析源: {rule.name}</h3>
              <p className="text-xs text-slate-400">实时请求测试搜索匹配和分集剧集嗅探</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Search Input */}
        <form onSubmit={handleTest} className="p-6 border-b border-slate-800 bg-slate-900/50">
          <div className="flex gap-2">
            <div className="relative flex-1">
              <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder="输入动漫名称测试，例如: 间谍过家家、咒术回战..."
                className="w-full pl-10 pr-4 py-2.5 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-pink-500 transition-colors"
              />
            </div>
            <button
              type="submit"
              disabled={loading || !keyword.trim()}
              className="px-5 py-2.5 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white font-medium text-sm rounded-xl flex items-center space-x-2 disabled:opacity-50 transition-all shadow-md shadow-pink-500/20 cursor-pointer"
            >
              {loading ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>抓取中...</span>
                </>
              ) : (
                <>
                  <Play className="w-4 h-4 fill-current" />
                  <span>运行测试</span>
                </>
              )}
            </button>
          </div>
        </form>

        {/* Results Area */}
        <div className="p-6 overflow-y-auto space-y-4 flex-1">
          {error && (
            <div className="p-4 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-start space-x-3">
              <AlertCircle className="w-5 h-5 shrink-0 mt-0.5" />
              <div>
                <div className="font-semibold">抓取错误</div>
                <div className="text-xs text-red-300/80 mt-0.5 break-all">{error}</div>
              </div>
            </div>
          )}

          {result && !result.success && (
            <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-300 text-sm flex items-start space-x-3">
              <AlertCircle className="w-5 h-5 shrink-0 mt-0.5" />
              <div>
                <div className="font-semibold">源站未返回有效结果</div>
                <div className="text-xs text-amber-200/70 mt-0.5">{result.error || '未匹配到相关条目'}</div>
              </div>
            </div>
          )}

          {result && result.success && (
            <div className="space-y-4">
              <div className="flex items-center justify-between text-xs text-slate-400 pb-2 border-b border-slate-800">
                <span className="flex items-center space-x-1.5 text-emerald-400 font-medium">
                  <CheckCircle2 className="w-4 h-4" />
                  <span>搜索成功，匹配到 {result.resultsCount} 个条目</span>
                </span>
                <span>BaseURL: {rule.baseURL}</span>
              </div>

              {/* Items List */}
              <div className="space-y-2">
                {result.results?.slice(0, 3).map((item, idx) => (
                  <div
                    key={idx}
                    className="p-3 bg-slate-950/80 border border-slate-800 rounded-xl flex items-center justify-between text-sm"
                  >
                    <div className="font-medium text-slate-200 truncate pr-4">{item.name}</div>
                    <a
                      href={item.dramaUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs text-pink-400 hover:text-pink-300 flex items-center space-x-1 shrink-0"
                    >
                      <span>打开源页</span>
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  </div>
                ))}
              </div>

              {/* Chapters Preview */}
              {result.chapters && result.chapters.length > 0 && (
                <div className="mt-4 pt-4 border-t border-slate-800">
                  <h4 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                    首个匹配条目剧集嗅探 (共 {result.chapters[0]?.episodes?.length || 0} 话)
                  </h4>
                  <div className="grid grid-cols-4 sm:grid-cols-6 gap-2 max-h-40 overflow-y-auto p-1">
                    {result.chapters[0]?.episodes?.map((ep, eIdx) => (
                      <div
                        key={eIdx}
                        title={ep.url}
                        className="px-2.5 py-1.5 bg-slate-800/60 border border-slate-700/50 rounded-lg text-xs text-center text-slate-300 truncate hover:border-pink-500/40 transition-colors"
                      >
                        {ep.name}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {!result && !error && !loading && (
            <div className="text-center py-12 text-slate-500">
              <Search className="w-8 h-8 mx-auto mb-2 opacity-40" />
              <p className="text-sm">点击“运行测试”验证此规则是否能够正常检索和嗅探剧集</p>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/50 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-medium rounded-xl transition-colors cursor-pointer"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  )
}
