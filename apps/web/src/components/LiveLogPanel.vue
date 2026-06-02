<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRealtimeStore } from '../stores/realtimeStore'
import { useChargePointStore } from '../stores/chargePointStore'

const rtStore = useRealtimeStore()
const cpStore = useChargePointStore()

const selectedId = computed(() => cpStore.selectedId)
const filter = ref('all')
const drawerEvent = ref<any>(null)

const filteredEvents = computed(() => {
  if (filter.value === 'all') return rtStore.events
  return rtStore.events.filter((e) => {
    if (filter.value === 'ocpp') return e.type.startsWith('ocpp.')
    if (filter.value === 'connection') return e.type.startsWith('charge_point.') || e.type.startsWith('connector.')
    if (filter.value === 'transaction') return e.type.startsWith('transaction.') || e.type.startsWith('meter_value.')
    if (filter.value === 'runtime') return e.type.startsWith('runtime.')
    return true
  })
})

function formatTime(ts: string) {
  try {
    return new Date(ts).toLocaleTimeString('en-GB', { hour12: false })
  } catch { return ts }
}

function eventClass(type: string) {
  if (type.startsWith('ocpp.')) return 'ocpp'
  if (type.startsWith('charge_point.') || type.startsWith('connector.')) return 'connection'
  if (type.startsWith('transaction.') || type.startsWith('meter_value.')) return 'transaction'
  if (type.startsWith('runtime.')) return 'runtime'
  if (type.includes('error') || type.includes('failed')) return 'error'
  return ''
}

function openDrawer(event: any) {
  drawerEvent.value = event
}

function closeDrawer() {
  drawerEvent.value = null
}
</script>

