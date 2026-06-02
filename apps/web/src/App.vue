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
.top-bar { display: flex; align-items: center; justify-content: space-between; padding: 0 1rem; height: 48px; background: #1a1a2e; color: #e0e0e0; border-bottom: 1px solid #333; }
.top-bar h1 { font-size: 1rem; font-weight: 600; margin: 0; }
.top-bar-status { display: flex; align-items: center; gap: 0.5rem; font-size: 0.8rem; color: #888; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #e06060; }
.status-dot.online { background: #4ade80; }
.content { flex: 1; display: grid; grid-template-columns: 260px 1fr 340px; overflow: hidden; }
</style>
