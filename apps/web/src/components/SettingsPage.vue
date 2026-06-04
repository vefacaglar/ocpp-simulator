<script setup lang="ts">
import { ref, watch } from 'vue'
import { useSettingsStore } from '../stores/settingsStore'

const settings = useSettingsStore()
const draftUrl = ref(settings.centralSystemUrl)
const saved = ref(false)

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

.saved {
  color: var(--status-online);
  font-family: var(--font-mono);
  font-size: 0.74rem;
}
</style>
