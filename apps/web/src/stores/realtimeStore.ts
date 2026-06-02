import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { RealtimeEvent } from '../realtime/realtimeClient'

export const useRealtimeStore = defineStore('realtime', () => {
  const events = ref<RealtimeEvent[]>([])
  const connected = ref(false)

  function appendEvent(event: RealtimeEvent) {
    events.value.push(event)
    if (events.value.length > 500) {
      events.value.shift()
    }
  }

  function clearEvents() {
    events.value = []
  }

  return { events, connected, appendEvent, clearEvents }
})
