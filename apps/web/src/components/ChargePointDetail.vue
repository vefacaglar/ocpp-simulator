<script setup lang="ts">
import { computed } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'

const store = useChargePointStore()

const connectors = computed(() => store.selectedDetail?.connectors ?? [])
const cpId = computed(() => store.selectedId)
const connected = computed(() => store.selectedDetail?.chargePoint?.status === 'connected')

async function startTransaction(connectorId: number) {
  if (!cpId.value) return
  await fetch(`/api/charge-points/${cpId.value}/connectors/${connectorId}/start-transaction`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ idTag: 'DEADBEEF' }),
  })
  await store.selectChargePoint(cpId.value)
}

async function stopTransaction(connectorId: number) {
  if (!cpId.value) return
  await fetch(`/api/charge-points/${cpId.value}/connectors/${connectorId}/stop-transaction`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reason: 'Local' }),
  })
  await store.selectChargePoint(cpId.value)
}

async function sendMeterValues(connectorId: number) {
  if (!cpId.value) return
  await fetch(`/api/charge-points/${cpId.value}/connectors/${connectorId}/meter-values`, {
    method: 'POST',
  })
}

async function setStatus(connectorId: number, status: string) {
  if (!cpId.value) return
  await fetch(`/api/charge-points/${cpId.value}/connectors/${connectorId}/status`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  })
  await store.selectChargePoint(cpId.value)
}

async function addConnector() {
  await store.addConnector()
}

function statusClass(status: string) {
  return status.toLowerCase().replace(/[^a-z]/g, '')
}
</script>

<template>
  <div class="charge-point-detail">
    <div class="panel-header">
      <h2>Control</h2>
    </div>
    <div class="panel-body">
      <div v-if="!store.selectedDetail" class="empty-state">
        <p>Select a charge point to begin.</p>
      </div>
      <div v-else class="detail-content">
        <div class="section">
          <h3>{{ store.selectedDetail.chargePoint.id }}</h3>
          <div class="field"><span class="label">Name</span><span>{{ store.selectedDetail.chargePoint.name }}</span></div>
          <div class="field"><span class="label">Version</span><span>{{ store.selectedDetail.chargePoint.ocppVersion }}</span></div>
          <div class="field"><span class="label">URL</span><span class="url">{{ store.selectedDetail.chargePoint.centralSystemUrl }}</span></div>
          <div class="field"><span class="label">Status</span><span class="status" :class="store.selectedDetail.chargePoint.status">{{ store.selectedDetail.chargePoint.status }}</span></div>
        </div>

        <div class="section">
          <div class="section-header">
            <h3>Connectors</h3>
            <button class="btn-small" @click="addConnector">+ Add</button>
          </div>
          <div v-if="connectors.length === 0" class="empty-hint">No connectors.</div>
          <div v-else class="connector-list">
            <div v-for="c in connectors" :key="c.id" class="connector-card">
              <div class="connector-top">
                <span class="connector-num">Connector {{ c.connectorNumber }}</span>
                <span class="connector-status" :class="statusClass(c.status)">{{ c.status }}</span>
              </div>
              <div class="connector-actions">
                <button v-if="c.status === 'Available' && connected" class="btn-action" @click="startTransaction(c.connectorNumber)">Start TX</button>
                <button v-if="c.status === 'Charging'" class="btn-action btn-stop" @click="stopTransaction(c.connectorNumber)">Stop TX</button>
                <button v-if="c.status === 'Charging'" class="btn-action" @click="sendMeterValues(c.connectorNumber)">MeterValues</button>
                <button v-if="c.status === 'Available' && connected" class="btn-action btn-fault" @click="setStatus(c.connectorNumber, 'Faulted')">Fault</button>
                <button v-if="c.status === 'Faulted'" class="btn-action" @click="setStatus(c.connectorNumber, 'Available')">Clear</button>
                <button v-if="c.status === 'Available' && connected" class="btn-action" @click="setStatus(c.connectorNumber, 'Unavailable')">Disable</button>
                <button v-if="c.status === 'Unavailable'" class="btn-action" @click="setStatus(c.connectorNumber, 'Available')">Enable</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.charge-point-detail { display: flex; flex-direction: column; height: 100%; background: #0f0f1a; }
.panel-header { display: flex; align-items: center; padding: 0.75rem 1rem; border-bottom: 1px solid #2a2a3a; background: #16162a; }
.panel-header h2 { font-size: 0.85rem; font-weight: 600; margin: 0; text-transform: uppercase; letter-spacing: 0.05em; color: #8888aa; }
.panel-body { flex: 1; overflow-y: auto; padding: 1rem; }
.empty-state { text-align: center; margin-top: 2rem; color: #555; font-size: 0.85rem; }
.detail-content { display: flex; flex-direction: column; gap: 1.5rem; }
.section h3 { font-size: 0.9rem; color: #c0c0e0; margin-bottom: 0.5rem; }
.section-header { display: flex; align-items: center; justify-content: space-between; }
.section-header h3 { margin-bottom: 0; }
.field { display: flex; justify-content: space-between; padding: 0.25rem 0; font-size: 0.8rem; }
.label { color: #666; }
.url { font-family: monospace; font-size: 0.75rem; color: #8888cc; }
.status { font-size: 0.7rem; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; }
.status.disconnected { background: #3a2020; color: #e06060; }
.status.connected { background: #203a20; color: #60e060; }
.btn-small { padding: 0.25rem 0.5rem; font-size: 0.75rem; border: 1px solid #4a4a6a; border-radius: 4px; background: transparent; color: #8888aa; cursor: pointer; }
.btn-small:hover { background: #2a2a4a; color: #e0e0e0; }
.empty-hint { font-size: 0.8rem; color: #555; }
.connector-list { display: flex; flex-direction: column; gap: 0.5rem; }
.connector-card { padding: 0.5rem 0.75rem; border: 1px solid #2a2a3a; border-radius: 6px; background: #161628; }
.connector-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem; }
.connector-num { font-size: 0.8rem; font-weight: 600; color: #c0c0e0; }
.connector-status { font-size: 0.65rem; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; }
.connector-status.available { background: #203a20; color: #60e060; }
.connector-status.charging { background: #203050; color: #60a0e0; }
.connector-status.preparing { background: #3a3a20; color: #e0c060; }
.connector-status.finishing { background: #2a2a40; color: #a0a0c0; }
.connector-status.faulted { background: #3a2020; color: #e06060; }
.connector-status.unavailable { background: #2a2a2a; color: #888; }
.connector-actions { display: flex; gap: 0.25rem; flex-wrap: wrap; }
.btn-action { padding: 0.2rem 0.5rem; font-size: 0.65rem; border: 1px solid #3a3a5a; border-radius: 3px; background: #1a1a2e; color: #8888bb; cursor: pointer; }
.btn-action:hover { background: #2a2a4e; color: #c0c0e0; }
.btn-action.btn-stop { border-color: #5a3a3a; color: #e08080; }
.btn-action.btn-stop:hover { background: #3a2020; }
.btn-action.btn-fault { border-color: #5a3a3a; color: #e06060; }
</style>
