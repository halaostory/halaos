<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NSpace, NTag, NProgress, NSkeleton, NEmpty,
} from 'naive-ui'
import { burnoutRiskAPI } from '../api/client'

interface BurnoutFactor {
  factor: string
  points: number
  detail: string
}

interface BurnoutEmployee {
  employee_id: number
  employee_no: string
  name: string
  department: string
  burnout_score: number
  factors: BurnoutFactor[]
  calculated_at: string
}

const router = useRouter()
const { t } = useI18n()

const loading = ref(true)
const employees = ref<BurnoutEmployee[]>([])

function getRiskColor(score: number): string {
  if (score >= 70) return '#d03050'
  if (score >= 50) return '#f0a020'
  return '#18a058'
}

function getRiskTagType(score: number): 'error' | 'warning' | 'success' {
  if (score >= 70) return 'error'
  if (score >= 50) return 'warning'
  return 'success'
}

function getIndicator(score: number): string {
  if (score >= 70) return '\uD83D\uDD34'
  if (score >= 50) return '\uD83D\uDFE1'
  return '\uD83D\uDFE2'
}

function factorLabel(factor: string): string {
  const labels: Record<string, string> = {
    overtime_frequency: t('burnoutRisk.factorOT'),
    leave_avoidance: t('burnoutRisk.factorLeave'),
    long_hours: t('burnoutRisk.factorHours'),
    weekend_work: t('burnoutRisk.factorWeekend'),
    attendance_irregularity: t('burnoutRisk.factorIrregularity'),
  }
  return labels[factor] || factor
}

onMounted(async () => {
  try {
    const res = await burnoutRiskAPI.getTopRisks()
    const data = (res as any)?.data ?? res
    employees.value = Array.isArray(data) ? data : []
  } catch (e) {
    console.error('Failed to load burnout risk data', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <NCard style="margin-bottom: 24px;">
    <template #header>
      <div style="display: flex; align-items: center; gap: 8px;">
        <span style="font-size: 18px;">&#128293;</span>
        <span>{{ t('burnoutRisk.title') }}</span>
      </div>
    </template>
    <template #header-extra>
      <NButton text type="primary" size="small" @click="router.push('/analytics')">
        {{ t('burnoutRisk.viewAll') }}
      </NButton>
    </template>

    <div v-if="loading" style="display: flex; flex-direction: column; gap: 16px;">
      <div v-for="i in 3" :key="i" style="display: flex; align-items: center; gap: 12px;">
        <NSkeleton :width="24" :height="24" circle />
        <div style="flex: 1;"><NSkeleton text :repeat="2" /></div>
      </div>
    </div>

    <NEmpty v-else-if="employees.length === 0" :description="t('burnoutRisk.noAlerts')" style="padding: 24px 0;" />

    <div v-else style="display: flex; flex-direction: column; gap: 16px;">
      <div
        v-for="emp in employees"
        :key="emp.employee_id"
        style="padding: 12px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5);"
      >
        <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 8px;">
          <span style="font-size: 16px;">{{ getIndicator(emp.burnout_score) }}</span>
          <div style="flex: 1; min-width: 0;">
            <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
              <span style="font-weight: 600; font-size: 14px;">{{ emp.name }}</span>
              <NTag :type="getRiskTagType(emp.burnout_score)" size="small" :bordered="false">
                {{ emp.burnout_score }}/100
              </NTag>
              <NTag size="small" :bordered="false">{{ emp.department }}</NTag>
            </div>
          </div>
        </div>

        <NProgress
          type="line"
          :percentage="emp.burnout_score"
          :color="getRiskColor(emp.burnout_score)"
          :rail-color="'rgba(0,0,0,0.08)'"
          :height="6"
          :show-indicator="false"
          style="margin-bottom: 8px;"
        />

        <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px;">
          <NTag
            v-for="factor in emp.factors.slice(0, 3)"
            :key="factor.factor"
            size="small"
            :bordered="false"
            :type="factor.points >= 15 ? 'warning' : 'default'"
          >
            {{ factorLabel(factor.factor) }}: {{ factor.detail }}
          </NTag>
        </div>

        <NSpace size="small">
          <NButton size="tiny" secondary @click="router.push(`/employees/${emp.employee_id}`)">
            {{ t('burnoutRisk.viewProfile') }}
          </NButton>
        </NSpace>
      </div>
    </div>
  </NCard>
</template>
