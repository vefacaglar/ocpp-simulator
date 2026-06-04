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
      <div>
        <h2>Live Logs</h2>
        <span class="subtitle">Realtime events</span>
      </div>
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
        <p class="empty-title">Select a charge point.</p>
      </div>
      <div v-else-if="filteredEvents.length === 0" class="empty-state">
        <p class="empty-title">No events yet.</p>
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
          <div class="drawer-field"><span class="label">Type</span><span class="value">{{ drawerEvent.type }}</span></div>
          <div class="drawer-field"><span class="label">Time</span><span class="value">{{ drawerEvent.timestamp }}</span></div>
          <div class="drawer-field"><span class="label">Charge Point</span><span class="value">{{ drawerEvent.chargePointId }}</span></div>
          <div v-if="drawerEvent.connectorId" class="drawer-field"><span class="label">Connector</span><span class="value">{{ drawerEvent.connectorId }}</span></div>
          <div v-if="drawerEvent.direction" class="drawer-field"><span class="label">Direction</span><span class="value">{{ drawerEvent.direction }}</span></div>
          <div v-if="drawerEvent.action" class="drawer-field"><span class="label">Action</span><span class="value">{{ drawerEvent.action }}</span></div>
          <div class="drawer-field"><span class="label">Message</span><span class="value">{{ drawerEvent.message }}</span></div>
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
.live-log-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-elevated);
  position: relative;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px 16px 24px;
  border-bottom: 1px solid var(--border-subtle);
}

.panel-header h2 {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  font-weight: 500;
  margin: 0;
  color: var(--text-primary);
  letter-spacing: 0.02em;
}

.subtitle {
  display: block;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  color: var(--text-muted);
  margin-top: 2px;
}

.count {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  padding: 2px 9px;
  color: var(--text-secondary);
}

.filter-bar {
  display: flex;
  gap: 4px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-subtle);
  background: transparent;
}

.filter-btn {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  padding: 4px 11px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.filter-btn:hover {
  color: var(--text-secondary);
  border-color: var(--border-subtle);
}
.filter-btn.active {
  color: var(--accent);
  border-color: rgba(232, 223, 200, 0.4);
  background: var(--accent-soft);
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px 16px 12px;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  background: var(--bg-sunken);
}

.empty-state {
  text-align: center;
  margin-top: 3rem;
}

.empty-title {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary);
  letter-spacing: 0.04em;
}

.log-rows {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.log-row {
  display: grid;
  grid-template-columns: 64px 150px 1fr;
  gap: 12px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  align-items: center;
  cursor: pointer;
  transition: background 0.12s;
  border: 1px solid transparent;
}
.log-row:hover {
  background: rgba(245, 242, 235, 0.03);
  border-color: var(--border-subtle);
}

.log-time {
  color: var(--text-muted);
  font-size: 0.68rem;
  letter-spacing: 0.04em;
}
.log-type {
  color: var(--text-secondary);
  font-size: 0.7rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: 0.02em;
}
.log-msg {
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-row.ocpp        .log-type { color: var(--accent); }
.log-row.connection  .log-type { color: var(--status-online); }
.log-row.connector   .log-type { color: var(--status-busy); }
.log-row.transaction .log-type { color: #c9a3d4; }
.log-row.runtime     .log-type { color: var(--status-fault); }
.log-row.error {
  background: rgba(200, 123, 95, 0.06);
  border-color: rgba(200, 123, 95, 0.20);
}
.log-row.error .log-type,
.log-row.error .log-msg { color: var(--status-fault); }

.drawer-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  z-index: 10;
  display: flex;
  justify-content: flex-end;
}

.drawer {
  width: 360px;
  background: var(--bg-elevated);
  border-left: 1px solid var(--border-strong);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border-subtle);
}
.drawer-header h3 {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  font-weight: 500;
  margin: 0;
  color: var(--text-primary);
  letter-spacing: 0.02em;
}
.drawer-close {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 1.2rem;
  cursor: pointer;
  transition: color 0.15s;
}
.drawer-close:hover { color: var(--text-primary); }

.drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px;
  font-size: 0.78rem;
}

.drawer-field {
  display: flex;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px solid var(--border-subtle);
  gap: 16px;
}
.drawer-field .label {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  color: var(--text-muted);
  min-width: 90px;
}
.drawer-field .value {
  font-family: var(--font-mono);
  color: var(--text-primary);
  text-align: right;
  word-break: break-word;
}

.drawer-payload {
  margin-top: 16px;
}
.drawer-payload .label {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  color: var(--text-muted);
  display: block;
  margin-bottom: 8px;
}
.drawer-payload pre {
  background: var(--bg-sunken);
  padding: 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-subtle);
  font-size: 0.68rem;
  color: var(--text-secondary);
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.5;
}
</style>
