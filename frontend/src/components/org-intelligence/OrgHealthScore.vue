<script setup lang="ts">
import { computed } from 'vue'
import { NProgress, NStatistic } from 'naive-ui'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  score: number
}>()

const { t } = useI18n()

const color = computed(() => {
  if (props.score >= 75) return '#18a058'
  if (props.score >= 50) return '#f0a020'
  return '#d03050'
})

const label = computed(() => {
  if (props.score >= 75) return t('orgIntelligence.healthy')
  if (props.score >= 50) return t('orgIntelligence.needsAttention')
  return t('orgIntelligence.critical')
})
</script>

<template>
  <div style="text-align: center;">
    <NProgress
      type="circle"
      :percentage="Math.round(score)"
      :color="color"
      :stroke-width="8"
      :indicator-text-color="color"
    >
      {{ Math.round(score) }}
    </NProgress>
    <div style="margin-top: 8px; font-weight: 600; font-size: 14px;" :style="{ color }">
      {{ label }}
    </div>
    <NStatistic :label="t('orgIntelligence.orgHealthScore')" :value="Math.round(score)" style="margin-top: 4px;" />
  </div>
</template>
