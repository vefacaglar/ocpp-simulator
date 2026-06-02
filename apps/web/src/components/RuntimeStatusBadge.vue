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
.badge.disconnected { background: #fde8e8; color: #c53030; }
.badge.connecting { background: #fef9c3; color: #a16207; }
.badge.connected { background: #dcfce7; color: #16a34a; }
.badge.available { background: #dcfce7; color: #16a34a; }
.badge.preparing { background: #fef9c3; color: #a16207; }
.badge.charging { background: #dbeafe; color: #2563eb; }
.badge.finishing { background: #e8e8f0; color: #6b7280; }
.badge.faulted { background: #fde8e8; color: #c53030; }
.badge.unavailable { background: #f3f4f6; color: #9ca3af; }
</style>
