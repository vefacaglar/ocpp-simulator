<script setup lang="ts">
import ChargePointList from './components/ChargePointList.vue'
import ChargePointDetail from './components/ChargePointDetail.vue'
import LiveLogPanel from './components/LiveLogPanel.vue'
</script>

<template>
  <div class="app-shell">
    <header class="top-bar">
      <div class="brand">
        <span class="brand-mark">OCPP</span>
        <span class="brand-sep">/</span>
        <span class="brand-mark-serif">Simulator</span>
      </div>
      <div class="meta">
        <span class="meta-item">v0.0.0</span>
        <span class="meta-dot">·</span>
        <span class="meta-item">localhost:7080</span>
        <span class="meta-dot">·</span>
        <span class="meta-item status-dot">●</span>
      </div>
    </header>
    <main class="content">
      <ChargePointList class="panel-left" />
      <ChargePointDetail class="panel-center" />
      <LiveLogPanel class="panel-right" />
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

.panel-right {
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
  border-left: none;
}
</style>
