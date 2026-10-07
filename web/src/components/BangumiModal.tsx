import React, { useState } from 'react'
import { X, ExternalLink, AlertCircle, CheckCircle2, Trash2 } from 'lucide-react'
import { api } from '../api'
import type { UserView } from '../types'

interface BangumiModalProps {
  user: UserView | null
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export const BangumiModal: React.FC<BangumiModalProps> = ({ user, isOpen, onClose, onSuccess }) => {
  const [token, setToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successData, setSuccessData] = useState<any | null>(null)

  if (!isOpen || !user) return null

  const handleBind = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!token.trim()) return

    setLoading(true)
    setError(null)
    try {
      const res = await api.bindBangumi(user.id, token.trim())
      setSuccessData(res)
      setTimeout(() => {
        onSuccess()
        onClose()
      }, 1500)
    } catch (err: any) {
      setError(err.message || '绑定 Bangumi 账号失败')
    } finally {
      setLoading(false)
    }
  }

  const handleUnbind = async () => {
    if (!confirm('确定要解除此用户的 Bangumi 账号绑定吗？')) return
    try {
      await api.unbindBangumi(user.id)
      onSuccess()
      onClose()
    } catch (err: any) {
      alert(err.message || '解绑失败')
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in">
      <div className="bg-slate-900 border border-slate-800 w-full max-w-lg rounded-2xl shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/50">
          <div className="flex items-center space-x-2">
            <div className="w-8 h-8 rounded-lg bg-pink-500/20 text-pink-400 flex items-center justify-center font-bold">
              🌺
            </div>
            <div>
              <h3 className="text-base font-semibold text-white">Bangumi 账号关联: {user.name}</h3>
              <p className="text-xs text-slate-400">绑定番组计划，在 Emby / Infuse 中同步个人在看与追番列表</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-4">
          {error && (
            <div className="p-3.5 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs flex items-center space-x-2">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {successData && (
            <div className="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs flex items-center space-x-2">
              <CheckCircle2 className="w-4 h-4 shrink-0" />
              <span>成功绑定 Bangumi 用户: {successData.nickname || successData.username}！</span>
            </div>
          )}

          {user.hasBgmToken && (
            <div className="p-4 bg-pink-500/10 border border-pink-500/20 rounded-xl flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 rounded-full bg-pink-500/20 flex items-center justify-center text-pink-300 font-bold text-lg">
                  🌸
                </div>
                <div>
                  <div className="text-sm font-semibold text-pink-200">当前已绑定 Bangumi 账号</div>
                  <div className="text-xs text-slate-400">用户名: {user.bgmUserId || '已配置 Token'}</div>
                </div>
              </div>
              <button
                onClick={handleUnbind}
                className="px-3 py-1.5 bg-red-500/20 hover:bg-red-500/30 text-red-300 text-xs font-medium rounded-lg flex items-center space-x-1 transition-colors cursor-pointer"
              >
                <Trash2 className="w-3.5 h-3.5" />
                <span>解除绑定</span>
              </button>
            </div>
          )}

          <form onSubmit={handleBind} className="space-y-4">
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider">
                  Bangumi 个人 Access Token
                </label>
                <a
                  href="https://next.bgm.tv/demo/access-token"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs text-pink-400 hover:text-pink-300 flex items-center space-x-1"
                >
                  <span>获取 Token</span>
                  <ExternalLink className="w-3 h-3" />
                </a>
              </div>
              <input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="粘贴 Bangumi Personal Access Token..."
                className="w-full px-4 py-2.5 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-pink-500 transition-colors"
              />
              <p className="text-[11px] text-slate-500 mt-1.5 leading-relaxed">
                绑定后，Emby 首页中的“正在追番”和“特别关注”将自动替换为您在 Bangumi 标记的「在看」与「想看」动画！
              </p>
            </div>

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
                disabled={loading || !token.trim()}
                className="px-5 py-2 bg-gradient-to-r from-pink-500 to-purple-600 hover:from-pink-600 hover:to-purple-700 text-white text-sm font-medium rounded-xl transition-all shadow-md shadow-pink-500/20 disabled:opacity-50 cursor-pointer"
              >
                {loading ? '验证并绑定中...' : '验证并绑定'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}
