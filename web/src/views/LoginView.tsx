import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Lock, User, ShieldAlert, ArrowRight } from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import { MdCard } from '../components/md3/MdCard'
import { MdButton } from '../components/md3/MdButton'
import { MdTextField } from '../components/md3/MdTextField'
import { AkariLogo } from '../components/md3/AkariLogo'

export const LoginView: React.FC = () => {
  const { login } = useAuth()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password) {
      setError('请输入管理员用户名和密码')
      return
    }

    setLoading(true)
    setError(null)

    try {
      await login(username.trim(), password)
    } catch (err: any) {
      setError(err.message || '登录失败，请检查密码')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-[var(--md-surface)] relative overflow-hidden select-none">
      {/* Dynamic Ambient Background Glows */}
      <motion.div
        animate={{
          scale: [1, 1.15, 1],
          opacity: [0.15, 0.25, 0.15],
        }}
        transition={{
          duration: 8,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
        className="absolute -top-40 -left-40 w-96 h-96 bg-[var(--md-primary)] rounded-full blur-3xl pointer-events-none"
      />
      <motion.div
        animate={{
          scale: [1, 1.2, 1],
          opacity: [0.12, 0.22, 0.12],
        }}
        transition={{
          duration: 10,
          repeat: Infinity,
          ease: 'easeInOut',
          delay: 1,
        }}
        className="absolute -bottom-40 -right-40 w-96 h-96 bg-[var(--md-tertiary)] rounded-full blur-3xl pointer-events-none"
      />

      {/* Expressive Login Card */}
      <motion.div
        initial={{ opacity: 0, scale: 0.94, y: 20 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        transition={{ type: 'spring', stiffness: 300, damping: 25 }}
        className="w-full max-w-md relative z-10"
      >
        <MdCard variant="elevated" className="p-8 sm:p-10 shadow-2xl border border-[var(--md-outline-variant)]">
          {/* Brand Header */}
          <div className="flex flex-col items-center text-center mb-8">
            <AkariLogo size={64} variant="badge" interactive={true} className="mb-4 shadow-xl shadow-[var(--md-primary)]/25 rounded-3xl" />
            <h1 className="text-2xl font-bold text-[var(--md-on-surface)] tracking-tight">
              Akari Media Bridge
            </h1>
            <p className="text-xs font-medium text-[var(--md-on-surface-variant)] mt-1.5">
              Material Design 3 Expressive 控制台
            </p>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <AnimatePresence>
              {error && (
                <motion.div
                  initial={{ opacity: 0, y: -10, scale: 0.95 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -10, scale: 0.95 }}
                  className="flex items-center gap-2.5 p-3.5 rounded-2xl bg-[var(--md-danger-container)] text-[var(--md-on-danger-container)] text-xs font-medium border border-[var(--md-danger)]/20 shadow-sm"
                >
                  <ShieldAlert className="w-4 h-4 shrink-0" />
                  <span>{error}</span>
                </motion.div>
              )}
            </AnimatePresence>

            <MdTextField
              label="管理员账号"
              placeholder="请输入管理员用户名"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              leadingIcon={<User className="w-4 h-4" />}
              autoComplete="username"
              required
            />

            <MdTextField
              label="访问密码"
              type="password"
              placeholder="请输入管理员密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              leadingIcon={<Lock className="w-4 h-4" />}
              autoComplete="current-password"
              required
            />

            <div className="mt-2">
              <MdButton
                type="submit"
                variant="filled"
                size="lg"
                loading={loading}
                className="w-full"
                icon={<ArrowRight className="w-4 h-4" />}
              >
                立即登录
              </MdButton>
            </div>
          </form>

          {/* Security Notice */}
          <div className="mt-8 pt-6 border-t border-[var(--md-outline-variant)]/60 text-center">
            <span className="text-[11px] text-[var(--md-on-surface-variant)] leading-relaxed">
              受防暴力破解与 JWT 签名安全保护 · 连续输错 5 次将自动锁定 IP
            </span>
          </div>
        </MdCard>
      </motion.div>
    </div>
  )
}
