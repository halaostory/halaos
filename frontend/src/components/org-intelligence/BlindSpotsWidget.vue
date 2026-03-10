<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import {
  NTag, NEmpty, NCollapse, NCollapseItem,
} from 'naive-ui'

interface AffectedEmployee {
  id: number
  name: string
  detail: string
}

interface BlindSpot {
  id: number
  manager_id: number
  spot_type: string
  severity: string
  title: string
  description: string
  employees: AffectedEmployee[] | string
  is_resolved: boolean
  week_date: string
  created_at: string
}

defineProps<{
  spots: BlindSpot[]
}>()

const { t } = useI18n()

function severityType(s: string): 'error' | 'warning' | 'info' {
  if (s === 'high') return 'error'
  if (s === 'medium') return 'warning'
  return 'info'
}

function spotIcon(type: string): string {
  const icons: Record<string, string> = {
    chronic_tardiness: '⏰',
    ot_concentration: '⚡',
    leave_never_taken: '🏖️',
    high_flight_risk: '🚨',
    feedback_gap: '💬',
  }
  return icons[type] || '👁️'
}

function parseEmployees(emps: AffectedEmployee[] | string): AffectedEmployee[] {
  if (typeof emps === 'string') {
    try { return JSON.parse(emps) } catch { return [] }
  }
  return emps || []
}
</script>

<template>
  <div>
    <NEmpty v-if="spots.length === 0" :description="t('blindSpots.noSpots')" style="padding: 16px 0;" />
    <NCollapse v-else>
      <NCollapseItem
        v-for="spot in spots"
        :key="spot.id"
        :name="String(spot.id)"
      >
        <template #header>
          <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
            <span>{{ spotIcon(spot.spot_type) }}</span>
            <span style="font-weight: 600; font-size: 13px;">{{ spot.title }}</span>
            <NTag :type="severityType(spot.severity)" size="small" :bordered="false">
              {{ spot.severity }}
            </NTag>
            <NTag v-if="spot.is_resolved" type="success" size="small" :bordered="false">
              {{ t('blindSpots.resolved') }}
            </NTag>
          </div>
        </template>
        <div style="padding: 0 0 8px 0;">
          <p style="margin: 0 0 10px 0; color: var(--n-text-color-3, #999); font-size: 13px;">
            {{ spot.description }}
          </p>
          <div
            v-for="emp in parseEmployees(spot.employees)"
            :key="emp.id"
            style="display: flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: 6px; background: var(--n-color-hover, #f5f5f5); margin-bottom: 6px; font-size: 13px;"
          >
            <span style="font-weight: 500;">{{ emp.name }}</span>
            <span style="color: var(--n-text-color-3, #999);">— {{ emp.detail }}</span>
          </div>
          <div style="margin-top: 8px; font-size: 12px; color: var(--n-text-color-3, #999);">
            {{ t('blindSpots.detectedOn') }} {{ spot.week_date }}
          </div>
        </div>
      </NCollapseItem>
    </NCollapse>
  </div>
</template>
