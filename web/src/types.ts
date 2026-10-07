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

export interface PlaybackHistory {
  userId: string
  itemId: string
  positionTicks: number
  totalTicks: number
  played: boolean
  playCount: number
  lastPlayedDate: string | null
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
