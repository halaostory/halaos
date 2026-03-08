<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NSpace, NTag, NProgress, NSkeleton, NEmpty,
} from 'naive-ui'
import { flightRiskAPI } from '../api/client'

interface FlightRiskFactor {
  factor: string
  points: number
  detail: string
}

interface FlightRiskEmployee {
  employee_id: number
  employee_no: string
  first_name: string
  last_name: string
  department: string
  risk_score: number
  factors: FlightRiskFactor[]
  calculated_at: string
}

const router = useRouter()
const { t } = useI18n()

const loading = ref(true)
const employees = ref<FlightRiskEmployee[]>([])

function getRiskLevel(score: number): 'high' | 'medium' | 'low' {
  if (score >= 70) return 'high'
  if (score >= 50) return 'medium'
  return 'low'
}

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

function viewProfile(employeeId: number) {
  router.push(`/employees/${employeeId}`)
}

onMounted(async () => {
  try {
    const res = await flightRiskAPI.getTopRisks()
    const data = (res as any)?.data ?? res
    employees.value = Array.isArray(data) ? data : []
  } catch (e) {
    console.error('Failed to load flight risk data', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <NCard style="margin-bottom: 24px;">
    <template #header>
      <div style="display: flex; align-items: center; gap: 8px;">
        <span style="font-size: 18px;">&#9888;&#65039;</span>
        <span>{{ t('flightRisk.title') }}</span>
      </div>
    </template>
    <template #header-extra>
      <NButton text type="primary" size="small" @click="router.push('/analytics')">
        {{ t('flightRisk.viewAll') }}
      </NButton>
    </template>

    <!-- Loading skeleton -->
    <div v-if="loading" style="display: flex; flex-direction: column; gap: 16px;">
      <div v-for="i in 3" :key="i" style="display: flex; align-items: center; gap: 12px;">
        <NSkeleton :width="24" :height="24" circle />
        <div style="flex: 1;">
          <NSkeleton text :repeat="2" />
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <NEmpty
      v-else-if="employees.length === 0"
      :description="t('flightRisk.noAlerts')"
      style="padding: 24px 0;"
    />

    <!-- Employee list -->
    <div v-else style="display: flex; flex-direction: column; gap: 16px;">
      <div
        v-for="emp in employees"
        :key="emp.employee_id"
        style="padding: 12px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5);"
      >
        <!-- Top row: indicator, name, score, department -->
        <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 8px;">
          <span style="font-size: 16px;">
            {{ getRiskLevel(emp.risk_score) === 'high' ? '\uD83D\uDD34' : getRiskLevel(emp.risk_score) === 'medium' ? '\uD83D\uDFE1' : '\uD83D\uDFE2' }}
          </span>
          <div style="flex: 1; min-width: 0;">
            <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
              <span style="font-weight: 600; font-size: 14px;">
                {{ emp.first_name }} {{ emp.last_name }}
              </span>
              <NTag :type="getRiskTagType(emp.risk_score)" size="small" :bordered="false">
                {{ emp.risk_score }}/100
              </NTag>
              <NTag size="small" :bordered="false">
                {{ emp.department }}
              </NTag>
            </div>
          </div>
        </div>

        <!-- Progress bar -->
        <NProgress
          type="line"
          :percentage="emp.risk_score"
          :color="getRiskColor(emp.risk_score)"
          :rail-color="'rgba(0,0,0,0.08)'"
          :height="6"
          :show-indicator="false"
          style="margin-bottom: 8px;"
        />

        <!-- Factors -->
        <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px;">
          <NTag
            v-for="factor in emp.factors.slice(0, 3)"
            :key="factor.factor"
            size="small"
            :bordered="false"
            :type="factor.points >= 15 ? 'warning' : 'default'"
          >
            {{ factor.detail }}
          </NTag>
        </div>

        <!-- Actions -->
        <NSpace size="small">
          <NButton size="tiny" secondary @click="viewProfile(emp.employee_id)">
            {{ t('flightRisk.viewProfile') }}
          </NButton>
          <NButton
            v-if="getRiskLevel(emp.risk_score) !== 'low'"
            size="tiny"
            secondary
            type="primary"
            @click="viewProfile(emp.employee_id)"
          >
            {{ t('flightRisk.scheduleOneOnOne') }}
          </NButton>
        </NSpace>
      </div>
    </div>
  </NCard>
</template>
