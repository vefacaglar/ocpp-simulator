<script setup lang="ts">
import { ref, computed } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'

const store = useChargePointStore()

const connectors = computed(() => store.selectedDetail?.connectors ?? [])
const connected = computed(() => store.selectedId ? store.isConnected(store.selectedId) : false)
const registered = computed(() => store.isRegistered)
const regState = computed(() => store.registrationState)

const idTagInputs = ref<Map<number, string>>(new Map())

function getIdTag(connectorId: number): string {
  return idTagInputs.value.get(connectorId) ?? ''
}

function setIdTag(connectorId: number, value: string) {
  idTagInputs.value.set(connectorId, value)
}

function connectorStatus(connectorId: number): string {
  return store.getConnectorStatus(connectorId)
}

function statusClass(status: string) {
  return status.toLowerCase().replace(/[^a-z]/g, '')
}

function handleAuthorize(connectorId: number) {
  const idTag = getIdTag(connectorId)
  if (!idTag.trim()) {
    store.error = 'idTag is required for Authorize'
    return
  }
  store.authorizeConnector(connectorId, idTag.trim())
}

function handleSimulateRemoteStart(connectorId: number) {
  const idTag = getIdTag(connectorId) || 'DEADBEEF'
  store.simulateRemoteStart(idTag, connectorId)
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
          <div v-if="connected" class="field"><span class="label">Registration</span><span class="reg-state" :class="regState">{{ regState }}</span></div>
        </div>

        <div class="section">
          <div class="section-header">
            <h3>Connection</h3>
          </div>
          <div class="connection-actions">
            <button v-if="!connected" class="btn-connect" @click="store.connectChargePoint()">Connect</button>
            <button v-else class="btn-disconnect" @click="store.disconnectChargePoint()">Disconnect</button>
            <button class="btn-action" :disabled="!connected" @click="store.bootChargePoint()">Boot</button>
            <button class="btn-action" :disabled="!registered" @click="store.heartbeatChargePoint()">Heartbeat</button>
          </div>
        </div>

        <div class="section">
          <div class="section-header">
            <h3>Connectors</h3>
            <button class="btn-small" @click="store.addConnector()">+ Add</button>
          </div>
          <div v-if="connectors.length === 0" class="empty-hint">No connectors.</div>
          <div v-else class="connector-list">
            <div v-for="c in connectors" :key="c.id" class="connector-card">
              <div class="connector-top">
                <span class="connector-num">Connector {{ c.connectorNumber }}</span>
                <span class="connector-status" :class="statusClass(connectorStatus(c.connectorNumber))">
                  {{ connectorStatus(c.connectorNumber) }}
                </span>
              </div>

              <!-- Available: Plug In, Fault, Disable -->
              <div v-if="connectorStatus(c.connectorNumber) === 'Available' && registered" class="connector-actions">
                <button class="btn-action btn-plug" @click="store.plugInConnector(c.connectorNumber)">Plug In</button>
                <button class="btn-action btn-fault" @click="store.setConnectorStatus(c.connectorNumber, 'Faulted')">Fault</button>
                <button class="btn-action" @click="store.setConnectorStatus(c.connectorNumber, 'Unavailable')">Disable</button>
              </div>

              <!-- Preparing: Authorize (with idTag input), Unplug, Fault, Simulate Remote Start -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Preparing'" class="connector-actions-col">
                <div class="auth-row">
                  <input
                    class="idtag-input"
                    type="text"
                    placeholder="idTag (e.g. ABCDEF12)"
                    :value="getIdTag(c.connectorNumber)"
                    @input="setIdTag(c.connectorNumber, ($event.target as HTMLInputElement).value)"
                    @keyup.enter="handleAuthorize(c.connectorNumber)"
                  />
                  <button class="btn-action btn-authorize" @click="handleAuthorize(c.connectorNumber)">Authorize</button>
                </div>
                <div class="action-row">
                  <button class="btn-action" @click="store.unplugConnector(c.connectorNumber)">Unplug</button>
                  <button class="btn-action btn-fault" @click="store.setConnectorStatus(c.connectorNumber, 'Faulted')">Fault</button>
                </div>
                <div class="dev-tools">
                  <span class="dev-label">Dev</span>
                  <button class="btn-action btn-dev" @click="handleSimulateRemoteStart(c.connectorNumber)">Simulate Remote Start</button>
                </div>
              </div>

              <!-- Charging: Stop, MeterValues, Fault -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Charging'" class="connector-actions">
                <button class="btn-action btn-stop" @click="store.stopConnectorTransaction(c.connectorNumber)">Stop</button>
                <button class="btn-action" @click="store.sendConnectorMeterValues(c.connectorNumber)">MeterValues</button>
                <button class="btn-action btn-fault" @click="store.setConnectorStatus(c.connectorNumber, 'Faulted')">Fault</button>
              </div>

              <!-- Finishing: Unplug -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Finishing'" class="connector-actions">
                <button class="btn-action btn-plug" @click="store.unplugConnector(c.connectorNumber)">Unplug</button>
              </div>

              <!-- Faulted: Clear -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Faulted'" class="connector-actions">
                <button class="btn-action" @click="store.setConnectorStatus(c.connectorNumber, 'Available')">Clear Fault</button>
              </div>

              <!-- Unavailable: Enable -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Unavailable'" class="connector-actions">
                <button class="btn-action" @click="store.setConnectorStatus(c.connectorNumber, 'Available')">Enable</button>
              </div>

              <!-- Default fallback (not registered) -->
              <div v-else-if="!registered" class="connector-actions">
                <span class="empty-hint">Register (Boot) to control connectors.</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.charge-point-detail { display: flex; flex-direction: column; height: 100%; background: #f5f5f8; }
.panel-header { display: flex; align-items: center; padding: 0.75rem 1rem; border-bottom: 1px solid #ddd; background: #fafafa; }
.panel-header h2 { font-size: 0.85rem; font-weight: 600; margin: 0; text-transform: uppercase; letter-spacing: 0.05em; color: #666; }
.panel-body { flex: 1; overflow-y: auto; padding: 1rem; }
.empty-state { text-align: center; margin-top: 2rem; color: #999; font-size: 0.85rem; }
.detail-content { display: flex; flex-direction: column; gap: 1.5rem; }
.section h3 { font-size: 0.9rem; color: #333; margin-bottom: 0.5rem; }
.section-header { display: flex; align-items: center; justify-content: space-between; }
.section-header h3 { margin-bottom: 0; }
.field { display: flex; justify-content: space-between; padding: 0.25rem 0; font-size: 0.8rem; }
.label { color: #999; }
.url { font-family: monospace; font-size: 0.75rem; color: #6366f1; }
.status { font-size: 0.7rem; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; }
.status.disconnected { background: #fde8e8; color: #c53030; }
.status.connected { background: #dcfce7; color: #16a34a; }
.reg-state { font-size: 0.7rem; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; font-weight: 600; }
.reg-state.pending { background: #fef9c3; color: #a16207; }
.reg-state.accepted { background: #dcfce7; color: #16a34a; }
.reg-state.rejected { background: #fde8e8; color: #c53030; }
.btn-small { padding: 0.25rem 0.5rem; font-size: 0.75rem; border: 1px solid #ccc; border-radius: 4px; background: transparent; color: #666; cursor: pointer; }
.btn-small:hover { background: #eee; color: #333; }
.empty-hint { font-size: 0.8rem; color: #999; }
.connector-list { display: flex; flex-direction: column; gap: 0.5rem; }
.connector-card { padding: 0.5rem 0.75rem; border: 1px solid #ddd; border-radius: 6px; background: #fff; }
.connector-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem; }
.connector-num { font-size: 0.8rem; font-weight: 600; color: #333; }
.connector-status { font-size: 0.65rem; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; }
.connector-status.available { background: #dcfce7; color: #16a34a; }
.connector-status.charging { background: #dbeafe; color: #2563eb; }
.connector-status.preparing { background: #fef9c3; color: #a16207; }
.connector-status.finishing { background: #e8e8f0; color: #6b7280; }
.connector-status.faulted { background: #fde8e8; color: #c53030; }
.connector-status.unavailable { background: #f3f4f6; color: #9ca3af; }
.connector-actions { display: flex; gap: 0.25rem; flex-wrap: wrap; }
.connector-actions-col { display: flex; flex-direction: column; gap: 0.35rem; }
.connection-actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
.btn-connect { padding: 0.4rem 1rem; font-size: 0.8rem; border: 1px solid #16a34a; border-radius: 4px; background: #16a34a; color: #fff; cursor: pointer; font-weight: 600; }
.btn-connect:hover { background: #15803d; }
.btn-disconnect { padding: 0.4rem 1rem; font-size: 0.8rem; border: 1px solid #dc2626; border-radius: 4px; background: #dc2626; color: #fff; cursor: pointer; font-weight: 600; }
.btn-disconnect:hover { background: #b91c1c; }
.btn-action { padding: 0.2rem 0.5rem; font-size: 0.65rem; border: 1px solid #ddd; border-radius: 3px; background: #fff; color: #666; cursor: pointer; }
.btn-action:hover { background: #f5f5f8; color: #333; }
.btn-action:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-action.btn-stop { border-color: #fca5a5; color: #dc2626; }
.btn-action.btn-stop:hover { background: #fef2f2; }
.btn-action.btn-fault { border-color: #fca5a5; color: #dc2626; }
.btn-action.btn-plug { border-color: #86efac; color: #16a34a; }
.btn-action.btn-plug:hover { background: #f0fdf4; }
.btn-action.btn-authorize { border-color: #93c5fd; color: #2563eb; font-weight: 600; }
.btn-action.btn-authorize:hover { background: #eff6ff; }
.btn-action.btn-dev { border-color: #d8b4fe; color: #7c3aed; font-style: italic; font-size: 0.6rem; }
.btn-action.btn-dev:hover { background: #faf5ff; }
.auth-row { display: flex; gap: 0.25rem; align-items: center; }
.action-row { display: flex; gap: 0.25rem; }
.dev-tools { display: flex; gap: 0.25rem; align-items: center; margin-top: 0.15rem; padding-top: 0.25rem; border-top: 1px dashed #e5e7eb; }
.dev-label { font-size: 0.55rem; color: #9ca3af; text-transform: uppercase; letter-spacing: 0.05em; }
.idtag-input { padding: 0.2rem 0.4rem; font-size: 0.65rem; border: 1px solid #d1d5db; border-radius: 3px; width: 120px; font-family: monospace; }
.idtag-input:focus { outline: none; border-color: #93c5fd; box-shadow: 0 0 0 1px #93c5fd; }
</style>
