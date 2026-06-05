<script setup lang="ts">
import { ref, computed } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'
import { useSettingsStore } from '../stores/settingsStore'

const store = useChargePointStore()
const settings = useSettingsStore()

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

const ocppVersion = computed(() => store.selectedDetail?.chargePoint.ocppVersion ?? '1.6J')

// activeTransactionFor returns a snapshot of the transaction
// state for the given connector, if any. Both 1.6J (numeric
// id) and 2.0.1 (string GUID) surface uniformly: the
// transactionId is rendered as-is.
function activeTransactionFor(connectorId: number) {
  return store.activeTransactionFor(connectorId)
}
</script>

<template>
  <div class="charge-point-detail">
    <div class="panel-body">
      <div v-if="!store.selectedDetail" class="empty-state">
        <p class="empty-title">No charge point selected.</p>
        <p class="hint">Pick one from the left to begin.</p>
      </div>
      <div v-else class="detail-content">
        <div class="section">
          <div class="section-eyebrow">— Identity</div>
          <h3 class="section-title">{{ store.selectedDetail.chargePoint.id }}</h3>
          <div class="kv-list">
            <div class="kv"><span class="kv-label">Name</span><span class="kv-value">{{ store.selectedDetail.chargePoint.name }}</span></div>
            <div class="kv"><span class="kv-label">Version</span><span class="kv-value">{{ store.selectedDetail.chargePoint.ocppVersion }}</span></div>
            <div class="kv"><span class="kv-label">CSMS URL</span><span class="kv-value mono">{{ settings.centralSystemUrl }}</span></div>
            <div class="kv">
              <span class="kv-label">Status</span>
              <span class="status-pill" :class="store.getConnectionStatus(store.selectedDetail.chargePoint.id)">
                {{ store.getConnectionStatus(store.selectedDetail.chargePoint.id) }}
              </span>
            </div>
            <div v-if="connected" class="kv">
              <span class="kv-label">Registration</span>
              <span class="status-pill" :class="regState">{{ regState }}</span>
            </div>
          </div>
        </div>

        <div class="section">
          <div class="section-eyebrow">— Connection</div>
          <div class="connection-actions">
            <button v-if="!connected" class="btn btn-primary" @click="store.connectChargePoint()">Connect</button>
            <button v-else class="btn btn-danger" @click="store.disconnectChargePoint()">Disconnect</button>
            <button class="btn" :disabled="!connected" @click="store.bootChargePoint()">Boot</button>
            <button class="btn" :disabled="!registered" @click="store.heartbeatChargePoint()">Heartbeat</button>
          </div>
        </div>

        <div class="section">
          <div class="section-header">
            <div>
              <div class="section-eyebrow">— Connectors</div>
              <h3 class="section-title">Active Ports</h3>
            </div>
            <button class="btn btn-small" @click="store.addConnector()">+ Add</button>
          </div>
          <div v-if="connectors.length === 0" class="empty-hint">No connectors.</div>
          <div v-else class="connector-list">
            <div v-for="c in connectors" :key="c.id" class="connector-card">
              <div class="connector-top">
                <span class="connector-num">Connector {{ c.connectorNumber }}</span>
                <div class="connector-top-actions">
                  <span class="status-pill" :class="statusClass(connectorStatus(c.connectorNumber))">
                    {{ connectorStatus(c.connectorNumber) }}
                  </span>
                  <button class="btn-delete" title="Delete Connector" @click="store.removeConnector(c.connectorNumber)">&times;</button>
                </div>
              </div>

              <!-- Available: Plug In, Fault, Disable -->
              <div v-if="connectorStatus(c.connectorNumber) === 'Available' && registered" class="connector-actions">
                <button class="btn" @click="store.plugInConnector(c.connectorNumber)">Plug In</button>
                <button class="btn btn-danger-soft" @click="store.setConnectorStatus(c.connectorNumber, 'Faulted')">Fault</button>
                <button class="btn" @click="store.setConnectorStatus(c.connectorNumber, 'Unavailable')">Disable</button>
              </div>

              <!-- Preparing: Authorize (with idTag input), Unplug, Fault -->
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
                  <button class="btn btn-primary" @click="handleAuthorize(c.connectorNumber)">Authorize</button>
                </div>
                <div class="action-row">
                  <button class="btn" @click="store.unplugConnector(c.connectorNumber)">Unplug</button>
                  <button class="btn btn-danger-soft" @click="store.setConnectorStatus(c.connectorNumber, 'Faulted')">Fault</button>
                </div>
              </div>

              <!-- Charging: Stop, MeterValues, Fault -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Charging'" class="connector-actions">
                <button class="btn btn-danger-soft" @click="store.stopConnectorTransaction(c.connectorNumber)">Stop</button>
                <button class="btn" @click="store.sendConnectorMeterValues(c.connectorNumber)">MeterValues</button>
                <button class="btn btn-danger-soft" @click="store.setConnectorStatus(c.connectorNumber, 'Faulted')">Fault</button>
                <div class="connector-meta">
                  <span class="kv-inline">
                    <span class="kv-label">tx:</span>
                    <span class="kv-value">{{ activeTransactionFor(c.connectorNumber)?.transactionId ?? '—' }}</span>
                  </span>
                  <span v-if="ocppVersion === '2.0.1'" class="kv-inline">
                    <span class="kv-label">evseId:</span>
                    <span class="kv-value">{{ c.evseId }}</span>
                  </span>
                  <span v-if="ocppVersion === '2.0.1'" class="kv-inline">
                    <span class="kv-label">seqNo:</span>
                    <span class="kv-value">{{ activeTransactionFor(c.connectorNumber)?.seqNo ?? '—' }}</span>
                  </span>
                </div>
              </div>

              <!-- Finishing: Unplug -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Finishing'" class="connector-actions">
                <button class="btn" @click="store.unplugConnector(c.connectorNumber)">Unplug</button>
              </div>

              <!-- Faulted: Clear -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Faulted'" class="connector-actions">
                <button class="btn" @click="store.setConnectorStatus(c.connectorNumber, 'Available')">Clear Fault</button>
              </div>

              <!-- Unavailable: Enable -->
              <div v-else-if="connectorStatus(c.connectorNumber) === 'Unavailable'" class="connector-actions">
                <button class="btn" @click="store.setConnectorStatus(c.connectorNumber, 'Available')">Enable</button>
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
.charge-point-detail {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-center);
  border-left: 1px solid var(--border-strong);
  border-right: 1px solid var(--border-strong);
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 32px 40px 40px 40px;
}

