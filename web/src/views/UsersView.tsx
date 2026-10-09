import React, { useState } from 'react'
import { motion, type Variants } from 'framer-motion'
import {
  Users,
  Plus,
  Key,
  Trash2,
  Lock,
  Unlock,
  UserCheck,
} from 'lucide-react'
import { api } from '../api'
import type { UserView } from '../types'
import { MdCard } from '../components/md3/MdCard'
import { MdButton } from '../components/md3/MdButton'
import { MdTextField } from '../components/md3/MdTextField'
import { MdDialog } from '../components/md3/MdDialog'
import { UserTokenModal } from '../components/UserTokenModal'
import { BangumiModal } from '../components/BangumiModal'

interface UsersViewProps {
  users: UserView[]
  onRefresh: () => void
}

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: {
      staggerChildren: 0.06,
    },
  },
}

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 12 },
  show: {
    opacity: 1,
    y: 0,
    transition: { type: 'spring', stiffness: 350, damping: 25 },
  },
}

export const UsersView: React.FC<UsersViewProps> = ({ users, onRefresh }) => {
  const [selectedUserForToken, setSelectedUserForToken] = useState<UserView | null>(null)
  const [selectedUserForBgm, setSelectedUserForBgm] = useState<UserView | null>(null)
  const [selectedUserForPassword, setSelectedUserForPassword] = useState<UserView | null>(null)
  const [isAddUserOpen, setIsAddUserOpen] = useState(false)

  // Edit Password Form State
  const [userNewPassword, setUserNewPassword] = useState('')
  const [updatingPassword, setUpdatingPassword] = useState(false)
  const [updatePasswordError, setUpdatePasswordError] = useState<string | null>(null)

  // Add User Form State
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newIsAdmin, setNewIsAdmin] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  const handleUpdatePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedUserForPassword) return
    setUpdatingPassword(true)
    setUpdatePasswordError(null)

    try {
      await api.setUserPassword(selectedUserForPassword.id, userNewPassword)
      setSelectedUserForPassword(null)
      setUserNewPassword('')
      onRefresh()
    } catch (err: any) {
      setUpdatePasswordError(err.message || '更新密码失败')
    } finally {
      setUpdatingPassword(false)
    }
  }

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newUsername.trim()) return
    setCreating(true)
    setCreateError(null)

    try {
      await api.createUser(newUsername.trim(), newPassword.trim() || undefined, newIsAdmin)
      setNewUsername('')
      setNewPassword('')
      setNewIsAdmin(false)
      setIsAddUserOpen(false)
      onRefresh()
    } catch (err: any) {
      setCreateError(err.message || '创建用户失败')
    } finally {
      setCreating(false)
    }
  }

  const handleDeleteUser = async (u: UserView) => {
    if (u.id === 'admin') {
      alert('无法删除默认系统管理员')
      return
    }
    if (!confirm(`确定要删除用户 "${u.name}" 及其所有令牌和播放记录吗？`)) return
    try {
      await api.deleteUser(u.id)
      onRefresh()
    } catch (err: any) {
      alert(err.message || '删除失败')
    }
  }

  return (
    <motion.div
      variants={containerVariants}
      initial="hidden"
      animate="show"
      className="flex flex-col gap-6 pb-12"
    >
      {/* Header & Add User Action */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h3 className="text-lg font-bold text-[var(--md-on-surface)] flex items-center gap-2">
            <Users className="w-5 h-5 text-[var(--md-primary)]" />
            <span>Emby 多用户与独立追番隔离</span>
          </h3>
          <p className="text-xs text-[var(--md-on-surface-variant)] mt-0.5">
            每个用户拥有独立的播放进度、断点续播、个人 Bangumi 收藏与客户端直连令牌
          </p>
        </div>

        <MdButton
          variant="filled"
          size="md"
          onClick={() => setIsAddUserOpen(true)}
          icon={<Plus className="w-4 h-4" />}
          className="self-start sm:self-auto"
        >
          创建新用户
        </MdButton>
      </div>

      {/* Users Cards Grid */}
      <motion.div variants={containerVariants} className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {users.map((u) => (
          <motion.div key={u.id} variants={itemVariants}>
            <MdCard
              variant="filled"
              className="p-6 flex flex-col justify-between h-full"
            >
            <div>
              {/* User Card Header */}
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3.5">
                  <div className="w-12 h-12 rounded-2xl bg-gradient-to-tr from-[var(--md-primary)] to-[var(--md-tertiary)] flex items-center justify-center font-bold text-white text-lg shadow-md shadow-[var(--md-primary)]/20">
                    {u.name.slice(0, 1).toUpperCase()}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-base text-[var(--md-on-surface)]">{u.name}</span>
                      {u.isAdmin ? (
                        <span className="px-2.5 py-0.5 text-[10px] font-semibold bg-[var(--md-primary-container)] text-[var(--md-on-primary-container)] rounded-full">
                          管理员
                        </span>
                      ) : (
                        <span className="px-2.5 py-0.5 text-[10px] font-semibold bg-[var(--md-surface-container-high)] text-[var(--md-on-surface-variant)] rounded-full">
                          普通用户
                        </span>
                      )}
                    </div>
                    <div className="text-xs font-mono text-[var(--md-on-surface-variant)] mt-0.5">ID: {u.id}</div>
                  </div>
                </div>

                {u.id !== 'admin' && (
                  <button
                    onClick={() => handleDeleteUser(u)}
                    className="p-1.5 text-[var(--md-on-surface-variant)] hover:text-[var(--md-danger)] hover:bg-[var(--md-danger-container)]/30 rounded-xl transition-colors cursor-pointer"
                    title="删除用户"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                )}
              </div>

              {/* Status List */}
              <div className="flex flex-col gap-2 mt-5 text-xs">
                {/* Password status */}
                <div className="p-3 bg-[var(--md-surface-container)] rounded-2xl border border-[var(--md-outline-variant)]/50 flex items-center justify-between">
                  <span className="text-[var(--md-on-surface-variant)] flex items-center gap-1.5">
                    {u.hasPassword ? <Lock className="w-3.5 h-3.5 text-amber-500" /> : <Unlock className="w-3.5 h-3.5 text-[var(--md-on-surface-variant)]" />}
                    <span>密码认证</span>
                  </span>
                  <button
                    onClick={() => {
                      setSelectedUserForPassword(u)
                      setUserNewPassword('')
                      setUpdatePasswordError(null)
                    }}
                    className={`font-semibold hover:underline cursor-pointer flex items-center gap-1 ${
                      u.hasPassword ? 'text-amber-500' : 'text-[var(--md-primary)]'
                    }`}
                  >
                    <span>{u.hasPassword ? '已设置密码' : '免密直接登录'}</span>
                    <span className="text-[10px] opacity-75 font-normal">({u.hasPassword ? '修改' : '去设置'})</span>
                  </button>
                </div>

                {/* Bangumi Bind Status */}
                <div className="p-3 bg-[var(--md-surface-container)] rounded-2xl border border-[var(--md-outline-variant)]/50 flex items-center justify-between">
                  <span className="text-[var(--md-on-surface-variant)] flex items-center gap-1.5">
                    <span>🌸</span>
                    <span>Bangumi 追番同步</span>
                  </span>
                  <button
                    onClick={() => setSelectedUserForBgm(u)}
                    className={`font-semibold hover:underline cursor-pointer ${
                      u.hasBgmToken ? 'text-pink-500' : 'text-[var(--md-primary)]'
                    }`}
                  >
                    {u.hasBgmToken ? `@${u.bgmUserId || '已绑定'}` : '+ 关联账号'}
                  </button>
                </div>

                {/* Tokens Count */}
                <div className="p-3 bg-[var(--md-surface-container)] rounded-2xl border border-[var(--md-outline-variant)]/50 flex items-center justify-between">
                  <span className="text-[var(--md-on-surface-variant)] flex items-center gap-1.5">
                    <Key className="w-3.5 h-3.5 text-emerald-500" />
                    <span>活跃设备令牌 (Tokens)</span>
                  </span>
                  <span className="text-emerald-500 font-semibold">{u.tokenCount} 个已授权</span>
                </div>
              </div>
            </div>

            {/* Bottom Actions */}
            <div className="mt-5 pt-3 border-t border-[var(--md-outline-variant)]/60 flex items-center gap-2">
              <MdButton
                variant="tonal"
                size="sm"
                onClick={() => setSelectedUserForToken(u)}
                icon={<Key className="w-3.5 h-3.5" />}
                className="flex-1"
              >
                设备令牌管理
              </MdButton>

              <MdButton
                variant="outlined"
                size="sm"
                onClick={() => setSelectedUserForBgm(u)}
              >
                Bangumi
              </MdButton>
            </div>
          </MdCard>
        </motion.div>
      ))}
    </motion.div>

      {/* Add User Dialog */}
      <MdDialog
        open={isAddUserOpen}
        onClose={() => setIsAddUserOpen(false)}
        title="创建新 Emby 用户"
        subtitle="为家庭成员或独立设备分配专属账号与访问权限。"
        icon={<UserCheck className="w-5 h-5 text-[var(--md-primary)]" />}
      >
        <form onSubmit={handleCreateUser} className="flex flex-col gap-4">
          {createError && (
            <div className="p-3 rounded-2xl bg-[var(--md-danger-container)] text-[var(--md-on-danger-container)] text-xs font-semibold">
              {createError}
            </div>
          )}

          <MdTextField
            label="用户名 (Username)"
            placeholder="例如: LivingRoom, Alice, Bob"
            value={newUsername}
            onChange={(e) => setNewUsername(e.target.value)}
            required
          />

          <MdTextField
            label="密码 (Password - 可选)"
            type="password"
            placeholder="留空则为免密便捷登录"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
          />

          <label className="flex items-center gap-2.5 px-1 text-xs text-[var(--md-on-surface)] cursor-pointer select-none">
            <input
              type="checkbox"
              checked={newIsAdmin}
              onChange={(e) => setNewIsAdmin(e.target.checked)}
              className="w-4 h-4 rounded text-[var(--md-primary)] focus:ring-0"
            />
            <span>赋予管理员权限 (可管理其他住户与规则)</span>
          </label>

          <div className="flex justify-end gap-2 pt-3 border-t border-[var(--md-outline-variant)]/60">
            <MdButton type="button" variant="text" onClick={() => setIsAddUserOpen(false)}>
              取消
            </MdButton>
            <MdButton type="submit" variant="filled" loading={creating}>
              确认创建
            </MdButton>
          </div>
        </form>
      </MdDialog>

      {/* Edit Password Dialog */}
      <MdDialog
        open={!!selectedUserForPassword}
        onClose={() => setSelectedUserForPassword(null)}
        title={`设置/修改密码: ${selectedUserForPassword?.name || ''}`}
        subtitle="为该用户更新 Emby 客户端与网页端的登录密码。留空并保存则重置为免密直接登录。"
        icon={<Lock className="w-5 h-5 text-amber-500" />}
      >
        <form onSubmit={handleUpdatePassword} className="flex flex-col gap-4">
          {updatePasswordError && (
            <div className="p-3 rounded-2xl bg-[var(--md-danger-container)] text-[var(--md-on-danger-container)] text-xs font-semibold">
              {updatePasswordError}
            </div>
          )}

          <MdTextField
            label="新密码 (Password)"
            type="password"
            placeholder="输入新密码，留空则免密"
            value={userNewPassword}
            onChange={(e) => setUserNewPassword(e.target.value)}
            autoFocus
          />

          <div className="flex justify-end gap-2 pt-3 border-t border-[var(--md-outline-variant)]/60">
            <MdButton type="button" variant="text" onClick={() => setSelectedUserForPassword(null)}>
              取消
            </MdButton>
            <MdButton type="submit" variant="filled" loading={updatingPassword}>
              保存密码
            </MdButton>
          </div>
        </form>
      </MdDialog>

      {/* User Token Modal */}
      <UserTokenModal
        user={selectedUserForToken}
        isOpen={!!selectedUserForToken}
        onClose={() => {
          setSelectedUserForToken(null)
          onRefresh()
        }}
      />

      {/* Bangumi Modal */}
      <BangumiModal
        user={selectedUserForBgm}
        isOpen={!!selectedUserForBgm}
        onClose={() => setSelectedUserForBgm(null)}
        onSuccess={onRefresh}
      />
    </motion.div>
  )
}
