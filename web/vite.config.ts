import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Build output goes into the Go server's embed directory so the server ships as
// a single self-contained binary.
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
