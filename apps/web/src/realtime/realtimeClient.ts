import { ref } from 'vue'

export interface RealtimeEvent {
  id: string
  type: string
  chargePointId: string
  connectorId?: number
  direction?: string
  action?: string
  payload?: unknown
  message?: string
  timestamp: string
}

export type EventHandler = (event: RealtimeEvent) => void

const socket = ref<WebSocket | null>(null)
const connected = ref(false)
const handlers: EventHandler[] = []

export function connectRealtime() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  socket.value = new WebSocket(`${protocol}//${host}/api/realtime`)

  socket.value.onopen = () => {
    connected.value = true
  }

  socket.value.onclose = () => {
    connected.value = false
    setTimeout(connectRealtime, 3000)
  }

  socket.value.onmessage = (msg) => {
    try {
      const event: RealtimeEvent = JSON.parse(msg.data)
      for (const handler of handlers) {
        handler(event)
      }
    } catch {}
  }
}

export function subscribeChargePoint(chargePointId: string) {
  socket.value?.send(JSON.stringify({ type: 'subscribe', chargePointId }))
}

export function unsubscribeChargePoint(chargePointId: string) {
  socket.value?.send(JSON.stringify({ type: 'unsubscribe', chargePointId }))
}

export function onRealtimeEvent(handler: EventHandler) {
  handlers.push(handler)
}

export function useRealtimeState() {
  return { connected }
}
