<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NCard, NButton, NTag, NSkeleton, NEmpty,
} from 'naive-ui'
import { complianceAlertsAPI } from '../api/client'

interface ComplianceAlert {
  id: number
  alert_type: string
  severity: string
  title: string
  description: string
  entity_type: string | null
  entity_id: number
  due_date: string | null
  days_remaining: number
  calculated_at: string
}

const router = useRouter()
const { t } = useI18n()

const loading = ref(true)
const alerts = ref<ComplianceAlert[]>([])

function getSeverityType(severity: string): 'error' | 'warning' | 'info' | 'default' {
  switch (severity) {
    case 'critical': return 'error'
    case 'high': return 'warning'
    case 'medium': return 'info'
    default: return 'default'
  }
}

function getAlertIcon(type: string): string {
  switch (type) {
    case 'document_expiry': return '\uD83D\uDCC4'
    case 'contract_expiry': return '\uD83D\uDCDD'
    case 'filing_overdue': return '\u26A0\uFE0F'
    case 'filing_upcoming': return '\uD83D\uDCC5'
    default: return '\uD83D\uDD14'
  }
}

function getAlertRoute(alert: ComplianceAlert): string {
  switch (alert.alert_type) {
    case 'document_expiry': return `/employees/${alert.entity_id}`
    case 'contract_expiry': return `/employees/${alert.entity_id}`
    case 'filing_overdue':
    case 'filing_upcoming': return '/tax-filings'
    default: return '/compliance'
  }
}

onMounted(async () => {
  try {
    const res = await complianceAlertsAPI.getAlerts()
    const data = (res as any)?.data ?? res
    alerts.value = Array.isArray(data) ? data : []
  } catch (e) {
    console.error('Failed to load compliance alerts', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <NCard style="margin-bottom: 24px;">
    <template #header>
      <div style="display: flex; align-items: center; gap: 8px;">
        <span style="font-size: 18px;">&#9878;&#65039;</span>
        <span>{{ t('complianceAlerts.title') }}</span>
      </div>
    </template>

    <div v-if="loading" style="display: flex; flex-direction: column; gap: 16px;">
      <div v-for="i in 3" :key="i" style="display: flex; align-items: center; gap: 12px;">
        <NSkeleton :width="24" :height="24" circle />
        <div style="flex: 1;"><NSkeleton text :repeat="2" /></div>
      </div>
    </div>

    <NEmpty v-else-if="alerts.length === 0" :description="t('complianceAlerts.noAlerts')" style="padding: 24px 0;" />

    <div v-else style="display: flex; flex-direction: column; gap: 12px;">
      <div
        v-for="alert in alerts"
        :key="alert.id"
        style="padding: 12px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5); display: flex; align-items: flex-start; gap: 10px;"
      >
        <span style="font-size: 18px; flex-shrink: 0;">{{ getAlertIcon(alert.alert_type) }}</span>
        <div style="flex: 1; min-width: 0;">
          <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 4px;">
            <NTag :type="getSeverityType(alert.severity)" size="small" :bordered="false">
              {{ alert.severity.toUpperCase() }}
            </NTag>
            <span style="font-weight: 600; font-size: 13px;">{{ alert.title }}</span>
          </div>
          <div style="font-size: 12px; color: #888; margin-bottom: 6px;">{{ alert.description }}</div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <NTag v-if="alert.days_remaining <= 0" type="error" size="small">
              {{ Math.abs(alert.days_remaining) }} {{ t('complianceAlerts.daysOverdue') }}
            </NTag>
            <NTag v-else size="small" :type="alert.days_remaining <= 7 ? 'warning' : 'default'">
              {{ alert.days_remaining }} {{ t('complianceAlerts.daysLeft') }}
            </NTag>
            <NButton size="tiny" secondary @click="router.push(getAlertRoute(alert))">
              {{ t('complianceAlerts.view') }}
            </NButton>
          </div>
        </div>
      </div>
    </div>
  </NCard>
</template>
