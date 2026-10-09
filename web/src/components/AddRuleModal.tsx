import React, { useState } from 'react'
import { Code, Download, Plus, AlertCircle, CheckCircle2, Globe, Sparkles } from 'lucide-react'
import { api } from '../api'
import { MdDialog } from './md3/MdDialog'
import { MdButton } from './md3/MdButton'
import { MdTextField } from './md3/MdTextField'

interface AddRuleModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export const AddRuleModal: React.FC<AddRuleModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const [mode, setMode] = useState<'url' | 'json'>('url')
  const [jsonText, setJsonText] = useState('')
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  const handleImport = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccessMsg(null)
    setLoading(true)

    try {
      if (mode === 'url') {
        if (!url.trim()) throw new Error('请输入有效的规则订阅 URL')
        const res = await api.importRulesFromUrl(url.trim())
        const detail = res.updatedCount > 0
          ? `成功导入 ${res.importedCount} 条规则（新增 ${res.addedCount} 条，覆盖更新 ${res.updatedCount} 条）！`
          : `成功导入 ${res.importedCount} 条规则！`
        setSuccessMsg(detail)
        setTimeout(() => {
          onSuccess()
          onClose()
        }, 1200)
      } else {
        if (!jsonText.trim()) throw new Error('请输入 Kazumi 规则 JSON 内容')
        let parsed: any
        try {
          parsed = JSON.parse(jsonText)
        } catch {
          throw new Error('JSON 格式错误，请检查语法')
        }

        if (Array.isArray(parsed)) {
          let count = 0
          for (const item of parsed) {
            await api.saveRule(item)
            count++
          }
          setSuccessMsg(`成功添加 ${count} 条规则！`)
        } else {
          await api.saveRule(parsed)
          setSuccessMsg(`成功保存规则 "${parsed.name || '未命名'}"！`)
        }

        setTimeout(() => {
          onSuccess()
          onClose()
        }, 1000)
      }
    } catch (err: any) {
      setError(err.message || '导入规则失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSetPreset = (presetUrl: string) => {
    setUrl(presetUrl)
    setMode('url')
  }

  return (
    <MdDialog
      open={isOpen}
      onClose={onClose}
      title="添加 / 导入 Kazumi 解析规则"
      subtitle="支持直接粘贴 Kazumi JSON 规则格式，或通过远程订阅链接批量导入源站规则仓库。"
      icon={<Plus className="w-5 h-5 text-[var(--md-primary)]" />}
      maxWidth="2xl"
      actions={
        <>
          <MdButton variant="text" onClick={onClose}>
            取消
          </MdButton>
          <MdButton
            variant="filled"
            onClick={handleImport}
            disabled={loading}
            loading={loading}
          >
            {loading ? '正在处理...' : '确认导入'}
          </MdButton>
        </>
      }
    >
      <div className="space-y-5">
        {/* Mode Switcher Pills */}

        {/* Mode Switcher Pills */}
        <div className="flex p-1 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] rounded-full border border-[var(--md-outline-variant)]">
          <button
            type="button"
            onClick={() => setMode('url')}
            className={`flex-1 flex items-center justify-center gap-2 py-2 text-xs font-semibold rounded-full transition-all duration-200 cursor-pointer ${
              mode === 'url'
                ? 'bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] shadow-sm'
                : 'text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-surface)]'
            }`}
          >
            <Download className="w-4 h-4" />
            <span>远程订阅 / URL 批量导入</span>
          </button>
          <button
            type="button"
            onClick={() => setMode('json')}
            className={`flex-1 flex items-center justify-center gap-2 py-2 text-xs font-semibold rounded-full transition-all duration-200 cursor-pointer ${
              mode === 'json'
                ? 'bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] shadow-sm'
                : 'text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-surface)]'
            }`}
          >
            <Code className="w-4 h-4" />
            <span>手动 JSON 规则输入</span>
          </button>
        </div>

        {/* Status Alerts */}
        {error && (
          <div className="p-3.5 rounded-2xl bg-[var(--md-danger)]/10 border border-[var(--md-danger)]/20 text-[var(--md-danger)] text-xs flex items-center gap-2.5 animate-fade-in">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {successMsg && (
          <div className="p-3.5 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-xs flex items-center gap-2.5 animate-fade-in">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{successMsg}</span>
          </div>
        )}

        {/* Mode Body */}
        {mode === 'url' ? (
          <div className="space-y-4">
            <MdTextField
              label="规则订阅 URL"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://raw.githubusercontent.com/.../plugins.json"
              leadingIcon={<Globe className="w-4 h-4" />}
            />

            {/* Presets */}
            <div className="space-y-2 pt-1">
              <div className="flex items-center gap-1.5 text-xs font-semibold text-[var(--md-on-surface-variant)] px-1">
                <Sparkles className="w-3.5 h-3.5 text-amber-500" />
                <span>常用 Kazumi 规则预设仓库</span>
              </div>
              <div className="grid grid-cols-1 gap-2">
                {[
                  {
                    title: 'Predidit/KazumiRules (jsDelivr 全球 CDN 加速)',
                    url: 'https://fastly.jsdelivr.net/gh/Predidit/KazumiRules@main/index.json',
                    desc: '推荐：国内及海外高速直连',
                  },
                  {
                    title: 'Predidit/KazumiRules (GitHub 官方源)',
                    url: 'https://raw.githubusercontent.com/Predidit/KazumiRules/main/index.json',
                    desc: '官方实时同步源（需要国际网络）',
                  },
                  {
                    title: 'Predidit/KazumiRules (GHProxy 国内镜像加速)',
                    url: 'https://ghproxy.net/https://raw.githubusercontent.com/Predidit/KazumiRules/main/index.json',
                    desc: '镜像代理节点加速',
                  },
                ].map((preset, idx) => (
                  <button
                    key={idx}
                    type="button"
                    onClick={() => handleSetPreset(preset.url)}
                    className="w-full text-left p-3.5 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)] hover:border-[var(--md-primary)] rounded-2xl flex items-center justify-between gap-3 transition-all duration-200 cursor-pointer group"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="text-xs font-semibold text-[var(--md-on-surface)] group-hover:text-[var(--md-primary)] transition-colors">
                        {preset.title}
                      </div>
                      <div className="text-[11px] text-[var(--md-on-surface-variant)]/70 font-mono truncate mt-0.5">
                        {preset.url}
                      </div>
                    </div>
                    <span className="text-xs font-medium text-[var(--md-primary)] bg-[var(--md-primary-container)]/40 px-2.5 py-1 rounded-full shrink-0">
                      填入
                    </span>
                  </button>
                ))}
              </div>
            </div>
          </div>
        ) : (
          <div className="space-y-2">
            <label className="block text-xs font-semibold text-[var(--md-on-surface-variant)] px-1">
              Kazumi 规则 JSON 内容 (单条对象或规则数组)
            </label>
            <div className="bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] rounded-2xl border border-[var(--md-outline-variant)] p-3 focus-within:ring-2 focus-within:ring-[var(--md-primary)]">
              <textarea
                rows={9}
                value={jsonText}
                onChange={(e) => setJsonText(e.target.value)}
                placeholder={`{\n  "name": "极速动漫",\n  "version": "1.2",\n  "api": "8",\n  "type": "anime",\n  "baseURL": "https://m.ezdmw.org/",\n  "searchMode": "xpath"\n}`}
                className="w-full font-mono text-xs bg-transparent text-[var(--md-on-surface)] placeholder:text-[var(--md-on-surface-variant)]/40 outline-none leading-relaxed resize-none"
              />
            </div>
          </div>
        )}
      </div>
    </MdDialog>
  )
}
