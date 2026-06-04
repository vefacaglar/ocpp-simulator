<script setup lang="ts">
defineProps<{
  status: string
}>()

function badgeClass(status: string) {
  return status.toLowerCase()
}
</script>

<template>
  <span class="badge" :class="badgeClass(status)">{{ status }}</span>
</template>

<style scoped>
.badge {
  font-family: var(--font-mono);
  font-size: 0.68rem;
  font-weight: 500;
  letter-spacing: 0.02em;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-strong);
  background: transparent;
  color: var(--text-secondary);
  white-space: nowrap;
}

.badge.disconnected { color: var(--status-fault); border-color: rgba(214, 139, 110, 0.35); }
.badge.connecting   { color: var(--status-busy);  border-color: rgba(212, 165, 116, 0.35); }
.badge.connected,
.badge.available    { color: var(--status-online); border-color: rgba(184, 196, 160, 0.35); }
.badge.preparing    { color: var(--status-busy);  border-color: rgba(212, 165, 116, 0.35); }
.badge.charging     { color: var(--accent);       border-color: rgba(232, 223, 200, 0.40); }
.badge.finishing    { color: var(--text-muted);   border-color: var(--border-subtle); }
.badge.faulted      { color: var(--status-fault); border-color: rgba(214, 139, 110, 0.40); background: rgba(214, 139, 110, 0.08); }
.badge.unavailable  { color: var(--text-muted);   border-color: var(--border-subtle); }
</style>
