import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://100.64.1.38:8070',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
})
