<template>
  <div class="office-stats">
    <div class="stat-item" v-for="s in items" :key="s.key">
      <span class="stat-dot" :style="{ backgroundColor: s.color }"></span>
      <span class="stat-value">{{ stats[s.key] ?? 0 }}</span>
      <span class="stat-label">{{ t(s.label) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ stats: Record<string, number> }>()
const { t } = useI18n()

const items = [
  { key: 'online', color: '#18a058', label: 'virtualOffice.online' },
  { key: 'in_meeting', color: '#9C27B0', label: 'virtualOffice.inMeeting' },
  { key: 'on_leave', color: '#d03050', label: 'virtualOffice.onLeave' },
  { key: 'offline', color: '#bbb', label: 'virtualOffice.offline' },
]
</script>

<style scoped>
.office-stats {
  display: flex;
  gap: 24px;
  padding: 10px 16px;
  background: #fff;
  border-radius: 10px;
  border: 1px solid #f0f0f0;
  box-shadow: 0 1px 3px rgba(0,0,0,0.04);
}
.stat-item {
  display: flex;
  align-items: center;
  gap: 6px;
}
.stat-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.stat-value {
  font-size: 20px;
  font-weight: 700;
  color: #333;
  line-height: 1;
}
.stat-label {
  font-size: 13px;
  color: #888;
}
</style>
