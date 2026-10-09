import React from 'react'
import { motion } from 'framer-motion'
import { useTheme } from '../../context/ThemeContext'

export interface AkariLogoProps {
  /** Size in pixels or CSS string (e.g. 24, 32, 44, 64, '100%') */
  size?: number | string
  /** 'badge': with squircle gradient container; 'glyph': standalone transparent vector mark */
  variant?: 'badge' | 'glyph'
  /** 'auto' (detect from ThemeContext), 'light', or 'dark' */
  theme?: 'auto' | 'light' | 'dark'
  /** Optional extra CSS classes */
  className?: string
  /** Enable subtle interactive spring animations on hover/tap */
  interactive?: boolean
  /** Enable ambient glow filter (default true) */
  glow?: boolean
}

export const AkariLogo: React.FC<AkariLogoProps> = ({
  size = 40,
  variant = 'badge',
  theme = 'auto',
  className = '',
  interactive = false,
  glow = true,
}) => {
  // Gracefully read theme or fallback
  let isDark = true
  try {
    const themeCtx = useTheme()
    isDark = themeCtx.isDark
  } catch {
    if (typeof document !== 'undefined') {
      isDark = document.documentElement.classList.contains('dark')
    }
  }

  // Determine active color scheme
  const activeDark = theme === 'dark' ? true : theme === 'light' ? false : isDark
  const isBadge = variant === 'badge'

  const svgContent = (
    <svg
      width={size}
      height={size}
      viewBox="0 0 100 100"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className="shrink-0 transition-all duration-300"
      aria-label="Akari Bridge Logo"
    >
      <defs>
        {/* Dark Theme Background Gradient */}
        <linearGradient id="akari-bg-dark" x1="0" y1="0" x2="100" y2="100" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#1E1B4B" />
          <stop offset="50%" stopColor="#0F172A" />
          <stop offset="100%" stopColor="#310D20" />
        </linearGradient>

        {/* Light Theme Background Gradient */}
        <linearGradient id="akari-bg-light" x1="0" y1="0" x2="100" y2="100" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#EEF2FF" />
          <stop offset="55%" stopColor="#FFFFFF" />
          <stop offset="100%" stopColor="#FCE7F3" />
        </linearGradient>

        {/* Dark Theme Gradients */}
        <linearGradient id="akari-bridge-dark" x1="10" y1="90" x2="90" y2="10" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#6366F1" />
          <stop offset="45%" stopColor="#8B5CF6" />
          <stop offset="100%" stopColor="#EC4899" />
        </linearGradient>

        <linearGradient id="akari-play-dark" x1="30" y1="20" x2="85" y2="75" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#FB7185" />
          <stop offset="40%" stopColor="#F43F5E" />
          <stop offset="85%" stopColor="#7C3AED" />
          <stop offset="100%" stopColor="#4F46E5" />
        </linearGradient>

        <linearGradient id="akari-flow-dark" x1="20" y1="50" x2="85" y2="50" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#38BDF8" stopOpacity="0.9" />
          <stop offset="50%" stopColor="#F43F5E" stopOpacity="0.8" />
          <stop offset="100%" stopColor="#FBBF24" stopOpacity="0.9" />
        </linearGradient>

        {/* Light Theme High-Contrast Gradients */}
        <linearGradient id="akari-bridge-light" x1="10" y1="90" x2="90" y2="10" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#4F46E5" />
          <stop offset="45%" stopColor="#7C3AED" />
          <stop offset="100%" stopColor="#DB2777" />
        </linearGradient>

        <linearGradient id="akari-play-light" x1="30" y1="20" x2="85" y2="75" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#F43F5E" />
          <stop offset="40%" stopColor="#E11D48" />
          <stop offset="85%" stopColor="#6D28D9" />
          <stop offset="100%" stopColor="#4338CA" />
        </linearGradient>

        <linearGradient id="akari-flow-light" x1="20" y1="50" x2="85" y2="50" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#0284C7" stopOpacity="0.95" />
          <stop offset="50%" stopColor="#E11D48" stopOpacity="0.85" />
          <stop offset="100%" stopColor="#D97706" stopOpacity="0.95" />
        </linearGradient>

        {/* Radiant Bloom Filters */}
        <filter id="akari-bloom-dark" x="-25%" y="-25%" width="150%" height="150%">
          <feGaussianBlur stdDeviation="3.2" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>

        <filter id="akari-bloom-light" x="-25%" y="-25%" width="150%" height="150%">
          <feGaussianBlur stdDeviation="2.2" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>

        {/* Drop shadow for star in light mode */}
        <filter id="akari-star-shadow" x="-30%" y="-30%" width="160%" height="160%">
          <feDropShadow dx="0" dy="1" stdDeviation="1.2" floodColor="#7C3AED" floodOpacity="0.35" />
        </filter>
      </defs>

      {/* Squircle Badge Base */}
      {isBadge && (
        <>
          <rect
            x="2"
            y="2"
            width="96"
            height="96"
            rx="26"
            fill={activeDark ? 'url(#akari-bg-dark)' : 'url(#akari-bg-light)'}
          />
          <rect
            x="2"
            y="2"
            width="96"
            height="96"
            rx="26"
            stroke={activeDark ? 'rgba(255, 255, 255, 0.14)' : 'rgba(99, 102, 241, 0.2)'}
            strokeWidth="1.5"
          />
          {glow && (
            <>
              <circle
                cx="50"
                cy="50"
                r="30"
                fill={activeDark ? '#6366F1' : '#818CF8'}
                opacity={activeDark ? 0.28 : 0.18}
                filter={activeDark ? 'url(#akari-bloom-dark)' : 'url(#akari-bloom-light)'}
              />
              <circle
                cx="62"
                cy="42"
                r="22"
                fill={activeDark ? '#F43F5E' : '#FB7185'}
                opacity={activeDark ? 0.25 : 0.16}
                filter={activeDark ? 'url(#akari-bloom-dark)' : 'url(#akari-bloom-light)'}
              />
            </>
          )}
        </>
      )}

      {/* Bridge Gateway Arch (Sweeping Interconnect) */}
      <path
        d="M 22 75 C 20 48, 42 25, 68 25 C 75 25, 82 28, 86 32 C 83 35, 76 32, 68 32 C 48 32, 29 50, 30 75 C 30 78, 22 78, 22 75 Z"
        fill={activeDark ? 'url(#akari-bridge-dark)' : 'url(#akari-bridge-light)'}
        opacity={isBadge ? 0.95 : 1}
      />

      {/* Stream Flow Under-Ribbon */}
      <path
        d="M 26 78 C 38 78, 54 74, 68 64 C 71 61, 74 64, 72 68 C 58 80, 42 86, 26 86 C 22 86, 22 78, 26 78 Z"
        fill={activeDark ? 'url(#akari-flow-dark)' : 'url(#akari-flow-light)'}
        opacity={isBadge ? (activeDark ? 0.75 : 0.85) : 0.9}
      />

      {/* Central Media Play Prism (Core Media Vector) */}
      <path
        d="M 38 30 C 38 26.5, 41.8 24.5, 44.8 26.3 L 74.5 44.5 C 77.2 46.2, 77.2 50.8, 74.5 52.5 L 44.8 70.7 C 41.8 72.5, 38 70.5, 38 67 Z"
        fill={activeDark ? 'url(#akari-play-dark)' : 'url(#akari-play-light)'}
        filter={
          glow && isBadge
            ? activeDark
              ? 'url(#akari-bloom-dark)'
              : 'url(#akari-bloom-light)'
            : undefined
        }
      />

      {/* Glass Prism Reflection Facet */}
      <path
        d="M 38 30 L 74.5 44.5 L 48 54 Z"
        fill="white"
        opacity={activeDark ? 0.26 : 0.32}
      />

      {/* Major Radiant Akari Star (あかりの主芒星) */}
      <path
        d="M 68 18 Q 68 27 59 27 Q 68 27 68 36 Q 68 27 77 27 Q 68 27 68 18 Z"
        fill="#FFFFFF"
        filter={
          activeDark
            ? glow
              ? 'url(#akari-bloom-dark)'
              : undefined
            : 'url(#akari-star-shadow)'
        }
      />
      <circle
        cx="68"
        cy="27"
        r={1.8}
        fill={activeDark ? '#FEF08A' : '#F59E0B'}
      />

      {/* Minor Satellite Spark (灵动伴星) */}
      <path
        d="M 30 38 Q 30 42 26 42 Q 30 42 30 46 Q 30 42 34 42 Q 30 42 30 38 Z"
        fill={activeDark ? '#FDE047' : '#D97706'}
        opacity={0.9}
      />
    </svg>
  )

  if (!interactive) {
    return <div className={`inline-flex items-center justify-center ${className}`}>{svgContent}</div>
  }

  return (
    <motion.div
      whileHover={{ scale: 1.06, rotate: 6 }}
      whileTap={{ scale: 0.95 }}
      transition={{ type: 'spring', stiffness: 400, damping: 15 }}
      className={`inline-flex items-center justify-center cursor-pointer ${className}`}
    >
      {svgContent}
    </motion.div>
  )
}
