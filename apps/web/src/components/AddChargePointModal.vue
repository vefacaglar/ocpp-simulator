<script setup lang="ts">
import { ref } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'

const emit = defineEmits<{ close: [] }>()
const store = useChargePointStore()

const id = ref('')
const name = ref('')
const ocppVersion = ref('1.6J')
const centralSystemUrl = ref('ws://localhost:8080/ocpp')

async function onSubmit() {
  if (!id.value) return
  await store.createChargePoint({
    id: id.value,
    name: name.value || id.value,
    ocppVersion: ocppVersion.value,
    centralSystemUrl: centralSystemUrl.value,
  })
  emit('close')
}
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal">
      <div class="modal-header">
        <h3>Add Charge Point</h3>
        <button class="modal-close" @click="emit('close')">&times;</button>
      </div>
      <form class="modal-body" @submit.prevent="onSubmit">
        <div class="form-field">
          <label>Charge Point ID</label>
          <input v-model="id" placeholder="CP-001" required />
        </div>
        <div class="form-field">
          <label>Name</label>
          <input v-model="name" placeholder="Optional name" />
        </div>
        <div class="form-field">
          <label>OCPP Version</label>
          <select v-model="ocppVersion">
            <option value="1.6J">OCPP 1.6J</option>
            <option value="2.0.1" disabled>OCPP 2.0.1 (planned)</option>
          </select>
        </div>
        <div class="form-field">
          <label>Central System URL</label>
          <input v-model="centralSystemUrl" placeholder="ws://localhost:8080/ocpp" />
        </div>
        <div class="form-actions">
          <button type="button" class="btn-cancel" @click="emit('close')">Cancel</button>
          <button type="submit" class="btn-submit">Create</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: #1a1a2e; border: 1px solid #3a3a5a; border-radius: 8px; width: 400px; max-width: 90vw; }
.modal-header { display: flex; align-items: center; justify-content: space-between; padding: 1rem; border-bottom: 1px solid #2a2a3a; }
.modal-header h3 { margin: 0; font-size: 0.95rem; color: #e0e0e0; }
.modal-close { background: none; border: none; color: #888; font-size: 1.2rem; cursor: pointer; }
.modal-close:hover { color: #e0e0e0; }
.modal-body { padding: 1rem; display: flex; flex-direction: column; gap: 0.75rem; }
.form-field { display: flex; flex-direction: column; gap: 0.25rem; }
.form-field label { font-size: 0.75rem; color: #8888aa; text-transform: uppercase; letter-spacing: 0.05em; }
.form-field input, .form-field select { padding: 0.5rem; background: #0f0f1a; border: 1px solid #2a2a3a; border-radius: 4px; color: #e0e0e0; font-size: 0.85rem; }
.form-field input:focus, .form-field select:focus { outline: none; border-color: #4a4a8a; }
.form-actions { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 0.5rem; }
.btn-cancel { padding: 0.4rem 0.8rem; border: 1px solid #3a3a5a; border-radius: 4px; background: transparent; color: #888; cursor: pointer; font-size: 0.8rem; }
.btn-cancel:hover { background: #2a2a3a; }
.btn-submit { padding: 0.4rem 0.8rem; border: 1px solid #4a4a8a; border-radius: 4px; background: #2a2a5a; color: #c0c0e0; cursor: pointer; font-size: 0.8rem; }
.btn-submit:hover { background: #3a3a6a; }
</style>
