import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { RealtimeEvent } from '../realtime/realtimeClient'
import { appendOcppLog, listOcppLogs } from '../db/browserDb'

export const useRealtimeStore = defineStore('realtime', () => {
  const events = ref<RealtimeEvent[]>([])
  const connected = ref(false)

  function appendEvent(event: RealtimeEvent) {
    events.value = events.value.filter((e) => e.id !== event.id)
    events.value.push(event)
    if (events.value.length > 100) {
      events.value.shift()
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
  }

  function clearEvents() {
    events.value = []
  }

  function clearEventsForChargePoint(chargePointId: string) {
    events.value = events.value.filter((event) => event.chargePointId !== chargePointId)
  }

  return { events, connected, appendEvent, loadEventsForChargePoint, clearEvents, clearEventsForChargePoint }
})
