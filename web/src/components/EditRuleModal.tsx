import React, { useState, useEffect } from 'react'
import { Edit3, Code, FileText, CheckCircle2, AlertCircle } from 'lucide-react'
import { api } from '../api'
import type { RulePlugin } from '../types'
import { MdDialog } from './md3/MdDialog'
import { MdButton } from './md3/MdButton'
import { MdTextField } from './md3/MdTextField'
import { MdSwitch } from './md3/MdSwitch'

interface EditRuleModalProps {
  rule: RulePlugin | null
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export const EditRuleModal: React.FC<EditRuleModalProps> = ({
  rule,
  isOpen,
  onClose,
  onSuccess,
}) => {
  const [activeTab, setActiveTab] = useState<'form' | 'json'>('form')
  const [fullRule, setFullRule] = useState<any>(null)
  const [jsonContent, setJsonContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  // Form Fields
  const [name, setName] = useState('')
  const [version, setVersion] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [searchURL, setSearchURL] = useState('')
  const [searchMode, setSearchMode] = useState('xpath')
  const [adBlocker, setAdBlocker] = useState(false)
  const [multiSources, setMultiSources] = useState(false)

  useEffect(() => {
    if (!isOpen || !rule) {
      setFullRule(null)
      setJsonContent('')
      setError(null)
      setSuccessMsg(null)
      return
    }

    const fetchFull = async () => {
      setLoading(true)
      setError(null)
      try {
        const data = await api.getRule(rule.name || rule.id)
        setFullRule(data)
        setJsonContent(JSON.stringify(data, null, 2))
        setName(data.name || rule.name)
        setVersion(data.version || rule.version || '1.0')
        setBaseURL(data.baseURL || rule.baseURL || '')
        setSearchURL(data.searchURL || rule.searchURL || '')
        setSearchMode(data.searchMode || rule.searchMode || 'xpath')
        setAdBlocker(!!data.adBlocker)
        setMultiSources(!!data.muliSources || !!data.multiSources)
      } catch (err: any) {
        // Fallback to prop
        setFullRule(rule)
        setJsonContent(JSON.stringify(rule, null, 2))
        setName(rule.name)
        setVersion(rule.version || '1.0')
        setBaseURL(rule.baseURL || '')
        setSearchURL(rule.searchURL || '')
        setSearchMode(rule.searchMode || 'xpath')
        setAdBlocker(!!rule.adBlocker)
        setMultiSources(!!rule.multiSources)
      } finally {
        setLoading(false)
      }
    }

    fetchFull()
  }, [isOpen, rule])

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccessMsg(null)
    setSaving(true)

    try {
      let payloadToSave: any
      if (activeTab === 'json') {
        try {
          payloadToSave = JSON.parse(jsonContent)
        } catch {
          throw new Error('JSON 格式无效，请检查语法')
        }
      } else {
        payloadToSave = {
          ...(fullRule || {}),
          name: name.trim(),
          version: version.trim() || '1.0',
          baseURL: baseURL.trim(),
          searchURL: searchURL.trim(),
          searchMode: searchMode.trim(),
          adBlocker,
          muliSources: multiSources,
        }
      }

      if (!payloadToSave.name) {
        throw new Error('规则名称不能为空')
      }

      await api.saveRule(payloadToSave)
      setSuccessMsg(`规则 "${payloadToSave.name}" 保存并覆盖更新成功！`)
      setTimeout(() => {
        onSuccess()
        onClose()
      }, 800)
    } catch (err: any) {
      setError(err.message || '保存规则失败')
    } finally {
      setSaving(false)
    }
  }

  if (!rule) return null

  return (
    <MdDialog
      open={isOpen}
      onClose={onClose}
      title={`编辑解析规则: ${rule.name}`}
      subtitle="修改规则配置或直接编辑 JSON 定义，保存后将自动覆盖旧版本。"
      icon={<Edit3 className="w-5 h-5 text-[var(--md-primary)]" />}
      maxWidth="2xl"
      actions={
        <>
          <MdButton variant="text" onClick={onClose}>
            取消
          </MdButton>
          <MdButton
            variant="filled"
            onClick={handleSave}
            disabled={loading || saving}
            loading={saving}
          >
            {saving ? '正在保存...' : '保存覆盖规则'}
          </MdButton>
        </>
      }
    >
      <div className="space-y-4">
        {/* Mode Switcher */}
        <div className="flex p-1 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] rounded-full border border-[var(--md-outline-variant)]">
          <button
            type="button"
            onClick={() => {
              if (activeTab === 'json') {
                try {
                  const p = JSON.parse(jsonContent)
                  setName(p.name || '')
                  setVersion(p.version || '1.0')
                  setBaseURL(p.baseURL || '')
                  setSearchURL(p.searchURL || '')
                  setSearchMode(p.searchMode || 'xpath')
                  setAdBlocker(!!p.adBlocker)
                  setMultiSources(!!p.muliSources || !!p.multiSources)
                } catch {}
              }
              setActiveTab('form')
            }}
            className={`flex-1 flex items-center justify-center gap-2 py-1.5 text-xs font-semibold rounded-full transition-colors cursor-pointer ${
              activeTab === 'form'
                ? 'bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] shadow-sm'
                : 'text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-surface)]'
            }`}
          >
            <FileText className="w-3.5 h-3.5" />
            <span>可视化表单配置</span>
          </button>
          <button
            type="button"
            onClick={() => {
              if (activeTab === 'form') {
                const updated = {
                  ...(fullRule || {}),
                  name: name.trim(),
                  version: version.trim() || '1.0',
                  baseURL: baseURL.trim(),
                  searchURL: searchURL.trim(),
                  searchMode: searchMode.trim(),
                  adBlocker,
                  muliSources: multiSources,
                }
                setJsonContent(JSON.stringify(updated, null, 2))
              }
              setActiveTab('json')
            }}
            className={`flex-1 flex items-center justify-center gap-2 py-1.5 text-xs font-semibold rounded-full transition-colors cursor-pointer ${
              activeTab === 'json'
                ? 'bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] shadow-sm'
                : 'text-[var(--md-on-surface-variant)] hover:text-[var(--md-on-surface)]'
            }`}
          >
            <Code className="w-3.5 h-3.5" />
            <span>JSON 源码编辑</span>
          </button>
        </div>

        {/* Status Alerts */}
        {error && (
          <div className="p-3 rounded-2xl bg-[var(--md-danger-container)] text-[var(--md-on-danger-container)] text-xs flex items-center gap-2 font-medium">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {successMsg && (
          <div className="p-3 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-xs flex items-center gap-2 font-medium">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{successMsg}</span>
          </div>
        )}

        {loading ? (
          <div className="py-12 text-center text-xs text-[var(--md-on-surface-variant)]">
            正在加载规则完整数据...
          </div>
        ) : activeTab === 'form' ? (
          <form onSubmit={handleSave} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="sm:col-span-2">
                <MdTextField
                  label="规则名称 (Name)"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="例如: 稀饭动漫, ezdmw"
                  required
                />
              </div>
              <div>
                <MdTextField
                  label="版本 (Version)"
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  placeholder="1.0"
                />
              </div>
            </div>

            <MdTextField
              label="站点根地址 (Base URL)"
              value={baseURL}
              onChange={(e) => setBaseURL(e.target.value)}
              placeholder="https://example.com"
              required
            />

            <MdTextField
              label="搜索地址 (Search URL)"
              value={searchURL}
              onChange={(e) => setSearchURL(e.target.value)}
              placeholder="https://example.com/search?q=@keyword"
            />

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
              <div>
                <label className="block text-xs font-semibold text-[var(--md-on-surface)] mb-1.5">
                  解析模式 (Mode)
                </label>
                <select
                  value={searchMode}
                  onChange={(e) => setSearchMode(e.target.value)}
                  className="w-full px-3.5 py-2.5 rounded-2xl bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)] text-xs text-[var(--md-on-surface)] focus:outline-none focus:ring-2 focus:ring-[var(--md-primary)]"
                >
                  <option value="xpath">XPath 网页抓取解析</option>
                  <option value="api">API 动态接口 (JSONPath)</option>
                </select>
              </div>

              <div className="flex flex-col justify-center gap-3 pt-4">
                <label className="flex items-center justify-between text-xs text-[var(--md-on-surface)] cursor-pointer select-none">
                  <span>内置去广告 (AdBlocker)</span>
                  <MdSwitch checked={adBlocker} onChange={() => setAdBlocker(!adBlocker)} />
                </label>
                <label className="flex items-center justify-between text-xs text-[var(--md-on-surface)] cursor-pointer select-none">
                  <span>支持多播放线路 (MultiSources)</span>
                  <MdSwitch checked={multiSources} onChange={() => setMultiSources(!multiSources)} />
                </label>
              </div>
            </div>
          </form>
        ) : (
          <div className="space-y-2">
            <textarea
              value={jsonContent}
              onChange={(e) => setJsonContent(e.target.value)}
              rows={14}
              placeholder="输入合法的 Kazumi JSON 规则格式..."
              className="w-full p-4 rounded-2xl bg-[var(--md-surface-container-lowest)] border border-[var(--md-outline-variant)] text-xs font-mono text-[var(--md-on-surface)] focus:outline-none focus:ring-2 focus:ring-[var(--md-primary)] leading-relaxed resize-y"
              spellCheck={false}
            />
          </div>
        )}
      </div>
    </MdDialog>
  )
}
