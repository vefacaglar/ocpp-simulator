<script setup lang="ts">
import ChargePointList from './components/ChargePointList.vue'
import ChargePointDetail from './components/ChargePointDetail.vue'
import LiveLogPanel from './components/LiveLogPanel.vue'
</script>

<template>
  <div class="app-shell">
    <header class="top-bar">
      <h1>OCPP Simulator</h1>
    </header>
    <main class="content">
      <ChargePointList class="panel-left" />
      <ChargePointDetail class="panel-center" />
      <LiveLogPanel class="panel-right" />
    </main>
  </div>
</template>

<style scoped>
.app-shell { display: flex; flex-direction: column; height: 100vh; font-family: system-ui, -apple-system, sans-serif; }
.top-bar { display: flex; align-items: center; justify-content: space-between; padding: 0 1rem; height: 48px; background: #fff; color: #1a1a2e; border-bottom: 1px solid #ddd; }
.top-bar h1 { font-size: 1rem; font-weight: 600; margin: 0; }
.content { flex: 1; display: grid; grid-template-columns: 260px 1fr 380px; overflow: hidden; }
</style>
