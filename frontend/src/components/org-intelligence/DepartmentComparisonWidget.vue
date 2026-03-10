<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { RadarChart } from 'echarts/charts'
import {
  TitleComponent, TooltipComponent, LegendComponent, RadarComponent,
} from 'echarts/components'
import { useThemeStore } from '../../stores/theme'
import { useI18n } from 'vue-i18n'

use([CanvasRenderer, RadarChart, TitleComponent, TooltipComponent, LegendComponent, RadarComponent])

const props = defineProps<{
  departments: Array<{
    department_name: string
    health_score: number
  }>
  deptRisks?: Array<{
    department_name: string
    avg_risk_score: any
  }>
  deptBurnout?: Array<{
    department_name: string
    avg_burnout_score: any
  }>
}>()

const themeStore = useThemeStore()
const { t } = useI18n()
const chartTextColor = computed(() => themeStore.isDark ? '#ccc' : '#333')

function numVal(v: any): number {
  if (typeof v === 'number') return v
  if (typeof v === 'string') return parseFloat(v) || 0
  return 0
}

const chartOption = computed(() => {
  const depts = props.departments.slice(0, 6)
  const names = depts.map(d => d.department_name)

  const riskMap = new Map((props.deptRisks || []).map(d => [d.department_name, numVal(d.avg_risk_score)]))
  const burnoutMap = new Map((props.deptBurnout || []).map(d => [d.department_name, numVal(d.avg_burnout_score)]))

  return {
    tooltip: {},
    legend: {
      data: [
        t('orgIntelligence.teamHealthAvg'),
        t('orgIntelligence.flightRiskAvg'),
        t('orgIntelligence.burnoutAvg'),
      ],
      textStyle: { color: chartTextColor.value },
    },
    radar: {
      indicator: names.map(n => ({ name: n, max: 100 })),
      axisName: { color: chartTextColor.value },
    },
    series: [{
      type: 'radar',
      data: [
        {
          value: depts.map(d => d.health_score),
          name: t('orgIntelligence.teamHealthAvg'),
          areaStyle: { opacity: 0.1 },
          lineStyle: { color: '#18a058' },
          itemStyle: { color: '#18a058' },
        },
        {
          value: names.map(n => riskMap.get(n) || 0),
          name: t('orgIntelligence.flightRiskAvg'),
          areaStyle: { opacity: 0.1 },
          lineStyle: { color: '#d03050' },
          itemStyle: { color: '#d03050' },
        },
        {
          value: names.map(n => burnoutMap.get(n) || 0),
          name: t('orgIntelligence.burnoutAvg'),
          areaStyle: { opacity: 0.1 },
          lineStyle: { color: '#f0a020' },
          itemStyle: { color: '#f0a020' },
        },
      ],
    }],
  }
})
</script>

<template>
  <VChart :option="chartOption" style="height: 350px;" autoresize />
</template>
