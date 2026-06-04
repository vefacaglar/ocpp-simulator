<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'
import RuntimeStatusBadge from './RuntimeStatusBadge.vue'
import AddChargePointModal from './AddChargePointModal.vue'

const store = useChargePointStore()
const showModal = ref(false)
const editId = ref<string | undefined>(undefined)

onMounted(() => store.loadChargePoints())

const hasConnected = computed(() =>
  store.chargePoints.some((cp) => store.isConnected(cp.id))
)
const hasDisconnected = computed(() =>
  store.chargePoints.some((cp) => !store.isConnected(cp.id))
)

function onSelect(id: string) {
  store.selectChargePoint(id)
}

async function onDelete(id: string, e: Event) {
  e.stopPropagation()
  await store.removeChargePoint(id)
}

function onEdit(id: string, e: Event) {
  e.stopPropagation()
  editId.value = id
  showModal.value = true
}

function openAddModal() {
  editId.value = undefined
  showModal.value = true
}
</script>

<template>
  <div class="charge-point-list">
    <div class="panel-header">
      <h2>Charge Points</h2>
      <button class="btn-add" title="Add Charge Point" @click="openAddModal">+</button>
    </div>
    <div v-if="store.chargePoints.length > 0" class="bulk-actions">
      <button class="btn-bulk" :disabled="!hasDisconnected" @click="store.connectAll()">Connect All</button>
      <button class="btn-bulk" :disabled="!hasConnected" @click="store.disconnectAll()">Disconnect All</button>
    </div>
    <div class="panel-body">
      <div v-if="store.loading" class="empty-state">Loading…</div>
      <div v-else-if="store.chargePoints.length === 0" class="empty-state">
        <p class="empty-title">No charge points yet.</p>
        <p class="hint">Press + to add one.</p>
      </div>
      <div v-else class="cp-items">
        <div
          v-for="cp in store.chargePointsWithStatus"
          :key="cp.id"
          class="cp-item"
          :class="{ selected: cp.id === store.selectedId }"
          @click="onSelect(cp.id)"
        >
          <div class="cp-info">
            <span class="cp-id">{{ cp.id }}</span>
            <RuntimeStatusBadge :status="cp.status" />
          </div>
          <div class="cp-meta">
            <span>{{ cp.ocppVersion }}</span>
            <div class="cp-actions">
              <button class="btn-icon" title="Edit" @click="onEdit(cp.id, $event)">&#9998;</button>
              <button class="btn-icon btn-delete" title="Delete" @click="onDelete(cp.id, $event)">&times;</button>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div v-if="store.error" class="error-bar">{{ store.error }}</div>
    <AddChargePointModal v-if="showModal" :edit-id="editId" @close="showModal = false" />
  </div>
</template>

<style scoped>
.charge-point-list {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-elevated);
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

.btn-add {
  width: 28px;
  height: 28px;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.9rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.btn-add:hover {
  background: var(--accent-soft);
  border-color: var(--accent);
  color: var(--accent);
}

.bulk-actions {
  display: flex;
  gap: 6px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border-subtle);
}

.btn-bulk {
  flex: 1;
  padding: 5px 8px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.btn-bulk:hover:not(:disabled) {
  color: var(--text-secondary);
  border-color: var(--border-strong);
  background: var(--accent-soft);
}
.btn-bulk:disabled {
  opacity: 0.35;
  cursor: default;
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 16px;
}

.empty-state {
  text-align: center;
  margin-top: 3rem;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.empty-title {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  color: var(--text-secondary);
  font-weight: 500;
  letter-spacing: 0.04em;
}

.empty-state .hint {
  margin-top: 6px;
  font-size: 0.7rem;
  color: var(--text-muted);
}

.cp-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.cp-item {
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  border: 1px solid transparent;
  background: transparent;
  transition: background 0.15s, border-color 0.15s;
}
.cp-item:hover {
  background: var(--accent-soft);
  border-color: var(--border-subtle);
}
.cp-item.selected {
  background: var(--accent-soft);
  border-color: var(--border-strong);
}

.cp-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.cp-id {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  font-weight: 500;
  color: var(--text-primary);
  letter-spacing: 0.01em;
}

.cp-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 6px;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: var(--text-muted);
  letter-spacing: 0.02em;
}

.cp-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.btn-icon {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.95rem;
  padding: 0 4px;
  line-height: 1;
  transition: color 0.15s;
}
.btn-icon:hover {
  color: var(--text-primary);
}

.btn-icon.btn-delete {
  font-size: 1.1rem;
}
.btn-icon.btn-delete:hover {
  color: var(--danger);
}

.error-bar {
  padding: 10px 24px;
  background: rgba(214, 139, 110, 0.10);
  color: var(--status-fault);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  border-top: 1px solid var(--border-subtle);
  letter-spacing: 0.04em;
}
</style>
