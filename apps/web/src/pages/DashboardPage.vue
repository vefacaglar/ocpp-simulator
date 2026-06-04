<script setup lang="ts">
</script>

<template>
  <div class="dashboard">
    <aside class="panel-left">
      <div class="panel-header">
        <h2>Charge Points</h2>
      </div>
      <div class="panel-body">
        <p class="empty-state">No charge points yet.</p>
      </div>
    </aside>

    <section class="panel-center">
      <div class="panel-header">
        <h2>Control</h2>
      </div>
      <div class="panel-body">
        <p class="empty-state">Select a charge point to begin.</p>
      </div>
    </section>

    <aside class="panel-right">
      <div class="panel-header">
        <h2>Live Logs</h2>
      </div>
      <div class="panel-body">
        <p class="empty-state">No events.</p>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.dashboard {
  display: grid;
  grid-template-columns: 260px 1fr 340px;
  height: 100%;
  background: #0f0f1a;
  color: #e0e0e0;
}

.panel-left,
.panel-center,
.panel-right {
  display: flex;
  flex-direction: column;
  border-right: 1px solid #2a2a3a;
  overflow: hidden;
}

.panel-right {
  border-right: none;
}

.panel-header {
  display: flex;
  align-items: center;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #2a2a3a;
  background: #16162a;
}

.panel-header h2 {
  font-size: 0.85rem;
  font-weight: 500;
  margin: 0;
  letter-spacing: 0.02em;
  color: #8888aa;
}

.panel-body {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
}

.empty-state {
  color: #555;
  font-size: 0.85rem;
  text-align: center;
  margin-top: 2rem;
}
</style>