.empty-state {
  text-align: center;
  margin-top: 4rem;
}

.empty-title {
  font-family: var(--font-mono);
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--text-secondary);
  letter-spacing: 0.04em;
}

.empty-state .hint {
  margin-top: 8px;
  font-size: 0.78rem;
  color: var(--text-muted);
  letter-spacing: 0.02em;
}

.detail-content {
  display: flex;
  flex-direction: column;
  gap: 40px;
  max-width: 720px;
}

.section-eyebrow {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.section-title {
  font-family: var(--font-mono);
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 20px 0;
  letter-spacing: 0.04em;
}

.section-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}

.kv-list {
  display: flex;
  flex-direction: column;
  gap: 0;
  border-top: 1px solid var(--border-subtle);
}

.kv {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-subtle);
  font-size: 0.82rem;
}

.kv-label {
  font-family: var(--font-mono);
  font-size: 0.74rem;
  letter-spacing: 0.02em;
  color: var(--text-muted);
}

.kv-value {
  color: var(--text-primary);
}

.kv-value.mono {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  color: var(--text-secondary);
}

.status-pill {
  font-family: var(--font-mono);
  font-size: 0.62rem;
  font-weight: 500;
  letter-spacing: 0.02em;
  padding: 3px 9px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-strong);
  color: var(--text-secondary);
  background: transparent;
  white-space: nowrap;
}

.status-pill.disconnected { color: var(--status-fault); border-color: rgba(214, 139, 110, 0.35); }
.status-pill.connecting   { color: var(--status-busy);  border-color: rgba(212, 165, 116, 0.35); }

