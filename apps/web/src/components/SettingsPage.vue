<script setup lang="ts">
import { ref, watch } from 'vue'
import { useSettingsStore } from '../stores/settingsStore'
import { exportBrowserDb, importBrowserDb, type BrowserDbExport } from '../db/browserDb'

const settings = useSettingsStore()
const draftUrl = ref(settings.centralSystemUrl)
const saved = ref(false)
const importInput = ref<HTMLInputElement | null>(null)
const importError = ref<string | null>(null)

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
  link.download = `ocpp-simulator-${new Date().toISOString().slice(0, 10)}.json`
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
        <input ref="importInput" class="file-input" type="file" accept="application/json,.json" @change="importData" />
      </div>
      <p v-if="importError" class="error">{{ importError }}</p>
    </section>
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

.btn-primary {
  background: var(--accent);
  color: var(--accent-ink);
  border-color: var(--accent);
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

@media (max-width: 768px) {
  .settings-page {
    padding: 24px 16px;
  }
}
</style>
