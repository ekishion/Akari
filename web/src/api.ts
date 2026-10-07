import type { SystemStatus, SystemConfig, UserView, UserToken, RulePlugin, PlaybackHistory, TestResult } from './types'

const BASE_URL = ''

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('akari_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers['X-Emby-Token'] = token
  }
  return headers
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = {
    ...getAuthHeaders(),
    ...(options.headers as Record<string, string> || {}),
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  })

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
  // System
  getStatus: () => request<SystemStatus>('/api/system/status'),
  getConfig: () => request<SystemConfig>('/api/system/config'),
  cleanCache: () => request<{ status: string; message: string }>('/api/system/clean-cache', { method: 'POST' }),

  // Auth
  login: (username: string, password?: string) =>
    request<{ token: string; user: any; serverId: string }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  getMe: () => request<any>('/api/auth/me'),
  changePassword: (userId: string, newPassword: string) =>
    request<{ status: string }>('/api/auth/change-password', {
      method: 'POST',
      body: JSON.stringify({ userId, newPassword }),
    }),

  // Users
  listUsers: () => request<UserView[]>('/api/users'),
  createUser: (username: string, password?: string, isAdmin: boolean = false) =>
    request<any>('/api/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, isAdmin }),
    }),
  deleteUser: (id: string) => request<{ status: string }>(`/api/users/${id}`, { method: 'DELETE' }),

  // User Tokens
  getUserTokens: (userId: string) => request<UserToken[]>(`/api/users/${userId}/tokens`),
  createUserToken: (userId: string, clientName: string) =>
    request<{ token: string; userId: string; clientName: string }>(`/api/users/${userId}/tokens`, {
      method: 'POST',
      body: JSON.stringify({ clientName }),
    }),
  revokeUserToken: (userId: string, token: string) =>
    request<{ status: string }>(`/api/users/${userId}/tokens/${token}`, { method: 'DELETE' }),

  // Bangumi Bind
  bindBangumi: (userId: string, accessToken: string) =>
    request<{ status: string; username: string; nickname: string; avatar: any }>(`/api/users/${userId}/bangumi`, {
      method: 'POST',
      body: JSON.stringify({ accessToken }),
    }),
  unbindBangumi: (userId: string) =>
    request<{ status: string }>(`/api/users/${userId}/bangumi`, { method: 'DELETE' }),

  // Rules
  listRules: () => request<RulePlugin[]>('/api/rules'),
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
    request<{ status: string; importedCount: number }>('/api/rules/import-url', {
      method: 'POST',
      body: JSON.stringify({ url }),
    }),
  testRule: (params: { plugin?: any; ruleName?: string; keyword: string }) =>
    request<TestResult>('/api/rules/test', {
      method: 'POST',
      body: JSON.stringify(params),
    }),

  // Playback History
  listHistory: () => request<PlaybackHistory[]>('/api/history'),
}
