<script setup lang="ts">
import { computed } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'

const store = useChargePointStore()

const connectors = computed(() => store.selectedDetail?.connectors ?? [])
const connected = computed(() => store.selectedId ? store.isConnected(store.selectedId) : false)
const registered = computed(() => store.isRegistered)
const regState = computed(() => store.registrationState)

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
                <span class="connector-status" :class="statusClass(c.status)">{{ c.status }}</span>
              </div>
              <div class="connector-actions">
                <button v-if="c.status === 'Available' && registered" class="btn-action" @click="store.startConnectorTransaction(c.connectorNumber)">Start TX</button>
                <button v-if="c.status === 'Charging'" class="btn-action btn-stop" @click="store.stopConnectorTransaction(c.connectorNumber)">Stop TX</button>
                <button v-if="c.status === 'Charging'" class="btn-action" @click="store.sendConnectorMeterValues(c.connectorNumber)">MeterValues</button>
                <button v-if="c.status === 'Available' && registered" class="btn-action btn-fault" @click="store.setConnectorStatus(c.connectorNumber, 'Faulted')">Fault</button>
                <button v-if="c.status === 'Faulted'" class="btn-action" @click="store.setConnectorStatus(c.connectorNumber, 'Available')">Clear</button>
                <button v-if="c.status === 'Available' && registered" class="btn-action" @click="store.setConnectorStatus(c.connectorNumber, 'Unavailable')">Disable</button>
                <button v-if="c.status === 'Unavailable'" class="btn-action" @click="store.setConnectorStatus(c.connectorNumber, 'Available')">Enable</button>
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
</style>
