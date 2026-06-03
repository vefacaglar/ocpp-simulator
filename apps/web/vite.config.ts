import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Vite dev server proxies:
//   /api/*  → simulator-api (CRUD, settings, realtime observation, health)
//   /ws/*   → ocpp-gateway (per-charge-point OCPP WebSocket, raw frames)
//
// In the v4.3 split the Vue app no longer talks to simulator-api for
// OCPP frame transport; it opens one WebSocket per simulated charge
// point directly to the gateway. simulator-api only carries CRUD and
// observation channels.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:7070',
        changeOrigin: true,
        ws: true,
      },
      '/ws': {
        target: 'ws://localhost:7080',
        ws: true,
        changeOrigin: true,
      },
    },
  },
})
