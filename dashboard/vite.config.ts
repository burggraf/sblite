import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  // Force development mode for better error messages
  define: {
    'process.env.NODE_ENV': JSON.stringify('development'),
  },
  // Base path for the dashboard - assets will be served at /_/assets/*
  base: '/_/',
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    // Proxy API requests to the Go backend
    proxy: {
      '/_/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/auth/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/rest/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/storage/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/functions/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/realtime/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true, // Enable WebSocket proxy for realtime
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    minify: false, // Disable minification for better error messages
    // Optimize for production embedding
    rollupOptions: {
      output: {
        manualChunks: {
          // Vendor chunk for React and other libs
          vendor: ['react', 'react-dom', 'react-router-dom'],
        },
      },
    },
  },
})
