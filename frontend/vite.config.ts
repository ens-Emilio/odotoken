import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Assets are served from /app/* by the FastAPI static mount
  base: '/app/',
  build: {
    outDir: '../src/odotoken/static',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1000,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8090',
    },
  },
})