<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'
import RuntimeStatusBadge from './RuntimeStatusBadge.vue'
import AddChargePointModal from './AddChargePointModal.vue'

const store = useChargePointStore()
const showModal = ref(false)

onMounted(() => store.loadChargePoints())

function onSelect(id: string) {
  store.selectChargePoint(id)
}

async function onDelete(id: string, e: Event) {
  e.stopPropagation()
  await store.removeChargePoint(id)
}
</script>

<template>
  <div class="charge-point-list">
    <div class="panel-header">
      <h2>Charge Points</h2>
      <button class="btn-add" title="Add Charge Point" @click="showModal = true">+</button>
    </div>
    <div class="panel-body">
      <div v-if="store.loading" class="empty-state">Loading...</div>
      <div v-else-if="store.chargePoints.length === 0" class="empty-state">
        <p>No charge points yet.</p>
        <p class="hint">Click + to add one.</p>
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
            <button class="btn-delete" title="Delete" @click="onDelete(cp.id, $event)">&times;</button>
          </div>
        </div>
      </div>
    </div>
    <div v-if="store.error" class="error-bar">{{ store.error }}</div>
    <AddChargePointModal v-if="showModal" @close="showModal = false" />
  </div>
</template>

<style scoped>
.charge-point-list { display: flex; flex-direction: column; height: 100%; background: #fff; border-right: 1px solid #ddd; }
.panel-header { display: flex; align-items: center; justify-content: space-between; padding: 0.75rem 1rem; border-bottom: 1px solid #ddd; background: #fafafa; }
.panel-header h2 { font-size: 0.85rem; font-weight: 600; margin: 0; text-transform: uppercase; letter-spacing: 0.05em; color: #666; }
.btn-add { width: 24px; height: 24px; border: 1px solid #ccc; border-radius: 4px; background: transparent; color: #666; font-size: 1rem; cursor: pointer; display: flex; align-items: center; justify-content: center; }
.btn-add:hover { background: #eee; color: #333; }
.panel-body { flex: 1; overflow-y: auto; padding: 0.5rem; }
.empty-state { text-align: center; margin-top: 2rem; color: #999; font-size: 0.85rem; }
.empty-state .hint { margin-top: 0.25rem; font-size: 0.75rem; color: #bbb; }
.cp-items { display: flex; flex-direction: column; gap: 0.25rem; }
.cp-item { padding: 0.5rem 0.75rem; border-radius: 6px; cursor: pointer; border: 1px solid transparent; transition: all 0.15s; }
.cp-item:hover { background: #f0f0f5; }
.cp-item.selected { background: #e8e8f4; border-color: #b0b0d0; }
.cp-info { display: flex; align-items: center; justify-content: space-between; }
.cp-id { font-size: 0.85rem; font-weight: 600; color: #1a1a2e; }
.cp-meta { display: flex; align-items: center; justify-content: space-between; margin-top: 0.25rem; font-size: 0.7rem; color: #999; }
.btn-delete { background: none; border: none; color: #999; cursor: pointer; font-size: 1rem; padding: 0 4px; }
.btn-delete:hover { color: #e06060; }
.error-bar { padding: 0.5rem 1rem; background: #fde8e8; color: #c53030; font-size: 0.75rem; border-top: 1px solid #fcd5d5; }
</style>
