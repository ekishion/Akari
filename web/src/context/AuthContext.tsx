import React, { createContext, useContext, useEffect, useState } from 'react'
import { api } from '../api'

interface AuthContextType {
  isAuthenticated: boolean
  username: string
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  checkAuth: () => Promise<boolean>
}

const AuthContext = createContext<AuthContextType | null>(null)

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => {
    return !!localStorage.getItem('akari_admin_token')
  })
  const [username, setUsername] = useState<string>('admin')
  const [loading, setLoading] = useState<boolean>(true)

  const checkAuth = async (): Promise<boolean> => {
    const token = localStorage.getItem('akari_admin_token')
    if (!token) {
      setIsAuthenticated(false)
      setLoading(false)
      return false
    }

    try {
      const profile = await api.adminProfile()
      setUsername(profile.username || 'admin')
      setIsAuthenticated(true)
      setLoading(false)
      return true
    } catch {
      localStorage.removeItem('akari_admin_token')
      setIsAuthenticated(false)
      setLoading(false)
      return false
    }
  }

  useEffect(() => {
    checkAuth()

    const handleUnauthorized = () => {
      setIsAuthenticated(false)
    }

    window.addEventListener('akari_unauthorized', handleUnauthorized)
    return () => window.removeEventListener('akari_unauthorized', handleUnauthorized)
  }, [])

  const login = async (user: string, pass: string) => {
    const res = await api.adminLogin(user, pass)
    if (res.token) {
      localStorage.setItem('akari_admin_token', res.token)
      setUsername(res.username || user)
      setIsAuthenticated(true)
    }
  }

  const logout = async () => {
    try {
      await api.adminLogout()
    } catch {
      // ignore
    } finally {
      localStorage.removeItem('akari_admin_token')
      setIsAuthenticated(false)
    }
  }

  return (
    <AuthContext.Provider value={{ isAuthenticated, username, loading, login, logout, checkAuth }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
