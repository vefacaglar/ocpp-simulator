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

const connected = ref(false)

export function connectRealtime() {
  // Realtime WebSocket is disabled.
}

export function subscribeChargePoint(_chargePointId: string) {
  // No-op: realtime is disabled.
}

export function unsubscribeChargePoint(_chargePointId: string) {
  // No-op: realtime is disabled.
}

export function onRealtimeEvent(_handler: EventHandler) {
  // No-op: realtime is disabled.
}

export function useRealtimeState() {
  return { connected }
}
