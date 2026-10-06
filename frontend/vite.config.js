import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// During `npm run dev` the frontend runs on its own port (5173),
// separate from the Go backend (8080). The app's API calls use
// relative paths (see src/services/api.js) so the SAME code works
// unmodified once the backend serves the built frontend directly in
// production/LAN mode. This proxy is what makes those relative paths
// also resolve correctly during local development.
const BACKEND_URL = process.env.VITE_DEV_BACKEND_URL || 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api/v1': { target: BACKEND_URL, changeOrigin: true },
      '/ws': { target: BACKEND_URL, ws: true, changeOrigin: true },
    },
  },
})
