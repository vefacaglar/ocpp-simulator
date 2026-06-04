import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

// Vite dev server proxy:
//   /ws/* → ocpp-gateway (per-charge-point OCPP WebSocket, raw frames)
export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg}'],
      },
      manifest: {
        name: 'OCPP Charge Point Simulator',
        short_name: 'OCPP Sim',
        description: 'EV Charging Station Simulator for OCPP 1.6J/2.0.1',
        theme_color: '#1e1e1e',
        background_color: '#141414',
        display: 'standalone',
        orientation: 'any',
        start_url: '/',
        icons: [
          {
            src: 'logo.png',
            sizes: '192x192',
            type: 'image/png',
            purpose: 'any'
          },
          {
            src: 'logo.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'any'
          },
          {
            src: 'logo.png',
            sizes: '1024x1024',
            type: 'image/png',
            purpose: 'any'
          }
        ]
      }
    })
  ],
  server: {
    port: 5173,
    proxy: {
      '/ws': {
        target: 'ws://localhost:7080',
        ws: true,
        changeOrigin: true,
      },
    },
  },
})
