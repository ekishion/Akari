import React, { useState } from 'react'
import { History, Search, Clock, PlayCircle, User } from 'lucide-react'
import type { PlaybackHistory } from '../types'

interface HistoryViewProps {
  history: PlaybackHistory[]
}

export const HistoryView: React.FC<HistoryViewProps> = ({ history }) => {
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedUser, setSelectedUser] = useState<string>('all')

  const usersList = Array.from(new Set(history.map((h) => h.userId)))

  const filtered = history.filter((h) => {
    const matchSearch = h.itemId.toLowerCase().includes(searchTerm.toLowerCase())
    const matchUser = selectedUser === 'all' || h.userId === selectedUser
    return matchSearch && matchUser
  })

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h2 className="text-xl font-bold text-white tracking-tight flex items-center space-x-2">
          <History className="w-5 h-5 text-indigo-400" />
          <span>全站播放与进度记录</span>
        </h2>
        <p className="text-xs text-slate-400 mt-0.5">
          记录用户在 Infuse、VidHub、Web 等客户端播放的断点进度、观看次数和最后观看时间
        </p>
      </div>

      {/* Filter Controls */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="搜索条目 ID (例如: bgm_sub_410943)..."
            className="w-full pl-10 pr-4 py-2 bg-slate-900/80 border border-slate-800 rounded-xl text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-indigo-500"
          />
        </div>

        {usersList.length > 1 && (
          <div className="flex items-center space-x-2 bg-slate-900/80 border border-slate-800 rounded-xl px-3 py-1.5">
            <User className="w-4 h-4 text-slate-400" />
            <select
              value={selectedUser}
              onChange={(e) => setSelectedUser(e.target.value)}
              className="bg-transparent text-xs text-slate-200 focus:outline-none cursor-pointer"
            >
              <option value="all" className="bg-slate-900">
                所有用户
              </option>
              {usersList.map((u) => (
                <option key={u} value={u} className="bg-slate-900">
                  {u}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      {/* History Table / Cards */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-3xl overflow-hidden shadow-xl">
        {filtered.length === 0 ? (
          <div className="text-center py-20 text-slate-500">
            <History className="w-10 h-10 mx-auto mb-2 opacity-30" />
            <p className="text-sm">暂无匹配的播放历史数据</p>
          </div>
        ) : (
          <div className="divide-y divide-slate-800/60">
            {filtered.map((h, idx) => {
              const pct =
                h.totalTicks > 0 ? Math.min(100, Math.round((h.positionTicks / h.totalTicks) * 100)) : 0
              return (
                <div
                  key={idx}
                  className="p-4 hover:bg-slate-800/30 transition-colors flex flex-col sm:flex-row sm:items-center justify-between gap-4"
                >
                  <div className="flex items-center space-x-3.5 min-w-0 flex-1">
                    <div className="w-10 h-10 rounded-xl bg-indigo-500/10 text-indigo-400 flex items-center justify-center shrink-0">
                      <PlayCircle className="w-5 h-5" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center space-x-2">
                        <span className="font-semibold text-sm text-slate-200 truncate">{h.itemId}</span>
                        {h.played ? (
                          <span className="px-2 py-0.5 text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 rounded-full">
                            已看完整集
                          </span>
                        ) : (
                          <span className="px-2 py-0.5 text-[10px] bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 rounded-full">
                            播放中 ({pct}%)
                          </span>
                        )}
                      </div>
                      <div className="text-xs text-slate-400 mt-1 flex items-center space-x-3">
                        <span>播放用户: {h.userId}</span>
                        <span>播放次数: {h.playCount} 次</span>
                      </div>
                    </div>
                  </div>

                  {/* Progress and Time */}
                  <div className="flex items-center space-x-6 shrink-0 sm:self-center">
                    <div className="w-32 hidden md:block">
                      <div className="h-1.5 bg-slate-800 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-indigo-500 rounded-full"
                          style={{ width: `${pct}%` }}
                        />
                      </div>
                    </div>

                    <div className="text-right text-xs text-slate-400 flex items-center space-x-1.5">
                      <Clock className="w-3.5 h-3.5 text-slate-500" />
                      <span>
                        {h.lastPlayedDate
                          ? new Date(h.lastPlayedDate).toLocaleString()
                          : '未知时间'}
                      </span>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
