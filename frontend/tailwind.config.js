/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        domjudge: {
          bg: '#f8fafc',
          surface: '#ffffff',
          'surface-alt': '#f1f5f9',
          border: '#e2e8f0',
          'border-dark': '#cbd5e1',
          primary: '#2563eb',
          'primary-hover': '#1d4ed8',
          accent: '#0284c7',
          gold: '#f59e0b',
          silver: '#94a3b8',
          bronze: '#d97706',
          success: '#059669',
          warning: '#d97706',
          danger: '#dc2626',
          muted: '#64748b',
        },
        balloon: {
          red: '#ef4444',
          blue: '#3b82f6',
          green: '#10b981',
          yellow: '#f59e0b',
          purple: '#8b5cf6',
          cyan: '#06b6d4',
          orange: '#f97316',
          pink: '#ec4899',
        }
      },
      keyframes: {
        'pulse-subtle': {
          '0%, 100%': { opacity: '1', transform: 'scale(1)' },
          '50%': { opacity: '0.85', transform: 'scale(1.03)' },
        },
        'shimmer': {
          '100%': { transform: 'translateX(100%)' }
        },
        'bounce-subtle': {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-3px)' },
        },
        'fade-in': {
          '0%': { opacity: '0', transform: 'translateY(6px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        }
      },
      animation: {
        'pulse-subtle': 'pulse-subtle 2s ease-in-out infinite',
        'bounce-subtle': 'bounce-subtle 1.5s ease-in-out infinite',
        'fade-in': 'fade-in 0.25s cubic-bezier(0.16, 1, 0.3, 1) forwards',
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
