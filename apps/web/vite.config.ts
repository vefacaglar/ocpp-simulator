import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Vite dev server proxy:
//   /ws/* → ocpp-gateway (per-charge-point OCPP WebSocket, raw frames)
export default defineConfig({
  plugins: [vue()],
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
