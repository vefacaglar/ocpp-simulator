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
  rawFrame?: unknown
}

export type EventHandler = (event: RealtimeEvent) => void

const socket = ref<WebSocket | null>(null)
const connected = ref(false)
const handlers: EventHandler[] = []
let subscribedChargePointId: string | null = null

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
      const data = JSON.parse(msg.data)
      if (Array.isArray(data)) {
        const frame = data as unknown[]
        const event: RealtimeEvent = {
          id: String(Date.now()),
          type: frame[0] === 2 ? 'ocpp.message.outbound' : 'ocpp.message.inbound',
          chargePointId: subscribedChargePointId || '',
          direction: frame[0] === 2 ? 'outbound' : 'inbound',
          action: typeof frame[2] === 'string' ? frame[2] : undefined,
          rawFrame: data,
          message: `${frame[0] === 2 ? 'CALL' : frame[0] === 3 ? 'CALLRESULT' : 'CALLERROR'} ${typeof frame[2] === 'string' ? frame[2] : ''}`,
          timestamp: new Date().toISOString(),
        }
        for (const handler of handlers) {
          handler(event)
        }
      } else {
        const event: RealtimeEvent = data
        for (const handler of handlers) {
          handler(event)
        }
      }
    } catch {}
  }
}

export function subscribeChargePoint(chargePointId: string) {
  subscribedChargePointId = chargePointId
  socket.value?.send(JSON.stringify({ type: 'subscribe', chargePointId }))
}

export function unsubscribeChargePoint(chargePointId: string) {
  subscribedChargePointId = null
  socket.value?.send(JSON.stringify({ type: 'unsubscribe', chargePointId }))
}

export function onRealtimeEvent(handler: EventHandler) {
  handlers.push(handler)
}

export function useRealtimeState() {
  return { connected }
}
