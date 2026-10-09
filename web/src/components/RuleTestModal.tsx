import React, { useState } from 'react'
import { Search, Play, CheckCircle2, AlertCircle, ExternalLink, Zap } from 'lucide-react'
import { api } from '../api'
import type { RulePlugin, TestResult } from '../types'
import { MdDialog } from './md3/MdDialog'
import { MdButton } from './md3/MdButton'

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

  if (!rule) return null

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
      setError(err.message || '测试解析失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <MdDialog
      open={isOpen}
      onClose={onClose}
      title={`测试解析源: ${rule.name}`}
      subtitle="实时向源站发送搜索与分集抓取请求，校验 XPath/JSON 规则的可用性与剧集嗅探结果。"
      icon={<Zap className="w-5 h-5 text-[var(--md-primary)]" />}
      maxWidth="2xl"
      actions={
        <MdButton variant="tonal" onClick={onClose}>
          关闭
        </MdButton>
      }
    >
      <div className="space-y-5">
        {/* Search Bar */}

        {/* Search Bar */}
        <form onSubmit={handleTest} className="flex gap-2.5">
          <div className="relative flex-1">
            <div className="relative flex items-center bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] rounded-2xl border border-[var(--md-outline-variant)] px-4 py-2.5 focus-within:ring-2 focus-within:ring-[var(--md-primary)] transition-all">
              <Search className="w-4 h-4 text-[var(--md-on-surface-variant)] mr-2.5 shrink-0" />
              <input
                type="text"
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder="输入动漫名称测试，例如: 葬送的芙莉莲、间谍过家家..."
                className="w-full bg-transparent text-sm text-[var(--md-on-surface)] placeholder:text-[var(--md-on-surface-variant)]/50 outline-none"
              />
            </div>
          </div>
          <MdButton
            type="submit"
            variant="filled"
            disabled={loading || !keyword.trim()}
            loading={loading}
            icon={!loading ? <Play className="w-4 h-4 fill-current" /> : undefined}
          >
            {loading ? '嗅探中...' : '运行测试'}
          </MdButton>
        </form>

        {/* Results Container */}
        <div className="space-y-4">
          {error && (
            <div className="p-4 rounded-2xl bg-[var(--md-danger)]/10 border border-[var(--md-danger)]/20 text-[var(--md-danger)] text-xs flex items-start gap-3">
              <AlertCircle className="w-5 h-5 shrink-0 mt-0.5" />
              <div>
                <div className="font-semibold text-sm">抓取失败</div>
                <div className="mt-1 leading-relaxed opacity-90 break-all">{error}</div>
              </div>
            </div>
          )}

          {result && !result.success && (
            <div className="p-4 rounded-2xl bg-amber-500/10 border border-amber-500/20 text-amber-600 dark:text-amber-400 text-xs flex items-start gap-3">
              <AlertCircle className="w-5 h-5 shrink-0 mt-0.5" />
              <div>
                <div className="font-semibold text-sm">源站未返回有效结果</div>
                <div className="mt-1 opacity-90">{result.error || '未匹配到相关条目'}</div>
              </div>
            </div>
          )}

          {result && result.success && (
            <div className="space-y-4 animate-fade-in">
              {/* Stats Bar */}
              <div className="flex flex-wrap items-center justify-between gap-2 p-3 bg-[var(--md-surface-container)] rounded-2xl border border-[var(--md-outline-variant)]/50">
                <div className="flex items-center gap-2 text-xs font-medium text-emerald-600 dark:text-emerald-400">
                  <CheckCircle2 className="w-4 h-4" />
                  <span>搜索成功，匹配到 {result.resultsCount} 个条目</span>
                </div>
                <span className="text-xs font-mono text-[var(--md-on-surface-variant)] truncate max-w-xs">
                  {rule.baseURL}
                </span>
              </div>

              {/* Items List */}
              <div className="space-y-2">
                <div className="text-xs font-semibold text-[var(--md-on-surface-variant)] uppercase tracking-wider px-1">
                  搜索匹配结果
                </div>
                {result.results?.slice(0, 3).map((item, idx) => (
                  <div
                    key={idx}
                    className="p-3.5 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)] rounded-2xl flex items-center justify-between gap-3 text-sm hover:border-[var(--md-primary)]/50 transition-colors"
                  >
                    <div className="font-medium text-[var(--md-on-surface)] truncate">{item.name}</div>
                    <a
                      href={item.dramaUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs font-medium text-[var(--md-primary)] hover:underline inline-flex items-center gap-1 shrink-0 px-2.5 py-1 rounded-full bg-[var(--md-primary-container)]/50"
                    >
                      <span>打开源页</span>
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  </div>
                ))}
              </div>

              {/* Chapters Preview */}
              {result.chapters && result.chapters.length > 0 && (
                <div className="space-y-2 pt-2">
                  <div className="text-xs font-semibold text-[var(--md-on-surface-variant)] uppercase tracking-wider px-1">
                    首条剧集嗅探预览 (共 {result.chapters[0]?.episodes?.length || 0} 话)
                  </div>
                  <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-2 max-h-48 overflow-y-auto p-1">
                    {result.chapters[0]?.episodes?.map((ep, eIdx) => (
                      <div
                        key={eIdx}
                        title={ep.url}
                        className="px-2.5 py-2 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)] rounded-xl text-xs text-center text-[var(--md-on-surface)] truncate hover:border-[var(--md-primary)] hover:bg-[var(--md-primary-container)]/20 transition-colors"
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
            <div className="text-center py-12 text-[var(--md-on-surface-variant)]/60 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)]/40 rounded-2xl border border-dashed border-[var(--md-outline-variant)]">
              <Search className="w-10 h-10 mx-auto mb-2 opacity-30" />
              <p className="text-sm font-medium">点击“运行测试”验证解析规则与剧集嗅探</p>
              <p className="text-xs text-[var(--md-on-surface-variant)]/50 mt-1">支持实时抓取番剧名称与分集播放直链</p>
            </div>
          )}
        </div>
      </div>
    </MdDialog>
  )
}
