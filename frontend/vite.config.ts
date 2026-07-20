import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
  },
  build: {
    rollupOptions: {
      output: {
        // Split heavy, rarely-changing vendors into their own cacheable chunks
        // so the initial payload is smaller and long-term caching works across
        // app deploys. Route- and tab-level code-splitting (React.lazy) keeps
        // the admin panel, charts and QR encoder out of the first load.
        manualChunks: {
          react: ['react', 'react-dom', 'react-router-dom'],
          webauthn: ['@simplewebauthn/browser'],
          qrcode: ['qrcode.react'],
        },
      },
    },
  },
})
