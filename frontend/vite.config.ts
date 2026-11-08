import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

const backendURL = process.env.DROVIK_API_PROXY ?? 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: backendURL,
        changeOrigin: false,
        ws: true,
      },
      '/healthz': backendURL,
      '/readyz': backendURL,
    },
  },
  build: {
    target: 'es2022',
    sourcemap: false,
  },
})
