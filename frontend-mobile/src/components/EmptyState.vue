<template>
  <div class="empty-state">
    <div class="empty-state-icon">{{ icon }}</div>
    <div class="empty-state-title">{{ title }}</div>
    <div class="empty-state-desc">{{ description }}</div>
    <div v-if="primaryAction" class="empty-state-actions">
      <van-button type="primary" size="small" round @click="primaryAction.handler">
        {{ primaryAction.label }}
      </van-button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  icon: string
  title: string
  description: string
  primaryAction?: { label: string; handler: () => void }
}>()
</script>

<style scoped>
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  padding: 40px 24px; text-align: center;
}
.empty-state-icon {
  width: 64px; height: 64px; border-radius: 50%;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  display: flex; align-items: center; justify-content: center;
  font-size: 28px; margin-bottom: 12px;
}
.empty-state-title { font-size: 15px; font-weight: 600; color: #333; margin-bottom: 4px; }
.empty-state-desc { font-size: 13px; color: #888; max-width: 260px; line-height: 1.5; }
.empty-state-actions { margin-top: 16px; }
</style>
