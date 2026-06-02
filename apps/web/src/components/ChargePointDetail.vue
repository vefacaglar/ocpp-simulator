<script setup lang="ts">
import { useChargePointStore } from '../stores/chargePointStore'

const store = useChargePointStore()

async function onAddConnector() {
  await store.addConnector()
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
            <button class="btn-small" @click="onAddConnector">+ Add</button>
          </div>
          <div v-if="store.selectedDetail.connectors.length === 0" class="empty-hint">No connectors.</div>
          <div v-else class="connector-list">
            <div v-for="c in store.selectedDetail.connectors" :key="c.id" class="connector-card">
              <div class="connector-top">
                <span class="connector-num">Connector {{ c.connectorNumber }}</span>
                <span class="connector-status" :class="c.status.toLowerCase()">{{ c.status }}</span>
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
.connector-top { display: flex; align-items: center; justify-content: space-between; }
.connector-num { font-size: 0.8rem; font-weight: 600; color: #c0c0e0; }
.connector-status { font-size: 0.65rem; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; }
.connector-status.available { background: #203a20; color: #60e060; }
.connector-status.charging { background: #203050; color: #60a0e0; }
.connector-status.preparing { background: #3a3a20; color: #e0c060; }
.connector-status.faulted { background: #3a2020; color: #e06060; }
</style>
