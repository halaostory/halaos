<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NTag, NProgress, NButton, NEmpty,
} from 'naive-ui'

interface ConcernEmployee {
  employee_id: number
  name: string
  employee_no: string
  department: string
  risk_score?: number
  burnout_score?: number
}

defineProps<{
  flightRisk: ConcernEmployee[]
  burnout: ConcernEmployee[]
}>()

const router = useRouter()
const { t } = useI18n()

function scoreColor(score: number): string {
  if (score >= 70) return '#d03050'
  if (score >= 50) return '#f0a020'
  return '#18a058'
}

function scoreTagType(score: number): 'error' | 'warning' | 'success' {
  if (score >= 70) return 'error'
  if (score >= 50) return 'warning'
  return 'success'
}
</script>

<template>
  <div>
    <!-- Flight Risk Section -->
    <h4 style="margin: 0 0 12px 0;">{{ t('flightRisk.title') }}</h4>
    <NEmpty v-if="flightRisk.length === 0" :description="t('flightRisk.noAlerts')" style="padding: 12px 0;" />
    <div v-else style="display: flex; flex-direction: column; gap: 10px; margin-bottom: 20px;">
      <div
        v-for="emp in flightRisk.slice(0, 5)"
        :key="'fr-' + emp.employee_id"
        style="display: flex; align-items: center; gap: 10px; padding: 8px 12px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5);"
      >
        <div style="flex: 1; min-width: 0;">
          <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
            <span style="font-weight: 600; font-size: 13px;">{{ emp.name }}</span>
            <NTag :type="scoreTagType(emp.risk_score ?? 0)" size="small" :bordered="false">
              {{ emp.risk_score }}/100
            </NTag>
            <NTag size="small" :bordered="false">{{ emp.department }}</NTag>
          </div>
          <NProgress
            type="line"
            :percentage="emp.risk_score ?? 0"
            :color="scoreColor(emp.risk_score ?? 0)"
            :height="4"
            :show-indicator="false"
            style="margin-top: 6px;"
          />
        </div>
        <NButton size="tiny" secondary @click="router.push(`/employees/${emp.employee_id}`)">
          {{ t('flightRisk.viewProfile') }}
        </NButton>
      </div>
    </div>

    <!-- Burnout Section -->
    <h4 style="margin: 0 0 12px 0;">{{ t('burnoutRisk.title') }}</h4>
    <NEmpty v-if="burnout.length === 0" :description="t('burnoutRisk.noAlerts')" style="padding: 12px 0;" />
    <div v-else style="display: flex; flex-direction: column; gap: 10px;">
      <div
        v-for="emp in burnout.slice(0, 5)"
        :key="'bo-' + emp.employee_id"
        style="display: flex; align-items: center; gap: 10px; padding: 8px 12px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5);"
      >
        <div style="flex: 1; min-width: 0;">
          <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
            <span style="font-weight: 600; font-size: 13px;">{{ emp.name }}</span>
            <NTag :type="scoreTagType(emp.burnout_score ?? 0)" size="small" :bordered="false">
              {{ emp.burnout_score }}/100
            </NTag>
            <NTag size="small" :bordered="false">{{ emp.department }}</NTag>
          </div>
          <NProgress
            type="line"
            :percentage="emp.burnout_score ?? 0"
            :color="scoreColor(emp.burnout_score ?? 0)"
            :height="4"
            :show-indicator="false"
            style="margin-top: 6px;"
          />
        </div>
        <NButton size="tiny" secondary @click="router.push(`/employees/${emp.employee_id}`)">
          {{ t('burnoutRisk.viewProfile') }}
        </NButton>
      </div>
    </div>
  </div>
</template>
