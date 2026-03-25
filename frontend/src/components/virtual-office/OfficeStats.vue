<template>
  <div style="display: flex; gap: 16px; padding: 8px 0">
    <n-statistic :label="t('virtualOffice.online')" :value="stats.online ?? 0">
      <template #prefix><span style="color: #18a058">●</span></template>
    </n-statistic>
    <n-statistic :label="t('virtualOffice.inMeeting')" :value="stats.in_meeting ?? 0">
      <template #prefix><span style="color: #2080f0">●</span></template>
    </n-statistic>
    <n-statistic :label="t('virtualOffice.onLeave')" :value="stats.on_leave ?? 0">
      <template #prefix><span style="color: #d03050">●</span></template>
    </n-statistic>
    <n-statistic :label="t('virtualOffice.offline')" :value="stats.offline ?? 0">
      <template #prefix><span style="color: #999">●</span></template>
    </n-statistic>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ stats: Record<string, number> }>()
const { t } = useI18n()
</script>
