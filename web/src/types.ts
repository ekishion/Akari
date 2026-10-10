export interface SystemStatus {
  serverName: string
  serverId: string
  version: string
  serverUrl: string
  httpPort: number
  udpPort: number
  uptimeSeconds: number
  activeRules: number
  totalRules: number
  goVersion: string
  os: string
  arch: string
  numGoroutines: number
  memoryAllocMB: number
  memorySysMB: number
  dbStats: {
    userCount: number
    tokenCount: number
    historyCount: number
    dbSizeBytes: number
  }
  singleUserMode: boolean
}

export interface SystemConfig {
  serverName: string
  serverId: string
  httpPort: number
  udpPort: number
  dataDir: string
  dandanHost: string
  bangumiHost: string
  bangumiImageHost?: string
  enableECH?: boolean
  customProxy: string
  hasPassword: boolean
}

export interface UserView {
  id: string
  name: string
  isAdmin: boolean
  isDisabled: boolean
  hasPassword: boolean
  bgmUserId: string
  hasBgmToken: boolean
  lastLoginDate: string | null
  tokenCount: number
}

export interface UserToken {
  token: string
  userId: string
  clientName: string
  deviceId: string
  createdAt: string
  expiresAt?: string
}

export interface RulePlugin {
  id: string
  name: string
  version: string
  type: string
  enabled: boolean
  multiSources?: boolean
  useNativePlayer?: boolean
  baseURL: string
  searchURL?: string
  searchMode?: string
  chapterMode?: string
  referer?: string
  userAgent?: string
  adBlocker?: boolean
}


export interface TestResult {
  success: boolean
  error?: string
  resultsCount?: number
  results?: Array<{
    name: string
    dramaUrl: string
    coverUrl?: string
    status?: string
  }>
  sampleDrama?: string
  chapters?: Array<{
    name: string
    episodes: Array<{
      name: string
      url: string
    }>
  }>
  chapterError?: string
}

export interface MirrorTestResult {
  success: boolean
  latencyMs?: number
  error?: string
  message?: string
}

export interface GlobalSynonym {
  id: number
  pattern: string
  replacement: string
  enabled: boolean
  createdAt: string
}

export interface SubjectAlias {
  subjectId: number
  title: string
  aliases: string[]
  updatedAt: string
}

export interface AdminAuthResponse {
  token: string
  expiresAt: string
  username: string
}

export interface AdminProfile {
  username: string
  role: string
}

export interface AuditLog {
  id: number
  eventType: string
  ipAddress: string
  userAgent: string
  details: string
  createdAt: string
}

export interface IPBan {
  ip: string
  reason: string
  bannedUntil?: string
  failedAttempts: number
  updatedAt: string
}

export interface TelemetryEvent {
  timestamp: string
  uptimeSeconds: number
  goroutines: number
  memoryAllocMB: number
  memorySysMB: number
  activeSessions: number
  totalRules: number
  enabledRules: number
  recentLog?: string
}

export interface BilibiliStatus {
  is_login: boolean
  mid: number
  uname: string
  face: string
  is_vip: boolean
  vip_due_date: number
  max_quality: number
  quality_desc: string
  stream_mode?: string
  enabled: boolean
  prefer: boolean
}

export interface BilibiliConfigPayload {
  sessdata?: string
  bili_jct?: string
  buvid3?: string
  dede_user_id?: string
  enabled?: boolean
  prefer_bilibili?: boolean
  max_quality?: number
  stream_mode?: string
}

export interface BilibiliQRGenerateResponse {
  code: number
  message: string
  data: {
    url: string
    qrcode_key: string
  }
}

export interface BilibiliQRPollResponse {
  poll: {
    code: number
    message: string
    data: {
      url: string
      refresh_token: string
      code: number
      message: string
    }
  }
  is_success: boolean
}



