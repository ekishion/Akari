import React, { createContext, useContext, useEffect, useState } from 'react'

type ThemeMode = 'light' | 'dark' | 'system'

interface ThemeContextType {
  mode: ThemeMode
  isDark: boolean
  setMode: (mode: ThemeMode) => void
  toggleTheme: () => void
}

const ThemeContext = createContext<ThemeContextType | null>(null)

export const ThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [mode, setMode] = useState<ThemeMode>(() => {
    return (localStorage.getItem('akari_theme_mode') as ThemeMode) || 'dark'
  })

  const [isDark, setIsDark] = useState<boolean>(true)

  useEffect(() => {
    localStorage.setItem('akari_theme_mode', mode)

    const root = document.documentElement
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

    const computeIsDark = () => {
      if (mode === 'dark') return true
      if (mode === 'light') return false
      return mediaQuery.matches
    }

    const currentDark = computeIsDark()
    setIsDark(currentDark)

    if (currentDark) {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }

    const handleChange = () => {
      if (mode === 'system') {
        const sysDark = mediaQuery.matches
        setIsDark(sysDark)
        if (sysDark) root.classList.add('dark')
        else root.classList.remove('dark')
      }
    }

    mediaQuery.addEventListener('change', handleChange)
    return () => mediaQuery.removeEventListener('change', handleChange)
  }, [mode])

  const toggleTheme = () => {
    setMode((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }

  return (
    <ThemeContext.Provider value={{ mode, isDark, setMode, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useTheme = () => {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
