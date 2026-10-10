import React, { useState, useEffect, useRef } from 'react'
import { QrCode, RefreshCw, CheckCircle2, AlertCircle, Smartphone, ShieldCheck } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { api } from '../api'
import { MdDialog } from './md3/MdDialog'
import { MdButton } from './md3/MdButton'

interface BilibiliQrModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export const BilibiliQrModal: React.FC<BilibiliQrModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [qrUrl, setQrUrl] = useState<string>('')
  const [qrKey, setQrKey] = useState<string>('')
  const [statusMsg, setStatusMsg] = useState<string>('请使用哔哩哔哩手机客户端扫码登录')
  const [statusCode, setStatusCode] = useState<number>(86101) // 86101: waiting, 86090: scanned waiting confirm, 0: success, 86038: expired
  const [isSuccess, setIsSuccess] = useState<boolean>(false)
  const pollTimerRef = useRef<any>(null)

  const fetchQRCode = async () => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
    setQrKey('')
    setQrUrl('')
    setError(null)
    setLoading(true)
    setIsSuccess(false)
    setStatusCode(86101)
    setStatusMsg('正在生成登录二维码...')

    try {
      const res = await api.generateBilibiliQR()
      if (res && res.data && res.data.qrcode_key) {
        setQrUrl(res.data.url)
        setQrKey(res.data.qrcode_key)
        setStatusMsg('请使用哔哩哔哩 App 扫一扫')
      } else {
        throw new Error(res.message || '获取二维码失败')
      }
    } catch (err: any) {
      setError(err.message || '获取二维码失败，请检查网络')
      setStatusMsg('二维码加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      fetchQRCode()
    } else {
      if (pollTimerRef.current) {
        clearInterval(pollTimerRef.current)
        pollTimerRef.current = null
      }
      setQrUrl('')
      setQrKey('')
      setIsSuccess(false)
      setError(null)
    }

    return () => {
      if (pollTimerRef.current) {
        clearInterval(pollTimerRef.current)
        pollTimerRef.current = null
      }
    }
  }, [isOpen])

  // Polling loop
  useEffect(() => {
    if (!isOpen || !qrKey || isSuccess) {
      if (pollTimerRef.current) {
        clearInterval(pollTimerRef.current)
        pollTimerRef.current = null
      }
      return
    }

    const currentKey = qrKey
    pollTimerRef.current = setInterval(async () => {
      try {
        const res = await api.pollBilibiliQR(currentKey)
        const code = res?.poll?.data?.code ?? res?.poll?.code ?? 86101

        setStatusCode(code)

        if (code === 0 || res.is_success) {
          // Success!
          setIsSuccess(true)
          setStatusMsg('登录成功！已自动保存凭据')
          if (pollTimerRef.current) {
            clearInterval(pollTimerRef.current)
            pollTimerRef.current = null
          }
          onSuccess()
          setTimeout(() => {
            onClose()
          }, 1000)
        } else if (code === 86090) {
          setStatusMsg('已扫码，请在手机客户端点击【确认登录】')
        } else if (code === 86038) {
          setStatusMsg('二维码已失效，请点击下方按钮刷新')
          if (pollTimerRef.current) {
            clearInterval(pollTimerRef.current)
            pollTimerRef.current = null
          }
        } else {
          setStatusMsg('请使用哔哩哔哩 App 扫一扫')
        }
      } catch (err) {
        console.error('Polling QR error:', err)
      }
    }, 1500)

    return () => {
      if (pollTimerRef.current) {
        clearInterval(pollTimerRef.current)
        pollTimerRef.current = null
      }
    }
  }, [isOpen, qrKey, isSuccess])

  return (
    <MdDialog
      open={isOpen}
      onClose={onClose}
      title="哔哩哔哩 扫码登录"
      subtitle="无需手动提取 SESSDATA，使用哔哩哔哩官方手机 App 扫码即可快捷授权登录并获取高清及大会员播放权限。"
      icon={<QrCode className="w-5 h-5 text-[var(--md-primary)]" />}
      maxWidth="md"
      actions={
        <>
          <MdButton variant="text" onClick={onClose}>
            关闭
          </MdButton>
          {statusCode === 86038 && (
            <MdButton variant="filled" onClick={fetchQRCode} loading={loading}>
              刷新二维码
            </MdButton>
          )}
        </>
      }
    >
      <div className="flex flex-col items-center justify-center p-4">
        {/* QR Box Container */}
        <div className="relative w-64 h-64 bg-white rounded-2xl p-3 shadow-md flex items-center justify-center border border-[var(--md-outline-variant)]">
          {loading ? (
            <div className="flex flex-col items-center justify-center gap-2 text-slate-500">
              <RefreshCw className="w-8 h-8 animate-spin text-[var(--md-primary)]" />
              <span className="text-xs">加载中...</span>
            </div>
          ) : isSuccess ? (
            <div className="flex flex-col items-center justify-center gap-3 text-emerald-600 animate-in fade-in zoom-in duration-300">
              <div className="w-16 h-16 rounded-full bg-emerald-100 flex items-center justify-center">
                <CheckCircle2 className="w-10 h-10 text-emerald-600" />
              </div>
              <span className="font-semibold text-base text-slate-800">登录成功！</span>
            </div>
          ) : qrUrl ? (
            <div className="relative w-full h-full flex flex-col items-center justify-center">
              <QRCodeSVG
                value={qrUrl}
                size={220}
                level="M"
                includeMargin={true}
                className={`w-full h-full object-contain rounded-lg transition-opacity duration-300 ${
                  statusCode === 86038 ? 'opacity-20 blur-sm' : 'opacity-100'
                }`}
              />

              {statusCode === 86090 && (
                <div className="absolute inset-0 bg-white/90 backdrop-blur-xs rounded-lg flex flex-col items-center justify-center gap-2 text-[var(--md-primary)]">
                  <Smartphone className="w-10 h-10 animate-bounce" />
                  <span className="text-xs font-semibold px-2 text-center text-slate-800">
                    已扫码，请在手机上确认
                  </span>
                </div>
              )}

              {statusCode === 86038 && (
                <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-slate-700">
                  <AlertCircle className="w-8 h-8 text-amber-500" />
                  <span className="text-xs font-medium">二维码已失效</span>
                  <button
                    type="button"
                    onClick={fetchQRCode}
                    className="mt-1 px-3 py-1 bg-[var(--md-primary)] text-white text-xs rounded-full font-medium shadow-sm hover:opacity-90 transition-opacity"
                  >
                    点击刷新
                  </button>
                </div>
              )}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center gap-2 text-rose-500">
              <AlertCircle className="w-8 h-8" />
              <span className="text-xs">{error || '加载失败'}</span>
              <button
                type="button"
                onClick={fetchQRCode}
                className="mt-1 text-xs text-[var(--md-primary)] underline"
              >
                重试
              </button>
            </div>
          )}
        </div>

        {/* Status Message */}
        <div className="mt-4 flex items-center gap-2 text-sm font-medium text-[var(--md-on-surface)]">
          {isSuccess ? (
            <span className="text-emerald-600 flex items-center gap-1.5">
              <CheckCircle2 className="w-4 h-4" />
              {statusMsg}
            </span>
          ) : statusCode === 86090 ? (
            <span className="text-amber-600 flex items-center gap-1.5 animate-pulse">
              <Smartphone className="w-4 h-4" />
              {statusMsg}
            </span>
          ) : statusCode === 86038 ? (
            <span className="text-slate-500 flex items-center gap-1.5">
              <AlertCircle className="w-4 h-4 text-amber-500" />
              {statusMsg}
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-slate-600">
              <Smartphone className="w-4 h-4 text-slate-400" />
              {statusMsg}
            </span>
          )}
        </div>

        {/* Security / Privacy Hint */}
        <div className="mt-4 p-3 rounded-xl bg-[var(--md-surface-container-high)]/60 border border-[var(--md-outline-variant)]/50 text-xs text-[var(--md-on-surface-variant)] flex items-start gap-2 max-w-md">
          <ShieldCheck className="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
          <span>
            凭据加密存储于本地 SQLite 数据库中（AES-256-GCM 保护），仅用于直连 B 站 API 获取视频/番剧高清播放流，绝不上报或外泄。
          </span>
        </div>
      </div>
    </MdDialog>
  )
}
