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
          v-for="cp in store.chargePoints"
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
.charge-point-list { display: flex; flex-direction: column; height: 100%; background: #12121f; border-right: 1px solid #2a2a3a; }
.panel-header { display: flex; align-items: center; justify-content: space-between; padding: 0.75rem 1rem; border-bottom: 1px solid #2a2a3a; background: #16162a; }
.panel-header h2 { font-size: 0.85rem; font-weight: 600; margin: 0; text-transform: uppercase; letter-spacing: 0.05em; color: #8888aa; }
.btn-add { width: 24px; height: 24px; border: 1px solid #4a4a6a; border-radius: 4px; background: transparent; color: #8888aa; font-size: 1rem; cursor: pointer; display: flex; align-items: center; justify-content: center; }
.btn-add:hover { background: #2a2a4a; color: #e0e0e0; }
.panel-body { flex: 1; overflow-y: auto; padding: 0.5rem; }
.empty-state { text-align: center; margin-top: 2rem; color: #555; font-size: 0.85rem; }
.empty-state .hint { margin-top: 0.25rem; font-size: 0.75rem; color: #444; }
.cp-items { display: flex; flex-direction: column; gap: 0.25rem; }
.cp-item { padding: 0.5rem 0.75rem; border-radius: 6px; cursor: pointer; border: 1px solid transparent; transition: all 0.15s; }
.cp-item:hover { background: #1a1a30; }
.cp-item.selected { background: #1e1e3a; border-color: #4a4a8a; }
.cp-info { display: flex; align-items: center; justify-content: space-between; }
.cp-id { font-size: 0.85rem; font-weight: 600; color: #e0e0e0; }
.cp-meta { display: flex; align-items: center; justify-content: space-between; margin-top: 0.25rem; font-size: 0.7rem; color: #666; }
.btn-delete { background: none; border: none; color: #666; cursor: pointer; font-size: 1rem; padding: 0 4px; }
.btn-delete:hover { color: #e06060; }
.error-bar { padding: 0.5rem 1rem; background: #3a2020; color: #e06060; font-size: 0.75rem; border-top: 1px solid #4a2020; }
</style>
