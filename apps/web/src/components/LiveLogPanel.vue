<script setup lang="ts">
import { useRealtimeStore } from '../stores/realtimeStore'
import { useChargePointStore } from '../stores/chargePointStore'
import { computed } from 'vue'

const rtStore = useRealtimeStore()
const cpStore = useChargePointStore()

const selectedId = computed(() => cpStore.selectedId)

function formatTime(ts: string) {
  try {
    const d = new Date(ts)
    return d.toLocaleTimeString('en-GB', { hour12: false })
  } catch {
    return ts
  }
}

function eventClass(type: string) {
  if (type.startsWith('ocpp.')) return 'ocpp'
  if (type.startsWith('charge_point.')) return 'connection'
  if (type.startsWith('connector.')) return 'connector'
  if (type.startsWith('transaction.')) return 'transaction'
  if (type.startsWith('runtime.')) return 'runtime'
  return ''
}
</script>

<template>
  <div class="live-log-panel">
    <div class="panel-header">
      <h2>Live Logs</h2>
      <span class="count" v-if="rtStore.events.length">{{ rtStore.events.length }}</span>
    </div>
    <div class="panel-body">
      <div v-if="!selectedId" class="empty-state">
        <p>Select a charge point.</p>
      </div>
      <div v-else-if="rtStore.events.length === 0" class="empty-state">
        <p>No events yet.</p>
      </div>
      <div v-else class="log-rows">
        <div v-for="event in rtStore.events" :key="event.id" class="log-row" :class="eventClass(event.type)">
          <span class="log-time">{{ formatTime(event.timestamp) }}</span>
          <span class="log-type">{{ event.type }}</span>
          <span class="log-msg">{{ event.message }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.live-log-panel { display: flex; flex-direction: column; height: 100%; background: #12121f; border-left: 1px solid #2a2a3a; }
.panel-header { display: flex; align-items: center; gap: 0.5rem; padding: 0.75rem 1rem; border-bottom: 1px solid #2a2a3a; background: #16162a; }
.panel-header h2 { font-size: 0.85rem; font-weight: 600; margin: 0; text-transform: uppercase; letter-spacing: 0.05em; color: #8888aa; }
.count { font-size: 0.65rem; background: #2a2a4a; color: #8888cc; padding: 1px 6px; border-radius: 10px; }
.panel-body { flex: 1; overflow-y: auto; padding: 0.25rem; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.7rem; }
.empty-state { text-align: center; margin-top: 2rem; color: #555; font-size: 0.85rem; }
.log-rows { display: flex; flex-direction: column; gap: 1px; }
.log-row { display: grid; grid-template-columns: 60px 140px 1fr; gap: 0.5rem; padding: 0.25rem 0.5rem; border-radius: 3px; align-items: center; }
.log-row:hover { background: #1a1a30; }
.log-time { color: #555; }
.log-type { color: #8888aa; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-msg { color: #aaa; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-row.ocpp .log-type { color: #60a0e0; }
.log-row.connection .log-type { color: #4ade80; }
.log-row.connector .log-type { color: #e0c060; }
.log-row.transaction .log-type { color: #c080e0; }
.log-row.runtime .log-type { color: #e06060; }
</style>
