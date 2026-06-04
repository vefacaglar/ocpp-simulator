import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { RealtimeEvent } from '../realtime/realtimeClient'
import { appendOcppLog, listOcppLogs, countOcppLogs } from '../db/browserDb'

export const useRealtimeStore = defineStore('realtime', () => {
  const events = ref<RealtimeEvent[]>([])
  const totalLogCounts = ref<Record<string, number>>({})
  const connected = ref(false)

  function appendEvent(event: RealtimeEvent) {
    events.value = events.value.filter((e) => e.id !== event.id)
    events.value.push(event)
    if (events.value.length > 100) {
      events.value.shift()
    }
    const cpId = event.chargePointId
    const current = totalLogCounts.value[cpId] || 0
    if (current < 1000) {
      totalLogCounts.value[cpId] = current + 1
    }
    void appendOcppLog(event).catch((err) => {
      // eslint-disable-next-line no-console
      console.warn('[realtime] failed to persist log event', err)
    })
  }

  async function loadEventsForChargePoint(chargePointId: string) {
    const loaded = await listOcppLogs(chargePointId, 100)
    const others = events.value.filter((e) => e.chargePointId !== chargePointId)
    events.value = [...others, ...loaded]
    
    const count = await countOcppLogs(chargePointId)
    totalLogCounts.value[chargePointId] = Math.min(count, 1000)
  }

  function clearEvents() {
    events.value = []
  }

  function clearEventsForChargePoint(chargePointId: string) {
    events.value = events.value.filter((event) => event.chargePointId !== chargePointId)
  }

  return { events, totalLogCounts, connected, appendEvent, loadEventsForChargePoint, clearEvents, clearEventsForChargePoint }
})
