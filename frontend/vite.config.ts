/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const testConfig = {
  test: {
    environment: 'jsdom' as const,
    setupFiles: ['./src/test/setup.ts'],
  },
}

export default defineConfig({
  ...testConfig,
  plugins: [react(), tailwindcss()],
  // Assets are served from /app/* by internal/server
  base: '/app/',
  build: {
    outDir: '../internal/server/static',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1000,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8091',
    },
  },
})