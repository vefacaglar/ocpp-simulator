<script setup lang="ts">
import { ref, watch } from 'vue'
import { useSettingsStore } from '../stores/settingsStore'
import { exportBrowserDb, importBrowserDb, clearBrowserDb, type BrowserDbExport } from '../db/browserDb'

const settings = useSettingsStore()
const draftUrl = ref(settings.centralSystemUrl)
const saved = ref(false)
const importInput = ref<HTMLInputElement | null>(null)
const importError = ref<string | null>(null)
const showClearConfirm = ref(false)
const clearError = ref<string | null>(null)
const isClearing = ref(false)
const clearSuccess = ref(false)

watch(
  () => settings.centralSystemUrl,
  (value) => {
    draftUrl.value = value
  }
)

async function save() {
  await settings.saveCentralSystemUrl(draftUrl.value)
  saved.value = true
  window.setTimeout(() => {
    saved.value = false
  }, 1800)
}

async function exportData() {
  const data = await exportBrowserDb()
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  const dateStr = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
  link.download = `ocpp-simulator-${dateStr}.json`
  link.click()
  URL.revokeObjectURL(url)
}

function openImportPicker() {
  importError.value = null
  importInput.value?.click()
}

async function importData(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    const text = await file.text()
    const data = JSON.parse(text) as BrowserDbExport
    await importBrowserDb(data)
    window.location.reload()
  } catch (err: any) {
    importError.value = err.message || 'Import failed'
  } finally {
    input.value = ''
  }
}

async function confirmClear() {
  isClearing.value = true
  clearError.value = null
  try {
    await clearBrowserDb()
    clearSuccess.value = true
    setTimeout(() => {
      window.location.reload()
    }, 1500)
  } catch (err: any) {
    clearError.value = err.message || 'Failed to clear database'
    isClearing.value = false
  }
}
</script>

<template>
  <div class="settings-page">
    <section class="settings-section">
      <div class="section-eyebrow">— Connection</div>
      <h2>Settings</h2>
      <div class="form-field">
        <label>Central System URL</label>
        <input
          v-model="draftUrl"
          placeholder="wss://example.com/ocpp/{chargePointId}"
          spellcheck="false"
        />
      </div>
      <p class="hint">
        Use <span>{chargePointId}</span> where the selected charge point id should be inserted.
      </p>
      <div class="actions">
        <button class="btn btn-primary" @click="save">Save</button>
        <span v-if="saved" class="saved">Saved</span>
      </div>
    </section>

    <section class="settings-section data-section">
      <div class="section-eyebrow">— Data</div>
      <h2>Import / Export</h2>
      <p class="hint">
        Export or restore charge points, connectors, selected state, settings, and live OCPP logs.
      </p>
      <div class="actions">
        <button class="btn" @click="exportData">Export JSON</button>
        <button class="btn" @click="openImportPicker">Import JSON</button>
        <button class="btn btn-danger" @click="showClearConfirm = true">Clear Database</button>
        <input ref="importInput" class="file-input" type="file" accept="application/json,.json" @change="importData" />
      </div>
      <p v-if="importError" class="error">{{ importError }}</p>
    </section>

    <!-- Clear Confirm Modal -->
    <Transition name="modal">
      <div v-if="showClearConfirm" class="modal-overlay" @click.self="showClearConfirm = false">
        <div class="modal">
        <div class="modal-header">
          <div>
            <div class="eyebrow">— Danger Zone</div>
            <h3>Clear Database</h3>
          </div>
          <button class="modal-close" @click="showClearConfirm = false">&times;</button>
        </div>
        <div class="modal-body">
          <p class="modal-text">Are you sure you want to clear the entire database? This will delete all charge points, connectors, settings, and logs.</p>
          <p class="modal-text danger-text">This action cannot be undone.</p>
          <p v-if="clearError" class="error">{{ clearError }}</p>
          <div class="form-actions">
            <button class="btn btn-cancel" @click="showClearConfirm = false" :disabled="isClearing || clearSuccess">Cancel</button>
            <button class="btn btn-danger" @click="confirmClear" :disabled="isClearing || clearSuccess">
              {{ clearSuccess ? 'Cleared! Reloading...' : (isClearing ? 'Clearing...' : 'Yes, Clear Everything') }}
            </button>
          </div>
        </div>
      </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.settings-page {
  height: 100%;
  background: var(--bg-center);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: 40px 48px;
}

.settings-section {
  max-width: 720px;
}

.data-section {
  margin-top: 44px;
  padding-top: 32px;
  border-top: 1px solid var(--border-subtle);
}

.section-eyebrow {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  margin-bottom: 8px;
}

h2 {
  font-family: var(--font-mono);
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 24px 0;
  letter-spacing: 0.04em;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-field label {
  font-family: var(--font-mono);
  font-size: 0.74rem;
  letter-spacing: 0.02em;
  color: var(--text-muted);
}

.form-field input {
  padding: 11px 12px;
  background: var(--bg-sunken);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 0.82rem;
  letter-spacing: 0.02em;
}

.form-field input:focus {
  border-color: var(--accent);
}

.hint {
  margin-top: 10px;
  color: var(--text-muted);
  font-size: 0.76rem;
  letter-spacing: 0.02em;
}

.hint span {
  color: var(--text-secondary);
}

.actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 24px;
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
}

.btn:hover {
  color: var(--text-primary);
  border-color: rgba(244, 241, 234, 0.22);
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

.btn-danger {
  border-color: var(--danger);
  color: var(--danger);
}
.btn-danger:hover {
  background: rgba(214, 139, 110, 0.10);
}

.btn-cancel { color: var(--text-muted); }
.btn-cancel:hover {
  color: var(--text-secondary);
  border-color: var(--border-strong);
}

.file-input {
  display: none;
}

.saved {
  color: var(--status-online);
  font-family: var(--font-mono);
  font-size: 0.74rem;
}

.error {
  margin-top: 12px;
  color: var(--status-fault);
  font-family: var(--font-mono);
  font-size: 0.74rem;
}

/* Modal Styles */
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
  gap: 12px;
}

.modal-text {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--text-secondary);
  line-height: 1.5;
  margin: 0;
}

.danger-text {
  color: var(--status-fault);
  font-weight: 600;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}

@media (max-width: 768px) {
  .settings-page {
    padding: 24px 16px;
  }
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
