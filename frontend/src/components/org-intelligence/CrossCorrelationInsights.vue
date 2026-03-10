<script setup lang="ts">
import { computed } from 'vue'
import { NAlert } from 'naive-ui'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  highBurnoutAndHighRisk: number
  deptRiskAverages: Array<{ department_name: string; avg_risk_score: any }>
  deptBurnoutAverages: Array<{ department_name: string; avg_burnout_score: any }>
}>()

const { t } = useI18n()

function numVal(v: any): number {
  if (typeof v === 'number') return v
  if (typeof v === 'string') return parseFloat(v) || 0
  return 0
}

interface Insight {
  type: 'error' | 'warning' | 'info' | 'success'
  title: string
  message: string
}

const insights = computed<Insight[]>(() => {
  const result: Insight[] = []

  if (props.highBurnoutAndHighRisk > 0) {
    result.push({
      type: props.highBurnoutAndHighRisk >= 5 ? 'error' : 'warning',
      title: t('orgIntelligence.correlationBurnoutRisk'),
      message: t('orgIntelligence.correlationBurnoutRiskMsg', { count: props.highBurnoutAndHighRisk }),
    })
  }

  // Highlight departments with high risk
  for (const dept of props.deptRiskAverages) {
    const score = numVal(dept.avg_risk_score)
    if (score >= 50) {
      result.push({
        type: score >= 70 ? 'error' : 'warning',
        title: t('orgIntelligence.deptHighRisk'),
        message: t('orgIntelligence.deptHighRiskMsg', { dept: dept.department_name, score: score.toFixed(1) }),
      })
    }
  }

  // Highlight departments with high burnout
  for (const dept of props.deptBurnoutAverages) {
    const score = numVal(dept.avg_burnout_score)
    if (score >= 50) {
      result.push({
        type: score >= 70 ? 'error' : 'warning',
        title: t('orgIntelligence.deptHighBurnout'),
        message: t('orgIntelligence.deptHighBurnoutMsg', { dept: dept.department_name, score: score.toFixed(1) }),
      })
    }
  }

  if (result.length === 0) {
    result.push({
      type: 'success',
      title: t('orgIntelligence.noCorrelations'),
      message: t('orgIntelligence.noCorrelationsMsg'),
    })
  }

  return result
})
</script>

<template>
  <div style="display: flex; flex-direction: column; gap: 12px;">
    <NAlert
      v-for="(insight, idx) in insights"
      :key="idx"
      :type="insight.type"
      :title="insight.title"
    >
      {{ insight.message }}
    </NAlert>
  </div>
</template>
