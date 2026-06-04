<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useChargePointStore } from '../stores/chargePointStore'

const props = defineProps<{
  editId?: string
}>()

const emit = defineEmits<{ close: [] }>()
const store = useChargePointStore()

const id = ref('')
const name = ref('')
const ocppVersion = ref('1.6J')

onMounted(() => {
  if (props.editId) {
    const cp = store.chargePoints.find(cp => cp.id === props.editId)
    if (cp) {
      id.value = cp.id
      name.value = cp.name
      ocppVersion.value = cp.ocppVersion
    }
  }
})

async function onSubmit() {
  if (!id.value) return
  if (props.editId) {
    await store.updateChargePoint(props.editId, {
      name: name.value || id.value,
      ocppVersion: ocppVersion.value,
    })
  } else {
    await store.createChargePoint({
      id: id.value,
      name: name.value || id.value,
      ocppVersion: ocppVersion.value,
    })
  }
  emit('close')
}
</script>

<template>
  <Transition name="modal" appear>
    <div class="modal-overlay" @click.self="emit('close')">
      <div class="modal">
      <div class="modal-header">
        <div>
          <div class="eyebrow">— {{ props.editId ? 'Edit Instance' : 'New Instance' }}</div>
          <h3>{{ props.editId ? 'Edit Charge Point' : 'Add Charge Point' }}</h3>
        </div>
        <button class="modal-close" @click="emit('close')">&times;</button>
      </div>
      <form class="modal-body" @submit.prevent="onSubmit">
        <div class="form-field">
          <label>Charge Point ID</label>
          <input v-model="id" placeholder="CP-001" required :disabled="!!props.editId" />
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
        <div class="form-actions">
          <button type="button" class="btn btn-cancel" @click="emit('close')">Cancel</button>
          <button type="submit" class="btn btn-primary">{{ props.editId ? 'Save' : 'Create' }}</button>
        </div>
      </form>
    </div>
  </div>
  </Transition>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-lg);
  width: 440px;
  max-width: 90vw;
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 24px 28px 20px 28px;
  border-bottom: 1px solid var(--border-subtle);
}

.eyebrow {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  margin-bottom: 6px;
}

.modal-header h3 {
  font-family: var(--font-mono);
  font-size: 0.9rem;
  font-weight: 500;
  margin: 0;
  color: var(--text-primary);
  letter-spacing: 0.02em;
}

.modal-close {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 1.2rem;
  cursor: pointer;
  transition: color 0.15s;
  margin-top: -4px;
}
.modal-close:hover { color: var(--text-primary); }

.modal-body {
  padding: 24px 28px 28px 28px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-field label {
  font-family: var(--font-mono);
  font-size: 0.74rem;
  letter-spacing: 0.02em;
  color: var(--text-muted);
}

.form-field input,
.form-field select {
  padding: 10px 12px;
  background: var(--bg-sunken);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 0.82rem;
  letter-spacing: 0.02em;
  transition: border-color 0.15s;
}
.form-field input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.form-field input::placeholder { color: var(--text-muted); }
.form-field input:focus:not(:disabled),
.form-field select:focus { border-color: var(--accent); }
.form-field select option {
  background: var(--bg-elevated);
  color: var(--text-primary);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

.btn {
  font-family: var(--font-mono);
  font-size: 0.78rem;
  font-weight: 500;
  letter-spacing: 0.02em;
  padding: 9px 16px;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.btn:hover {
  color: var(--text-primary);
  border-color: rgba(244, 241, 234, 0.22);
}

.btn-cancel { color: var(--text-muted); }
.btn-cancel:hover {
  color: var(--text-secondary);
  border-color: var(--border-strong);
}

.btn-primary {
  background: var(--accent);
  color: var(--accent-ink);
  border-color: var(--accent);
}
.btn-primary:hover {
  background: #efe7d0;
  color: var(--accent-ink);
  border-color: #efe7d0;
}

/* Modal Animation */
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-active .modal,
.modal-leave-active .modal {
  transition: transform 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.modal-enter-from .modal,
.modal-leave-to .modal {
  transform: scale(0.95);
}
</style>

