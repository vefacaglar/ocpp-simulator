<script setup lang="ts">
import { onMounted, ref } from 'vue'
import ChargePointList from './components/ChargePointList.vue'
import ChargePointDetail from './components/ChargePointDetail.vue'
import LiveLogPanel from './components/LiveLogPanel.vue'
import SettingsPage from './components/SettingsPage.vue'
import { useSettingsStore } from './stores/settingsStore'

const settings = useSettingsStore()
const view = ref<'simulator' | 'settings'>('simulator')

onMounted(() => {
  void settings.loadSettings()
})
</script>

<template>
  <div class="app-shell">
    <header class="top-bar">
      <div class="brand">
        <span class="brand-mark">OCPP</span>
        <span class="brand-sep">/</span>
        <span class="brand-mark-serif">Simulator</span>
      </div>
      <nav class="top-nav">
        <button :class="{ active: view === 'simulator' }" @click="view = 'simulator'">Simulator</button>
        <button :class="{ active: view === 'settings' }" @click="view = 'settings'">Settings</button>
      </nav>
      <div class="meta">
        <span class="meta-item">v0.0.0</span>
        <span class="meta-dot">·</span>
        <span class="meta-item">browser-only</span>
        <span class="meta-dot">·</span>
        <span class="meta-item status-dot">●</span>
      </div>
    </header>
    <main v-if="view === 'simulator'" class="content">
      <ChargePointList class="panel-left" />
      <ChargePointDetail class="panel-center" />
      <LiveLogPanel class="panel-right" />
    </main>
    <main v-else class="settings-content">
      <SettingsPage />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--bg-base);
}

.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 48px;
  height: 72px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--border-subtle);
}

.brand {
  display: flex;
  align-items: baseline;
  gap: 10px;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  letter-spacing: 0.04em;
  color: var(--text-secondary);
}

.top-nav {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: 24px;
  margin-right: auto;
}

.top-nav button {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.02em;
  padding: 6px 10px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}

.top-nav button:hover,
.top-nav button.active {
  color: var(--accent);
  border-color: var(--border-strong);
  background: var(--accent-soft);
}

.brand-mark {
  font-weight: 600;
  color: var(--text-primary);
}

.brand-sep {
  color: var(--text-muted);
}

.brand-mark-serif {
  font-family: var(--font-mono);
  font-weight: 500;
  letter-spacing: 0.04em;
  font-size: 0.82rem;
  color: var(--text-primary);
}

.meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.meta-dot {
  color: var(--text-muted);
  opacity: 0.6;
}

.status-dot {
  color: var(--status-online);
  font-size: 0.5rem;
}

.content {
  flex: 1;
  display: grid;
  grid-template-columns: 280px 1fr 400px;
  gap: 0;
  overflow: hidden;
  padding: 0 48px 48px 48px;
}

.settings-content {
  flex: 1;
  overflow: hidden;
  padding: 0 48px 48px 48px;
}

.panel-left,
.panel-center,
.panel-right {
  height: 100%;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  overflow: hidden;
}

.panel-left {
  border-radius: var(--radius-md) 0 0 var(--radius-md);
  border-right: none;
}

.panel-center {
  background: var(--bg-center);
  border-color: var(--border-strong);
}

.panel-right {
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
  border-left: none;
}
</style>
