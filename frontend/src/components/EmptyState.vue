<template>
  <div class="empty-state">
    <div class="empty-state-icon">{{ icon }}</div>
    <div class="empty-state-title">{{ title }}</div>
    <div class="empty-state-desc">{{ description }}</div>
    <NSpace v-if="primaryAction || secondaryAction" style="margin-top: 16px;">
      <NButton v-if="primaryAction" type="primary" @click="primaryAction.handler">
        {{ primaryAction.label }}
      </NButton>
      <NButton v-if="secondaryAction" @click="secondaryAction.handler">
        {{ secondaryAction.label }}
      </NButton>
    </NSpace>
  </div>
</template>

<script setup lang="ts">
import { NButton, NSpace } from 'naive-ui'

defineProps<{
  icon: string
  title: string
  description: string
  primaryAction?: { label: string; handler: () => void }
  secondaryAction?: { label: string; handler: () => void }
}>()
</script>

<style scoped>
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; padding: 48px 24px; text-align: center;
}
.empty-state-icon {
  width: 80px; height: 80px; border-radius: 50%;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  display: flex; align-items: center; justify-content: center;
  font-size: 36px; margin-bottom: 16px;
}
.empty-state-title {
  font-size: 16px; font-weight: 600; color: #333; margin-bottom: 6px;
}
.empty-state-desc {
  font-size: 13px; color: #888; max-width: 320px; line-height: 1.5;
}
</style>
