import React, { useState, useEffect } from 'react'
import { X, Key, Plus, Trash2, Copy, Check, ShieldCheck, Laptop } from 'lucide-react'
import { api } from '../api'
import type { UserView, UserToken } from '../types'

interface UserTokenModalProps {
  user: UserView | null
  isOpen: boolean
  onClose: () => void
}

export const UserTokenModal: React.FC<UserTokenModalProps> = ({ user, isOpen, onClose }) => {
  const [tokens, setTokens] = useState<UserToken[]>([])
  const [loading, setLoading] = useState(false)
  const [clientName, setClientName] = useState('')
  const [copiedToken, setCopiedToken] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen && user) {
      loadTokens()
    }
  }, [isOpen, user])

  const loadTokens = async () => {
    if (!user) return
    setLoading(true)
    setError(null)
    try {
      const list = await api.getUserTokens(user.id)
      setTokens(list || [])
    } catch (err: any) {
      setError(err.message || '加载令牌列表失败')
    } finally {
      setLoading(false)
    }
  }

  const handleCreateToken = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!user) return
    setCreating(true)
    setError(null)
    try {
      await api.createUserToken(user.id, clientName.trim() || 'Infuse Client')
      setClientName('')
      await loadTokens()
    } catch (err: any) {
      setError(err.message || '创建令牌失败')
    } finally {
      setCreating(false)
    }
  }

  const handleRevokeToken = async (tokenStr: string) => {
    if (!user) return
    if (!confirm('确定要注销此令牌吗？对应的播放客户端将失去连接访问权限。')) return
    try {
      await api.revokeUserToken(user.id, tokenStr)
      await loadTokens()
    } catch (err: any) {
      alert(err.message || '注销失败')
    }
  }

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedToken(text)
    setTimeout(() => setCopiedToken(null), 2000)
  }

  if (!isOpen || !user) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 w-full max-w-2xl rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/50">
          <div className="flex items-center space-x-2">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/20 text-emerald-400 flex items-center justify-center font-bold">
              <Key className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-semibold text-white">客户端访问令牌: {user.name}</h3>
              <p className="text-xs text-slate-400">
                可为电视盒子、Apple TV Infuse、VidHub 等客户端生成免密直连 API 令牌
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Create Token Form */}
        <form onSubmit={handleCreateToken} className="p-6 border-b border-slate-800 bg-slate-900/50 flex gap-2">
          <div className="relative flex-1">
            <Laptop className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              value={clientName}
              onChange={(e) => setClientName(e.target.value)}
              placeholder="设备或客户端备注，例如: 客厅 Apple TV、卧室 iPad Infuse..."
              className="w-full pl-10 pr-4 py-2.5 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-pink-500 transition-colors"
            />
          </div>
          <button
            type="submit"
            disabled={creating}
            className="px-5 py-2.5 bg-gradient-to-r from-emerald-500 to-teal-600 hover:from-emerald-600 hover:to-teal-700 text-white font-medium text-sm rounded-xl flex items-center space-x-1.5 transition-all shadow-md shadow-emerald-500/20 disabled:opacity-50 cursor-pointer shrink-0"
          >
            <Plus className="w-4 h-4" />
            <span>{creating ? '生成中...' : '生成新令牌'}</span>
          </button>
        </form>

        {/* Tokens List */}
        <div className="p-6 overflow-y-auto space-y-3 flex-1">
          {error && <div className="p-3 bg-red-500/10 text-red-400 text-xs rounded-xl">{error}</div>}

          {tokens.length === 0 && !loading && (
            <div className="text-center py-10 text-slate-500">
              <ShieldCheck className="w-8 h-8 mx-auto mb-2 opacity-40" />
              <p className="text-sm">暂无独立客户端令牌，可通过上方输入备注立即生成</p>
            </div>
          )}

          {tokens.map((t) => (
            <div
              key={t.token}
              className="p-4 bg-slate-950/80 border border-slate-800 rounded-xl flex items-center justify-between gap-3 group hover:border-slate-700 transition-colors"
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center space-x-2">
                  <span className="font-semibold text-sm text-slate-200">{t.clientName || 'Emby Client'}</span>
                  <span className="text-[11px] text-slate-500">
                    创建于 {new Date(t.createdAt).toLocaleDateString()}
                  </span>
                </div>
                <div className="font-mono text-xs text-pink-400 bg-slate-900 px-2 py-1 rounded-lg mt-1.5 inline-block select-all max-w-full truncate">
                  {t.token}
                </div>
              </div>

              <div className="flex items-center space-x-2 shrink-0">
                <button
                  onClick={() => handleCopy(t.token)}
                  className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg flex items-center space-x-1 transition-colors cursor-pointer"
                >
                  {copiedToken === t.token ? (
                    <>
                      <Check className="w-3.5 h-3.5 text-emerald-400" />
                      <span className="text-emerald-400">已复制</span>
                    </>
                  ) : (
                    <>
                      <Copy className="w-3.5 h-3.5" />
                      <span>复制令牌</span>
                    </>
                  )}
                </button>
                <button
                  onClick={() => handleRevokeToken(t.token)}
                  title="注销令牌"
                  className="p-1.5 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-colors cursor-pointer"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/50 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-medium rounded-xl transition-colors cursor-pointer"
          >
            完成
          </button>
        </div>
      </div>
    </div>
  )
}