<template>
  <div class="live-log-panel">
    <div class="panel-header">
      <h2>Live Logs</h2>
      <span class="count" v-if="rtStore.events.length">{{ filteredEvents.length }}</span>
    </div>
    <div class="filter-bar">
      <button v-for="f in ['all','ocpp','connection','transaction','runtime']" :key="f"
        class="filter-btn" :class="{ active: filter === f }" @click="filter = f">
        {{ f }}
      </button>
    </div>
    <div class="panel-body">
      <div v-if="!selectedId" class="empty-state">
        <p>Select a charge point.</p>
      </div>
      <div v-else-if="filteredEvents.length === 0" class="empty-state">
        <p>No events.</p>
      </div>
      <div v-else class="log-rows">
        <div v-for="event in filteredEvents" :key="event.id" class="log-row" :class="eventClass(event.type)" @click="openDrawer(event)">
          <span class="log-time">{{ formatTime(event.timestamp) }}</span>
          <span class="log-type">{{ event.type }}</span>
          <span class="log-msg">{{ event.message }}</span>
        </div>
      </div>
    </div>

    <div v-if="drawerEvent" class="drawer-overlay" @click.self="closeDrawer">
      <div class="drawer">
        <div class="drawer-header">
          <h3>Event Detail</h3>
          <button class="drawer-close" @click="closeDrawer">&times;</button>
        </div>
        <div class="drawer-body">
          <div class="drawer-field"><span class="label">Type</span><span>{{ drawerEvent.type }}</span></div>
          <div class="drawer-field"><span class="label">Time</span><span>{{ drawerEvent.timestamp }}</span></div>
          <div class="drawer-field"><span class="label">Charge Point</span><span>{{ drawerEvent.chargePointId }}</span></div>
          <div v-if="drawerEvent.connectorId" class="drawer-field"><span class="label">Connector</span><span>{{ drawerEvent.connectorId }}</span></div>
          <div v-if="drawerEvent.direction" class="drawer-field"><span class="label">Direction</span><span>{{ drawerEvent.direction }}</span></div>
          <div v-if="drawerEvent.action" class="drawer-field"><span class="label">Action</span><span>{{ drawerEvent.action }}</span></div>
          <div class="drawer-field"><span class="label">Message</span><span>{{ drawerEvent.message }}</span></div>
          <div v-if="drawerEvent.rawFrame" class="drawer-payload">
            <span class="label">Raw Frame</span>
            <pre>{{ JSON.stringify(drawerEvent.rawFrame, null, 2) }}</pre>
          </div>
          <div v-else-if="drawerEvent.payload" class="drawer-payload">
            <span class="label">Payload</span>
            <pre>{{ JSON.stringify(drawerEvent.payload, null, 2) }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.live-log-panel { display: flex; flex-direction: column; height: 100%; background: #fff; border-left: 1px solid #ddd; position: relative; }
.panel-header { display: flex; align-items: center; gap: 0.5rem; padding: 0.75rem 1rem; border-bottom: 1px solid #ddd; background: #fafafa; }
.panel-header h2 { font-size: 0.85rem; font-weight: 600; margin: 0; text-transform: uppercase; letter-spacing: 0.05em; color: #666; }
.count { font-size: 0.65rem; background: #e8e8f0; color: #666; padding: 1px 6px; border-radius: 10px; }
.filter-bar { display: flex; gap: 0.25rem; padding: 0.25rem 0.5rem; border-bottom: 1px solid #eee; background: #fafafa; }
.filter-btn { padding: 0.15rem 0.4rem; font-size: 0.6rem; border: 1px solid #ddd; border-radius: 3px; background: transparent; color: #999; cursor: pointer; text-transform: capitalize; }
.filter-btn:hover { color: #666; border-color: #bbb; }
.filter-btn.active { background: #e8e8f4; color: #6366f1; border-color: #b0b0d0; }
.panel-body { flex: 1; overflow-y: auto; padding: 0.25rem; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.7rem; }
.empty-state { text-align: center; margin-top: 2rem; color: #999; font-size: 0.85rem; }
.log-rows { display: flex; flex-direction: column; gap: 1px; }
.log-row { display: grid; grid-template-columns: 60px 130px 1fr; gap: 0.5rem; padding: 0.25rem 0.5rem; border-radius: 3px; align-items: center; cursor: pointer; }
.log-row:hover { background: #f5f5f8; }
.log-time { color: #bbb; }
.log-type { color: #666; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-msg { color: #999; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.log-row.ocpp .log-type { color: #2563eb; }
.log-row.connection .log-type { color: #16a34a; }
.log-row.connector .log-type { color: #ca8a04; }
.log-row.transaction .log-type { color: #9333ea; }
.log-row.runtime .log-type { color: #dc2626; }
.log-row.error { background: #fef2f2; }
.log-row.error .log-type { color: #dc2626; }
.log-row.error .log-msg { color: #ef4444; }

.drawer-overlay { position: absolute; inset: 0; background: rgba(0,0,0,0.2); z-index: 10; display: flex; justify-content: flex-end; }
.drawer { width: 320px; background: #fff; border-left: 1px solid #ddd; display: flex; flex-direction: column; overflow: hidden; }
.drawer-header { display: flex; align-items: center; justify-content: space-between; padding: 0.75rem 1rem; border-bottom: 1px solid #eee; }
.drawer-header h3 { font-size: 0.85rem; margin: 0; color: #333; }
.drawer-close { background: none; border: none; color: #999; font-size: 1.2rem; cursor: pointer; }
.drawer-close:hover { color: #333; }
.drawer-body { flex: 1; overflow-y: auto; padding: 0.75rem; font-size: 0.75rem; }
.drawer-field { display: flex; justify-content: space-between; padding: 0.25rem 0; border-bottom: 1px solid #f0f0f0; }
.drawer-field .label { color: #999; min-width: 80px; }
.drawer-payload { margin-top: 0.5rem; }
.drawer-payload .label { color: #999; display: block; margin-bottom: 0.25rem; }
.drawer-payload pre { background: #f5f5f8; padding: 0.5rem; border-radius: 4px; font-size: 0.65rem; color: #6366f1; overflow-x: auto; white-space: pre-wrap; word-break: break-all; }
</style>
