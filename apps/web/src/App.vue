<script setup lang="ts">
import { onMounted, watch } from 'vue'
import ChargePointList from './components/ChargePointList.vue'
import ChargePointDetail from './components/ChargePointDetail.vue'
import LiveLogPanel from './components/LiveLogPanel.vue'
import { useChargePointStore } from './stores/chargePointStore'
import { useRealtimeStore } from './stores/realtimeStore'
import {
  connectRealtime,
  subscribeChargePoint,
  unsubscribeChargePoint,
  onRealtimeEvent,
  useRealtimeState,
} from './realtime/realtimeClient'

const cpStore = useChargePointStore()
const rtStore = useRealtimeStore()
const { connected: wsConnected } = useRealtimeState()

onMounted(() => {
  connectRealtime()
  onRealtimeEvent((event) => {
    rtStore.appendEvent(event)
    if (event.type === 'charge_point.connected' || event.type === 'charge_point.disconnected') {
      cpStore.loadChargePoints()
    }
  })
})

watch(
  () => cpStore.selectedId,
  (newId, oldId) => {
    if (oldId) unsubscribeChargePoint(oldId)
    if (newId) {
      subscribeChargePoint(newId)
      rtStore.clearEvents()
    }
  }
)
</script>

<template>
  <div class="app-shell">
    <header class="top-bar">
      <h1>OCPP Simulator</h1>
      <div class="top-bar-status">
        <span class="status-dot" :class="{ online: wsConnected }"></span>
        <span>{{ wsConnected ? 'Connected' : 'Disconnected' }}</span>
      </div>
    </header>
    <main class="content">
      <ChargePointList class="panel-left" />
      <ChargePointDetail class="panel-center" />
      <LiveLogPanel class="panel-right" />
    </main>
  </div>
</template>

<style scoped>
.app-shell { display: flex; flex-direction: column; height: 100vh; font-family: system-ui, -apple-system, sans-serif; }
.top-bar { display: flex; align-items: center; justify-content: space-between; padding: 0 1rem; height: 48px; background: #fff; color: #1a1a2e; border-bottom: 1px solid #ddd; }
.top-bar h1 { font-size: 1rem; font-weight: 600; margin: 0; }
.top-bar-status { display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; color: #666; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #e06060; }
.status-dot.online { background: #22c55e; }
.content { flex: 1; display: grid; grid-template-columns: 260px 1fr 380px; overflow: hidden; }
</style>
