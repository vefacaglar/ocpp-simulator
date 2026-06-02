<script setup lang="ts">
</script>

<template>
  <div class="app-shell">
    <header class="top-bar">
      <h1>OCPP Simulator</h1>
    </header>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  font-family: system-ui, -apple-system, sans-serif;
}

.top-bar {
  display: flex;
  align-items: center;
  padding: 0 1rem;
  height: 48px;
  background: #1a1a2e;
  color: #e0e0e0;
  border-bottom: 1px solid #333;
}

.top-bar h1 {
  font-size: 1rem;
  font-weight: 600;
  margin: 0;
}

.content {
  flex: 1;
  overflow: hidden;
}
</style>
