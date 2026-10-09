import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  base: './',
  build: {
    target: 'esnext',
    cssTarget: 'chrome100',
    minify: 'terser',
  },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8096',
      '/emby': 'http://localhost:8096',
    },
  },
})
