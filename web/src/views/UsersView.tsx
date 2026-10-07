import React, { useState } from 'react'
import {
  Users,
  Plus,
  Key,
  Trash2,
  Lock,
  Unlock,
} from 'lucide-react'
import { api } from '../api'
import type { UserView } from '../types'
import { UserTokenModal } from '../components/UserTokenModal'
import { BangumiModal } from '../components/BangumiModal'

interface UsersViewProps {
  users: UserView[]
  onRefresh: () => void
}

export const UsersView: React.FC<UsersViewProps> = ({ users, onRefresh }) => {
  const [selectedUserForToken, setSelectedUserForToken] = useState<UserView | null>(null)
  const [selectedUserForBgm, setSelectedUserForBgm] = useState<UserView | null>(null)
  const [isAddUserOpen, setIsAddUserOpen] = useState(false)

  // Add User Form State
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newIsAdmin, setNewIsAdmin] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

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
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-white tracking-tight flex items-center space-x-2">
            <Users className="w-5 h-5 text-purple-400" />
            <span>用户与多住户隔离管理</span>
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">
            每个用户拥有独立的播放进度、断点续播、个人 Bangumi 追番收藏列表以及客户端直连 API 令牌
          </p>
        </div>

        <button
          onClick={() => setIsAddUserOpen(true)}
          className="px-4 py-2 bg-gradient-to-r from-purple-500 to-indigo-600 hover:from-purple-600 hover:to-indigo-700 text-white text-xs font-semibold rounded-xl flex items-center space-x-1.5 shadow-md shadow-purple-500/20 transition-all cursor-pointer self-start sm:self-auto"
        >
          <Plus className="w-4 h-4" />
          <span>创建新用户</span>
        </button>
      </div>

      {/* Users Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
        {users.map((u) => (
          <div
            key={u.id}
            className="p-6 bg-slate-900/70 border border-slate-800 hover:border-purple-500/30 rounded-3xl shadow-xl flex flex-col justify-between transition-all"
          >
            <div>
              {/* User Header */}
              <div className="flex items-start justify-between">
                <div className="flex items-center space-x-3">
                  <div className="w-12 h-12 rounded-2xl bg-gradient-to-tr from-purple-600 to-pink-500 flex items-center justify-center font-bold text-white text-lg shadow-lg shadow-purple-500/20">
                    {u.name.slice(0, 1).toUpperCase()}
                  </div>
                  <div>
                    <div className="flex items-center space-x-2">
                      <span className="font-bold text-base text-white">{u.name}</span>
                      {u.isAdmin ? (
                        <span className="px-2 py-0.5 text-[10px] font-semibold bg-purple-500/10 text-purple-400 border border-purple-500/20 rounded-full">
                          管理员
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 text-[10px] font-semibold bg-slate-800 text-slate-400 rounded-full">
                          标准用户
                        </span>
                      )}
                    </div>
                    <div className="text-xs font-mono text-slate-400 mt-0.5">ID: {u.id}</div>
                  </div>
                </div>

                {u.id !== 'admin' && (
                  <button
                    onClick={() => handleDeleteUser(u)}
                    className="p-1.5 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-colors cursor-pointer"
                    title="删除用户"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                )}
              </div>

              {/* Status Pills */}
              <div className="space-y-2 mt-5 text-xs">
                {/* Password status */}
                <div className="p-2.5 bg-slate-950/80 rounded-xl border border-slate-800/80 flex items-center justify-between">
                  <span className="text-slate-400 flex items-center space-x-1.5">
                    {u.hasPassword ? <Lock className="w-3.5 h-3.5 text-amber-400" /> : <Unlock className="w-3.5 h-3.5 text-slate-500" />}
                    <span>密码认证</span>
                  </span>
                  <span className={u.hasPassword ? 'text-amber-400 font-medium' : 'text-slate-500'}>
                    {u.hasPassword ? '已设置密码' : '免密直接登录'}
                  </span>
                </div>

                {/* Bangumi Bind Status */}
                <div className="p-2.5 bg-slate-950/80 rounded-xl border border-slate-800/80 flex items-center justify-between">
                  <span className="text-slate-400 flex items-center space-x-1.5">
                    <span className="text-pink-400">🌸</span>
                    <span>Bangumi 关联</span>
                  </span>
                  <button
                    onClick={() => setSelectedUserForBgm(u)}
                    className={`font-medium hover:underline cursor-pointer ${
                      u.hasBgmToken ? 'text-pink-400' : 'text-slate-500 hover:text-pink-300'
                    }`}
                  >
                    {u.hasBgmToken ? `@${u.bgmUserId || '已绑定'}` : '+ 绑定账号'}
                  </button>
                </div>

                {/* Tokens Count */}
                <div className="p-2.5 bg-slate-950/80 rounded-xl border border-slate-800/80 flex items-center justify-between">
                  <span className="text-slate-400 flex items-center space-x-1.5">
                    <Key className="w-3.5 h-3.5 text-emerald-400" />
                    <span>客户端令牌 (Tokens)</span>
                  </span>
                  <span className="text-emerald-400 font-medium">{u.tokenCount} 个活跃</span>
                </div>
              </div>
            </div>

            {/* Bottom Actions */}
            <div className="mt-5 pt-3 border-t border-slate-800 flex items-center gap-2">
              <button
                onClick={() => setSelectedUserForToken(u)}
                className="flex-1 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center justify-center space-x-1.5 transition-colors cursor-pointer"
              >
                <Key className="w-3.5 h-3.5 text-emerald-400" />
                <span>管理设备令牌</span>
              </button>

              <button
                onClick={() => setSelectedUserForBgm(u)}
                className="px-3 py-2 bg-pink-500/10 hover:bg-pink-500/20 text-pink-300 text-xs font-semibold rounded-xl flex items-center justify-center space-x-1 transition-colors cursor-pointer border border-pink-500/20"
              >
                <span>Bangumi</span>
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* Add User Modal */}
      {isAddUserOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-fade-in">
          <div className="bg-slate-900 border border-slate-800 w-full max-w-md rounded-2xl shadow-2xl overflow-hidden">
            <div className="px-6 py-4 border-b border-slate-800 bg-slate-950/50 flex items-center justify-between">
              <h3 className="text-base font-semibold text-white">创建新用户</h3>
              <button
                onClick={() => setIsAddUserOpen(false)}
                className="text-slate-400 hover:text-white"
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleCreateUser} className="p-6 space-y-4">
              {createError && (
                <div className="p-3 bg-red-500/10 text-red-400 text-xs rounded-xl">{createError}</div>
              )}

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                  用户名 (Username)
                </label>
                <input
                  type="text"
                  required
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  placeholder="例如: Alice, Bob, LivingRoom"
                  className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 focus:outline-none focus:border-purple-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                  密码 (Password - 可选)
                </label>
                <input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="留空则为免密模式"
                  className="w-full px-4 py-2 bg-slate-950 border border-slate-700 rounded-xl text-sm text-slate-100 focus:outline-none focus:border-purple-500"
                />
              </div>

              <div className="flex items-center space-x-2 pt-2">
                <input
                  type="checkbox"
                  id="isAdminCheckbox"
                  checked={newIsAdmin}
                  onChange={(e) => setNewIsAdmin(e.target.checked)}
                  className="w-4 h-4 rounded bg-slate-950 border-slate-700 text-purple-600 focus:ring-0"
                />
                <label htmlFor="isAdminCheckbox" className="text-xs font-medium text-slate-300 cursor-pointer">
                  赋予系统管理员权限 (可管理规则与所有用户)
                </label>
              </div>

              <div className="flex justify-end space-x-3 pt-4 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setIsAddUserOpen(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-medium rounded-xl cursor-pointer"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="px-5 py-2 bg-gradient-to-r from-purple-500 to-indigo-600 hover:from-purple-600 text-white text-sm font-medium rounded-xl shadow-md shadow-purple-500/20 disabled:opacity-50 cursor-pointer"
                >
                  {creating ? '创建中...' : '确认创建'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

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
    </div>
  )
}