.status-pill.connected,
.status-pill.available    { color: var(--status-online); border-color: rgba(184, 196, 160, 0.35); }
.status-pill.preparing    { color: var(--status-busy);  border-color: rgba(212, 165, 116, 0.35); }
.status-pill.charging     { color: var(--accent);       border-color: rgba(232, 223, 200, 0.40); }
.status-pill.finishing    { color: var(--text-muted);   border-color: var(--border-subtle); }
.status-pill.faulted      { color: var(--status-fault); border-color: rgba(214, 139, 110, 0.40); background: rgba(214, 139, 110, 0.08); }
.status-pill.unavailable  { color: var(--text-muted);   border-color: var(--border-subtle); }
.status-pill.pending      { color: var(--status-busy);  border-color: rgba(212, 165, 116, 0.35); }
.status-pill.accepted     { color: var(--status-online); border-color: rgba(184, 196, 160, 0.35); }
.status-pill.rejected     { color: var(--status-fault); border-color: rgba(214, 139, 110, 0.40); }

.connection-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.btn {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  font-weight: 500;
  letter-spacing: 0.02em;
  padding: 8px 14px;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}

.btn:hover:not(:disabled) {
  color: var(--text-primary);
  border-color: rgba(244, 241, 234, 0.22);
  background: rgba(244, 241, 234, 0.04);
}

.btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--accent);
  color: var(--accent-ink);
  border-color: var(--accent);
}
.btn-primary:hover:not(:disabled) {
  background: #efe7d0;
  color: var(--accent-ink);
  border-color: #efe7d0;
}

.btn-danger {
  border-color: var(--danger);
  color: var(--danger);
}
.btn-danger:hover:not(:disabled) {
  background: rgba(214, 139, 110, 0.10);
}

.btn-danger-soft {
  border-color: rgba(214, 139, 110, 0.40);
  color: var(--status-fault);
}
.btn-danger-soft:hover:not(:disabled) {
  background: rgba(214, 139, 110, 0.08);
  border-color: rgba(214, 139, 110, 0.55);
}

.btn-small {
  font-size: 0.65rem;
  padding: 5px 10px;
}

.empty-hint {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  color: var(--text-muted);
  letter-spacing: 0.02em;
}

.connector-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 16px;
}

.connector-card {
  padding: 16px 18px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-elevated);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.connector-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.connector-num {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  color: var(--text-primary);
  font-weight: 600;
  letter-spacing: 0.04em;
}

.connector-top-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.btn-delete {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1.1rem;
  padding: 0 4px;
  line-height: 1;
  transition: color 0.15s;
}
.btn-delete:hover {
  color: var(--danger);
}

.connector-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.connector-meta {
  display: flex;
  gap: 10px;
  flex-basis: 100%;
  font-family: var(--font-mono);
  font-size: 0.7rem;
  color: var(--text-muted);
  margin-top: 4px;
}

.kv-inline {
  display: inline-flex;
  gap: 4px;
  align-items: baseline;
}

.kv-inline .kv-label {
  color: var(--text-muted);
  letter-spacing: 0.02em;
}

.kv-inline .kv-value {
  color: var(--text-primary);
}

.connector-actions-col {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.auth-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.action-row {
  display: flex;
  gap: 6px;
}

.idtag-input {
  flex: 1;
  padding: 8px 12px;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-primary);
  background: var(--bg-sunken);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  letter-spacing: 0.04em;
  transition: border-color 0.15s;
}
.idtag-input::placeholder {
  color: var(--text-muted);
}
.idtag-input:focus {
  border-color: var(--border-strong);
}

@media (max-width: 768px) {
  .panel-body {
    padding: 20px 16px 24px 16px;
  }
  .detail-content {
    gap: 24px;
  }
}

@media (max-width: 480px) {
  .auth-row {
    flex-direction: column;
    align-items: stretch;
  }
  .idtag-input {
    width: 100%;
  }
}
</style>
