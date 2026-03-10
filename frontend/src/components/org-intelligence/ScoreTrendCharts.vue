<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import {
  TitleComponent, TooltipComponent, LegendComponent,
  GridComponent,
} from 'echarts/components'
import { useThemeStore } from '../../stores/theme'
import { useI18n } from 'vue-i18n'

use([CanvasRenderer, LineChart, TitleComponent, TooltipComponent, LegendComponent, GridComponent])

const props = defineProps<{
  trends: Array<{
    week_date: string
    avg_flight_risk: any
    avg_burnout: any
    avg_team_health: any
  }>
}>()

const themeStore = useThemeStore()
const { t } = useI18n()

const chartTextColor = computed(() => themeStore.isDark ? '#ccc' : '#333')

function numVal(v: any): number {
  if (typeof v === 'number') return v
  if (typeof v === 'string') return parseFloat(v) || 0
  if (v && typeof v === 'object' && 'Float64' in v) return v.Float64 || 0
  return 0
}

const chartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  legend: {
    data: [
      t('orgIntelligence.flightRiskAvg'),
      t('orgIntelligence.burnoutAvg'),
      t('orgIntelligence.teamHealthAvg'),
    ],
    textStyle: { color: chartTextColor.value },
  },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: {
    type: 'category',
    data: props.trends.map(t => t.week_date?.slice(5) || ''),
    axisLabel: { color: chartTextColor.value },
  },
  yAxis: {
    type: 'value',
    min: 0,
    max: 100,
    axisLabel: { color: chartTextColor.value },
  },
  series: [
    {
      name: t('orgIntelligence.flightRiskAvg'),
      type: 'line',
      data: props.trends.map(t => numVal(t.avg_flight_risk)),
      smooth: true,
      itemStyle: { color: '#d03050' },
    },
    {
      name: t('orgIntelligence.burnoutAvg'),
      type: 'line',
      data: props.trends.map(t => numVal(t.avg_burnout)),
      smooth: true,
      itemStyle: { color: '#f0a020' },
    },
    {
      name: t('orgIntelligence.teamHealthAvg'),
      type: 'line',
      data: props.trends.map(t => numVal(t.avg_team_health)),
      smooth: true,
      itemStyle: { color: '#18a058' },
    },
  ],
}))
</script>

<template>
  <VChart :option="chartOption" style="height: 350px;" autoresize />
</template>
