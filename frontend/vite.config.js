import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// During dev, proxy API calls to the Go backend so the frontend can use
// same-origin relative paths. The WebSocket connects to :8080 directly.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
