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
.badge { font-size: 0.65rem; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; font-weight: 600; }
.badge.disconnected { background: #3a2020; color: #e06060; }
.badge.connecting { background: #3a3a20; color: #e0c060; }
.badge.connected { background: #203a20; color: #60e060; }
.badge.available { background: #203a20; color: #60e060; }
.badge.preparing { background: #3a3a20; color: #e0c060; }
.badge.charging { background: #203050; color: #60a0e0; }
.badge.finishing { background: #2a2a40; color: #a0a0c0; }
.badge.faulted { background: #3a2020; color: #e06060; }
.badge.unavailable { background: #2a2a2a; color: #888; }
</style>
