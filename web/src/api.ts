import type {
  SystemStatus,
  SystemConfig,
  UserView,
  UserToken,
  RulePlugin,
  TestResult,
  MirrorTestResult,
  GlobalSynonym,
  SubjectAlias,
  AdminAuthResponse,
  AdminProfile,
  AuditLog,
  IPBan,
  TelemetryEvent,
} from './types'

const BASE_URL = ''

function getAuthHeaders(): Record<string, string> {
  const adminToken = localStorage.getItem('akari_admin_token')
  const userToken = localStorage.getItem('akari_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (adminToken) {
    headers['Authorization'] = `Bearer ${adminToken}`
  } else if (userToken) {
    headers['X-Emby-Token'] = userToken
  }
  return headers
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = {
    ...getAuthHeaders(),
    ...((options.headers as Record<string, string>) || {}),
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  })

  if (res.status === 401 && !path.startsWith('/api/auth/login')) {
    // If unauthorized, clear token and notify auth state
    localStorage.removeItem('akari_admin_token')
    window.dispatchEvent(new CustomEvent('akari_unauthorized'))
  }

  if (!res.ok) {
    let errMsg = `Request failed (${res.status})`
    try {
      const errJson = await res.json()
      if (errJson.error) errMsg = errJson.error
    } catch {
      // ignore
    }
    throw new Error(errMsg)
  }

  return res.json()
}

export const api = {
  // Admin Auth
  adminLogin: (username: string, password: string) =>
    request<AdminAuthResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  adminLogout: () =>
    request<{ message: string }>('/api/auth/logout', {
      method: 'POST',
    }),
  adminProfile: () => request<AdminProfile>('/api/auth/me'),
  adminChangePassword: (oldPassword: string, newPassword: string) =>
    request<{ message: string }>('/api/auth/change-password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),

  // Security & Audit
  getAuditLogs: (limit: number = 50, offset: number = 0) =>
    request<{ total: number; items: AuditLog[] }>(`/api/security/audit?limit=${limit}&offset=${offset}`),
  clearAuditLogs: () =>
    request<{ message: string }>('/api/security/audit', { method: 'DELETE' }),
  getBannedIPs: () => request<{ items: IPBan[] }>('/api/security/bans'),
  unbanIP: (ip: string) =>
    request<{ message: string }>('/api/security/unban', {
      method: 'POST',
      body: JSON.stringify({ ip }),
    }),

  // SSE Real-time Telemetry Stream
  subscribeTelemetry: (onData: (data: TelemetryEvent) => void, onError?: (err: any) => void): (() => void) => {
    const token = localStorage.getItem('akari_admin_token') || ''
    const url = `${BASE_URL}/api/events?token=${encodeURIComponent(token)}`
    const eventSource = new EventSource(url)

    eventSource.addEventListener('telemetry', (e: MessageEvent) => {
      try {
        const parsed = JSON.parse(e.data) as TelemetryEvent
        onData(parsed)
      } catch (err) {
        console.error('Failed to parse telemetry SSE:', err)
      }
    })

    eventSource.onerror = (e) => {
      if (onError) onError(e)
    }

    return () => {
      eventSource.close()
    }
  },

  // System
  getStatus: () => request<SystemStatus>('/api/system/status'),
  getConfig: () => request<SystemConfig>('/api/system/config'),
  updateConfig: (data: Partial<SystemConfig>) =>
    request<SystemConfig>('/api/system/config', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  testBangumiMirror: (url: string) =>
    request<MirrorTestResult>('/api/system/bangumi/test', {
      method: 'POST',
      body: JSON.stringify({ url }),
    }),
  cleanCache: () => request<{ status: string; message: string }>('/api/system/clean-cache', { method: 'POST' }),

  // Emby Users & Tokens
  listUsers: () => request<UserView[]>('/api/users'),
  createUser: (username: string, password?: string, isAdmin: boolean = false) =>
    request<any>('/api/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, isAdmin }),
    }),
  deleteUser: (id: string) => request<{ status: string }>(`/api/users/${id}`, { method: 'DELETE' }),
  setUserPassword: (userId: string, password: string) =>
    request<{ status: string; message: string }>(`/api/users/${userId}/password`, {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  getUserTokens: (userId: string) => request<UserToken[]>(`/api/users/${userId}/tokens`),
  createUserToken: (userId: string, clientName: string) =>
    request<{ token: string; userId: string; clientName: string }>(`/api/users/${userId}/tokens`, {
      method: 'POST',
      body: JSON.stringify({ clientName }),
    }),
  revokeUserToken: (userId: string, token: string) =>
    request<{ status: string }>(`/api/users/${userId}/tokens/${token}`, { method: 'DELETE' }),
  bindBangumi: (userId: string, accessToken: string) =>
    request<{ status: string; username: string; nickname: string; avatar: any }>(`/api/users/${userId}/bangumi`, {
      method: 'POST',
      body: JSON.stringify({ accessToken }),
    }),
  unbindBangumi: (userId: string) =>
    request<{ status: string }>(`/api/users/${userId}/bangumi`, { method: 'DELETE' }),

  // Rules & Plugins
  listRules: () => request<RulePlugin[]>('/api/rules'),
  getRule: (nameOrId: string) => request<any>(`/api/rules/${encodeURIComponent(nameOrId)}`),
  saveRule: (rule: any) =>
    request<RulePlugin>('/api/rules', {
      method: 'POST',
      body: JSON.stringify(rule),
    }),
  toggleRule: (name: string, enabled: boolean) =>
    request<{ status: string; name: string; enabled: boolean }>(`/api/rules/${encodeURIComponent(name)}/toggle`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    }),
  deleteRule: (name: string) =>
    request<{ status: string }>(`/api/rules/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  importRulesFromUrl: (url: string) =>
    request<{ status: string; importedCount: number; addedCount: number; updatedCount: number }>('/api/rules/import-url', {
      method: 'POST',
      body: JSON.stringify({ url }),
    }),
  updateAllRules: (url?: string) =>
    request<{ status: string; importedCount: number; addedCount: number; updatedCount: number }>('/api/rules/update-all', {
      method: 'POST',
      body: JSON.stringify({ url: url || '' }),
    }),
  testRule: (params: { plugin?: any; ruleName?: string; keyword: string }) =>
    request<TestResult>('/api/rules/test', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  // Aliases & Synonyms
  listGlobalSynonyms: () => request<GlobalSynonym[]>('/api/aliases/synonyms'),
  upsertGlobalSynonym: (pattern: string, replacement: string, enabled: boolean = true) =>
    request<{ success: boolean }>('/api/aliases/synonyms', {
      method: 'POST',
      body: JSON.stringify({ pattern, replacement, enabled }),
    }),
  deleteGlobalSynonym: (pattern: string) =>
    request<{ success: boolean }>(`/api/aliases/synonyms/${encodeURIComponent(pattern)}`, { method: 'DELETE' }),
  resetGlobalSynonyms: () =>
    request<{ success: boolean }>('/api/aliases/synonyms/reset', { method: 'POST' }),

  listSubjectAliases: () => request<SubjectAlias[]>('/api/aliases/subjects'),
  upsertSubjectAliases: (subjectId: number, title: string, aliases: string[]) =>
    request<{ success: boolean }>('/api/aliases/subjects', {
      method: 'POST',
      body: JSON.stringify({ subjectId, title, aliases }),
    }),
  deleteSubjectAliases: (subjectId: number) =>
    request<{ success: boolean }>(`/api/aliases/subjects/${subjectId}`, { method: 'DELETE' }),
}
