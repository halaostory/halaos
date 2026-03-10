<script setup lang="ts">
import { computed } from 'vue'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart } from 'echarts/charts'
import {
  TitleComponent, TooltipComponent, LegendComponent, GridComponent,
} from 'echarts/components'
import { useThemeStore } from '../../stores/theme'

use([CanvasRenderer, BarChart, TitleComponent, TooltipComponent, LegendComponent, GridComponent])

const props = defineProps<{
  tiers: Array<{ tier: string; count: number }>
}>()

const themeStore = useThemeStore()
const chartTextColor = computed(() => themeStore.isDark ? '#ccc' : '#333')

const tierColors: Record<string, string> = {
  critical: '#d03050',
  high: '#f0a020',
  medium: '#2080f0',
  low: '#18a058',
}

const chartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: {
    type: 'category',
    data: props.tiers.map(t => t.tier),
    axisLabel: { color: chartTextColor.value },
  },
  yAxis: {
    type: 'value',
    axisLabel: { color: chartTextColor.value },
  },
  series: [{
    type: 'bar',
    data: props.tiers.map(tier => ({
      value: tier.count,
      itemStyle: { color: tierColors[tier.tier] || '#999' },
    })),
    barWidth: '50%',
  }],
}))
</script>

<template>
  <VChart :option="chartOption" style="height: 300px;" autoresize />
</template>
