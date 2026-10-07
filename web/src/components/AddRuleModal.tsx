import React, { useState } from 'react'
import { X, Code, Download, Plus, AlertCircle, CheckCircle2 } from 'lucide-react'
import { api } from '../api'

interface AddRuleModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export const AddRuleModal: React.FC<AddRuleModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const [mode, setMode] = useState<'json' | 'url'>('url')
  const [jsonText, setJsonText] = useState('')
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)

  if (!isOpen) return null

  const handleImport = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccessMsg(null)
    setLoading(true)

    try {
      if (mode === 'url') {
        if (!url.trim()) throw new Error('请输入有效的规则订阅 URL')
        const res = await api.importRulesFromUrl(url.trim())
        setSuccessMsg(`成功导入 ${res.importedCount} 条规则！`)
        setTimeout(() => {
          onSuccess()
          onClose()
        }, 1000)
      } else {
        if (!jsonText.trim()) throw new Error('请输入 Kazumi 规则 JSON 内容')
        let parsed
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 w-full max-w-2xl rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/50">
          <div className="flex items-center space-x-2">
            <div className="w-8 h-8 rounded-lg bg-indigo-500/20 text-indigo-400 flex items-center justify-center font-bold">
              <Plus className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-semibold text-white">添加 / 导入 Kazumi 解析规则</h3>
              <p className="text-xs text-slate-400">支持直接粘贴 Kazumi JSON 格式或通过远程订阅链接批量导入</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-slate-800 bg-slate-900 px-6 pt-3 gap-4">
          <button
            onClick={() => setMode('url')}
            className={`pb-3 text-sm font-medium flex items-center space-x-2 border-b-2 transition-colors ${
              mode === 'url'
                ? 'border-pink-500 text-pink-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Download className="w-4 h-4" />
            <span>远程订阅 / URL 导入</span>
          </button>
          <button
            onClick={() => setMode('json')}
            className={`pb-3 text-sm font-medium flex items-center space-x-2 border-b-2 transition-colors ${
              mode === 'json'
                ? 'border-pink-500 text-pink-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Code className="w-4 h-4" />
            <span>手动 JSON 输入</span>
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleImport} className="p-6 overflow-y-auto space-y-4 flex-1">
          {error && (
            <div className="p-3.5 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-center space-x-2">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {successMsg && (
            <div className="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs flex items-center space-x-2">
              <CheckCircle2 className="w-4 h-4 shrink-0" />
              <span>{successMsg}</span>
            </div>
          )}

          {mode === 'url' ? (
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                  规则订阅 URL
                </label>
                <input
                  type="url"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="https://raw.githubusercontent.com/.../plugins.json"
                  className="w-full px-4 py-2.5 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-pink-500 transition-colors"
                />
              </div>

              {/* Preset Links */}
              <div>
                <div className="text-xs font-medium text-slate-400 mb-2">常用 Kazumi 规则预设仓库:</div>
                <div className="space-y-2">
                  <button
                    type="button"
                    onClick={() =>
                      handleSetPreset('https://fastly.jsdelivr.net/gh/Predidit/KazumiRules@main/index.json')
                    }
                    className="w-full text-left p-2.5 bg-slate-950/60 border border-slate-800 hover:border-pink-500/40 rounded-xl text-xs flex items-center justify-between text-slate-300 transition-colors cursor-pointer"
                  >
                    <div>
                      <div className="font-medium text-slate-200">Predidit/KazumiRules (jsDelivr 全球加速)</div>
                      <div className="text-[11px] text-slate-500 font-mono">@main/index.json</div>
                    </div>
                    <span className="text-pink-400 font-mono text-[11px]">点击填入</span>
                  </button>

                  <button
                    type="button"
                    onClick={() =>
                      handleSetPreset('https://raw.githubusercontent.com/Predidit/KazumiRules/main/index.json')
                    }
                    className="w-full text-left p-2.5 bg-slate-950/60 border border-slate-800 hover:border-pink-500/40 rounded-xl text-xs flex items-center justify-between text-slate-300 transition-colors cursor-pointer"
                  >
                    <div>
                      <div className="font-medium text-slate-200">Predidit/KazumiRules (GitHub 官方源)</div>
                      <div className="text-[11px] text-slate-500 font-mono">main/index.json</div>
                    </div>
                    <span className="text-pink-400 font-mono text-[11px]">点击填入</span>
                  </button>

                  <button
                    type="button"
                    onClick={() =>
                      handleSetPreset('https://ghproxy.net/https://raw.githubusercontent.com/Predidit/KazumiRules/main/index.json')
                    }
                    className="w-full text-left p-2.5 bg-slate-950/60 border border-slate-800 hover:border-pink-500/40 rounded-xl text-xs flex items-center justify-between text-slate-300 transition-colors cursor-pointer"
                  >
                    <div>
                      <div className="font-medium text-slate-200">Predidit/KazumiRules (GHProxy 国内镜像)</div>
                      <div className="text-[11px] text-slate-500 font-mono">ghproxy.net 镜像代理</div>
                    </div>
                    <span className="text-pink-400 font-mono text-[11px]">点击填入</span>
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div>
              <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                Kazumi 规则 JSON (单条对象或数组)
              </label>
              <textarea
                rows={10}
                value={jsonText}
                onChange={(e) => setJsonText(e.target.value)}
                placeholder={`{\n  "name": "极速动漫",\n  "version": "1.2",\n  "api": "8",\n  "type": "anime",\n  "baseURL": "https://m.ezdmw.org/",\n  "searchMode": "xpath"\n}`}
                className="w-full font-mono text-xs p-4 bg-slate-950 border border-slate-700 rounded-xl text-slate-200 placeholder-slate-600 focus:outline-none focus:border-pink-500 transition-colors leading-relaxed"
              />
            </div>
          )}

          {/* Footer */}
          <div className="flex justify-end space-x-3 pt-4 border-t border-slate-800">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-medium rounded-xl transition-colors cursor-pointer"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-5 py-2 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-sm font-medium rounded-xl transition-all shadow-md shadow-pink-500/20 disabled:opacity-50 cursor-pointer"
            >
              {loading ? '正在处理...' : '确认导入'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
