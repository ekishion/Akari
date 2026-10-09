import React, { useState } from 'react'
import { ExternalLink, AlertCircle, CheckCircle2, Trash2, Heart } from 'lucide-react'
import { api } from '../api'
import type { UserView } from '../types'
import { MdDialog } from './md3/MdDialog'
import { MdButton } from './md3/MdButton'
import { MdTextField } from './md3/MdTextField'

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

  if (!user) return null

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
    <MdDialog
      open={isOpen}
      onClose={onClose}
      title={`Bangumi 账号关联: ${user.name}`}
      subtitle="绑定番组计划，在 Emby 首页中自动同步您在 Bangumi 标记的「在看」与「想看」动画。"
      icon={<Heart className="w-5 h-5 text-pink-500 fill-pink-500" />}
      maxWidth="lg"
      actions={
        <>
          <MdButton variant="text" onClick={onClose}>
            取消
          </MdButton>
          <MdButton
            variant="filled"
            onClick={handleBind}
            disabled={loading || !token.trim()}
            loading={loading}
          >
            {loading ? '验证并绑定中...' : '验证并绑定'}
          </MdButton>
        </>
      }
    >
      <div className="space-y-5">
        {error && (
          <div className="p-3.5 rounded-2xl bg-[var(--md-danger)]/10 border border-[var(--md-danger)]/20 text-[var(--md-danger)] text-xs flex items-center gap-2.5 animate-fade-in">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {successData && (
          <div className="p-3.5 rounded-2xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-xs flex items-center gap-2.5 animate-fade-in">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>成功绑定 Bangumi 用户: {successData.nickname || successData.username}！</span>
          </div>
        )}

        {user.hasBgmToken && (
          <div className="p-4 bg-[var(--md-primary-container)]/30 border border-[var(--md-primary)]/30 rounded-2xl flex items-center justify-between gap-3">
            <div className="flex items-center gap-3 min-w-0">
              <div className="w-10 h-10 rounded-full bg-pink-500/20 text-pink-500 flex items-center justify-center font-bold text-base shrink-0">
                🌺
              </div>
              <div className="min-w-0">
                <div className="text-sm font-semibold text-[var(--md-on-surface)] truncate">
                  当前已绑定 Bangumi 账号
                </div>
                <div className="text-xs text-[var(--md-on-surface-variant)] truncate">
                  用户标识: {user.bgmUserId || '已配置 Token'}
                </div>
              </div>
            </div>
            <button
              type="button"
              onClick={handleUnbind}
              className="px-3 py-1.5 bg-[var(--md-danger)]/15 hover:bg-[var(--md-danger)]/25 text-[var(--md-danger)] text-xs font-semibold rounded-full flex items-center gap-1 transition-colors cursor-pointer shrink-0"
            >
              <Trash2 className="w-3.5 h-3.5" />
              <span>解除绑定</span>
            </button>
          </div>
        )}

        <form onSubmit={handleBind} className="space-y-4">
          <div className="space-y-2">
            <div className="flex items-center justify-between px-1">
              <span className="text-xs font-semibold text-[var(--md-on-surface-variant)]">
                Bangumi 个人 Access Token
              </span>
              <a
                href="https://next.bgm.tv/demo/access-token"
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs font-medium text-[var(--md-primary)] hover:underline inline-flex items-center gap-1"
              >
                <span>获取 Token</span>
                <ExternalLink className="w-3 h-3" />
              </a>
            </div>

            <MdTextField
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="粘贴 Bangumi Personal Access Token..."
            />
            <p className="text-[11px] text-[var(--md-on-surface-variant)]/70 px-1 leading-relaxed">
              Token 仅保存在本地服务器，用于直接拉取 Bangumi 收藏数据，不会向第三方泄漏。
            </p>
          </div>
        </form>
      </div>
    </MdDialog>
  )
}
