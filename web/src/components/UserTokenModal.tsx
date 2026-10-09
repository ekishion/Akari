import React, { useState, useEffect } from 'react'
import { Key, Plus, Trash2, Copy, Check, ShieldCheck, Laptop } from 'lucide-react'
import { api } from '../api'
import type { UserView, UserToken } from '../types'
import { MdDialog } from './md3/MdDialog'
import { MdButton } from './md3/MdButton'
import { MdTextField } from './md3/MdTextField'

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

  if (!user) return null

  return (
    <MdDialog
      open={isOpen}
      onClose={onClose}
      title={`客户端访问令牌: ${user.name}`}
      subtitle="可为电视盒子、Apple TV Infuse、VidHub 等客户端生成免密直连 API 令牌。"
      icon={<Key className="w-5 h-5 text-[var(--md-primary)]" />}
      maxWidth="2xl"
      actions={
        <MdButton variant="tonal" onClick={onClose}>
          完成
        </MdButton>
      }
    >
      <div className="space-y-5">
        {/* Create Token Form */}

        {/* Create Token Form */}
        <form onSubmit={handleCreateToken} className="flex gap-2.5">
          <div className="flex-1">
            <MdTextField
              value={clientName}
              onChange={(e) => setClientName(e.target.value)}
              placeholder="设备备注，例如: 客厅 Apple TV、卧室 iPad Infuse..."
              leadingIcon={<Laptop className="w-4 h-4" />}
            />
          </div>
          <MdButton
            type="submit"
            variant="filled"
            disabled={creating}
            loading={creating}
            icon={<Plus className="w-4 h-4" />}
          >
            {creating ? '生成中...' : '生成新令牌'}
          </MdButton>
        </form>

        {/* Tokens List */}
        <div className="space-y-3">
          {error && (
            <div className="p-3 bg-[var(--md-danger)]/10 text-[var(--md-danger)] text-xs rounded-2xl border border-[var(--md-danger)]/20">
              {error}
            </div>
          )}

          {tokens.length === 0 && !loading && (
            <div className="text-center py-10 text-[var(--md-on-surface-variant)]/60 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)]/40 rounded-2xl border border-dashed border-[var(--md-outline-variant)]">
              <ShieldCheck className="w-9 h-9 mx-auto mb-2 opacity-30" />
              <p className="text-sm font-medium">暂无独立客户端令牌</p>
              <p className="text-xs text-[var(--md-on-surface-variant)]/50 mt-1">
                可通过上方输入设备备注并立即生成
              </p>
            </div>
          )}

          {tokens.map((t) => (
            <div
              key={t.token}
              className="p-4 bg-[var(--md-surface-container-low)] dark:bg-[var(--md-surface-container)] border border-[var(--md-outline-variant)] rounded-2xl flex items-center justify-between gap-3 group hover:border-[var(--md-primary)]/40 transition-colors"
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-semibold text-sm text-[var(--md-on-surface)]">
                    {t.clientName || 'Emby Client'}
                  </span>
                  <span className="text-[11px] text-[var(--md-on-surface-variant)]/70">
                    创建于 {new Date(t.createdAt).toLocaleDateString()}
                  </span>
                </div>
                <div className="font-mono text-xs text-[var(--md-primary)] bg-[var(--md-surface-container-high)] border border-[var(--md-outline-variant)]/50 px-2.5 py-1 rounded-xl mt-1.5 inline-block select-all max-w-full truncate">
                  {t.token}
                </div>
              </div>

              <div className="flex items-center gap-2 shrink-0">
                <MdButton
                  variant="tonal"
                  size="sm"
                  onClick={() => handleCopy(t.token)}
                  icon={
                    copiedToken === t.token ? (
                      <Check className="w-3.5 h-3.5 text-emerald-500" />
                    ) : (
                      <Copy className="w-3.5 h-3.5" />
                    )
                  }
                >
                  {copiedToken === t.token ? '已复制' : '复制令牌'}
                </MdButton>
                <button
                  type="button"
                  onClick={() => handleRevokeToken(t.token)}
                  title="注销令牌"
                  className="p-2 text-[var(--md-on-surface-variant)] hover:text-[var(--md-danger)] hover:bg-[var(--md-danger)]/10 rounded-full transition-colors cursor-pointer"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </MdDialog>
  )
}
