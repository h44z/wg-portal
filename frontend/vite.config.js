import { fileURLToPath, URL } from 'url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
    outDir: '../internal/app/api/core/frontend-dist',
    emptyOutDir: true
  },
  // local dev api (proxy to avoid cors problems)
  server: {
    port: 5000,
    proxy: {
      "/api/v0": {
        target: "http://localhost:8888",
        changeOrigin: true,
        secure: false,
        withCredentials: true,
        headers: {
          "x-wg-dev": true,
        },
        rewrite: (path) => path,
      },
    },
  },
})
