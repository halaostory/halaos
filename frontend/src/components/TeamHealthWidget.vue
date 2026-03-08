<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NTag, NProgress, NSkeleton, NEmpty, NSpace,
} from 'naive-ui'
import { teamHealthAPI } from '../api/client'

interface HealthFactor {
  name: string
  score: number
  detail: string
}

interface DepartmentHealth {
  department_id: number
  department_name: string
  health_score: number
  factors: HealthFactor[]
  calculated_at: string
}

const { t } = useI18n()

const loading = ref(true)
const departments = ref<DepartmentHealth[]>([])

function getHealthColor(score: number): string {
  if (score >= 75) return '#18a058'
  if (score >= 50) return '#f0a020'
  return '#d03050'
}

function getHealthTagType(score: number): 'success' | 'warning' | 'error' {
  if (score >= 75) return 'success'
  if (score >= 50) return 'warning'
  return 'error'
}

function getHealthLabel(score: number): string {
  if (score >= 75) return t('teamHealth.healthy')
  if (score >= 50) return t('teamHealth.needsAttention')
  return t('teamHealth.critical')
}

function getFactorLabel(name: string): string {
  const key = `teamHealth.${name}` as const
  return t(key)
}

onMounted(async () => {
  try {
    const res = await teamHealthAPI.getScores()
    const data = (res as any)?.data ?? res
    departments.value = Array.isArray(data) ? data : []
  } catch (e) {
    console.error('Failed to load team health data', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <NCard style="margin-bottom: 24px;">
    <template #header>
      <span>{{ t('teamHealth.title') }}</span>
    </template>

    <!-- Loading skeleton -->
    <div v-if="loading" style="display: flex; flex-direction: column; gap: 16px;">
      <div v-for="i in 3" :key="i" style="display: flex; align-items: center; gap: 12px;">
        <NSkeleton :width="80" :height="16" />
        <div style="flex: 1;">
          <NSkeleton text />
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <NEmpty
      v-else-if="departments.length === 0"
      :description="t('teamHealth.noData')"
      style="padding: 24px 0;"
    />

    <!-- Department list -->
    <div v-else style="display: flex; flex-direction: column; gap: 14px;">
      <div
        v-for="dept in departments"
        :key="dept.department_id"
        style="padding: 10px 12px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5);"
      >
        <!-- Header: department name + score tag -->
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px;">
          <span style="font-weight: 600; font-size: 14px;">{{ dept.department_name }}</span>
          <NTag :type="getHealthTagType(dept.health_score)" size="small">
            {{ dept.health_score }}/100 - {{ getHealthLabel(dept.health_score) }}
          </NTag>
        </div>

        <!-- Progress bar -->
        <NProgress
          type="line"
          :percentage="dept.health_score"
          :color="getHealthColor(dept.health_score)"
          :rail-color="'rgba(0,0,0,0.08)'"
          :height="6"
          :show-indicator="false"
          style="margin-bottom: 8px;"
        />

        <!-- Factor breakdown -->
        <NSpace size="small" style="flex-wrap: wrap;">
          <NTag
            v-for="factor in dept.factors"
            :key="factor.name"
            size="small"
            :bordered="false"
            :type="factor.score >= 15 ? 'success' : factor.score >= 10 ? 'warning' : 'error'"
          >
            {{ getFactorLabel(factor.name) }}: {{ factor.score }}/20
          </NTag>
        </NSpace>
      </div>
    </div>
  </NCard>
</template>
