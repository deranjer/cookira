import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Build output lands directly in internal/webapp/dist so the Go binary can
// embed it with go:embed (embed can't reach outside its own package dir).
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/webapp/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5190,
    strictPort: true,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
