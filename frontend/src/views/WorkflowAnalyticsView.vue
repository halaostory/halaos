<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NGrid, NGi, NCard, NStatistic, NSpin } from 'naive-ui'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, PieChart, LineChart } from 'echarts/charts'
import {
  TitleComponent, TooltipComponent, LegendComponent,
  GridComponent,
} from 'echarts/components'
import { workflowAPI } from '../api/client'

use([
  CanvasRenderer, BarChart, PieChart, LineChart,
  TitleComponent, TooltipComponent, LegendComponent,
  GridComponent,
])

const { t } = useI18n()
const loading = ref(true)
const analytics = ref<Record<string, any>>({})

async function fetchAnalytics() {
  loading.value = true
  try {
    const res = await workflowAPI.getAnalytics() as any
    analytics.value = res?.data ?? res ?? {}
  } catch { /* ignore */ }
  finally { loading.value = false }
}

const summary = computed(() => analytics.value?.summary ?? {})
const autoApproval = computed(() => analytics.value?.auto_approval ?? {})
const sla = computed(() => analytics.value?.sla_compliance ?? {})

// KPI: SLA compliance percentage
const slaCompliancePct = computed(() => {
  const s = sla.value
  if (!s.total || s.total === 0) return '-'
  const withinSla = Number(s.within_sla || 0)
  return Math.round((withinSla / s.total) * 100) + '%'
})

// KPI: Auto-approval rate
const autoApprovalPct = computed(() => {
  const a = autoApproval.value
  const autoCount = Number(a.auto_approved || 0) + Number(a.auto_rejected || 0)
  const totalApprovals = Number(summary.value.approved || 0) + Number(summary.value.rejected || 0) + autoCount
  if (totalApprovals === 0) return '-'
  return Math.round((autoCount / totalApprovals) * 100) + '%'
})

// Volume line chart
const volumeChartOption = computed(() => {
  const volume = analytics.value?.volume_by_day ?? []
  return {
    tooltip: { trigger: 'axis' },
    legend: { data: ['Approved', 'Rejected', 'Pending'] },
    grid: { left: 40, right: 20, bottom: 30, top: 40 },
    xAxis: { type: 'category', data: volume.map((v: any) => v.day) },
    yAxis: { type: 'value' },
    series: [
      { name: 'Approved', type: 'line', data: volume.map((v: any) => v.approved), smooth: true, color: '#18a058' },
      { name: 'Rejected', type: 'line', data: volume.map((v: any) => v.rejected), smooth: true, color: '#d03050' },
      { name: 'Pending', type: 'line', data: volume.map((v: any) => v.pending), smooth: true, color: '#f0a020' },
    ],
  }
})

// Approval type pie chart
const typePieOption = computed(() => {
  const s = summary.value
  return {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0 },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      data: [
        { value: Number(s.approved || 0), name: 'Approved', itemStyle: { color: '#18a058' } },
        { value: Number(s.rejected || 0), name: 'Rejected', itemStyle: { color: '#d03050' } },
        { value: Number(s.pending || 0), name: 'Pending', itemStyle: { color: '#f0a020' } },
      ],
    }],
  }
})

// Pending age bar chart
const pendingAgeOption = computed(() => {
  const age = analytics.value?.pending_age ?? []
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: 40, right: 20, bottom: 30, top: 10 },
    xAxis: { type: 'category', data: age.map((a: any) => a.bucket) },
    yAxis: { type: 'value' },
    series: [{
      type: 'bar',
      data: age.map((a: any) => a.count),
      color: '#2080f0',
    }],
  }
})

// Avg approval time bar chart
const avgTimeOption = computed(() => {
  const times = analytics.value?.avg_approval_time ?? []
  return {
    tooltip: { trigger: 'axis' },
    grid: { left: 120, right: 20, bottom: 30, top: 10 },
    yAxis: { type: 'category', data: times.map((t: any) => t.entity_type) },
    xAxis: { type: 'value', name: 'Hours' },
    series: [{
      type: 'bar',
      data: times.map((t: any) => parseFloat(t.avg_hours)),
      color: '#2080f0',
    }],
  }
})

onMounted(fetchAnalytics)
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px">{{ t('workflowAnalytics.title') }}</h2>

    <NSpin v-if="loading" style="display: flex; justify-content: center; padding: 48px;" />

    <template v-else>
      <!-- KPI Cards -->
      <NGrid :cols="4" :x-gap="12" :y-gap="12" style="margin-bottom: 16px" responsive="screen" :item-responsive="true">
        <NGi :span="24" style="min-width: 0;">
          <NGrid :cols="4" :x-gap="12" responsive="screen">
            <NGi>
              <NCard>
                <NStatistic :label="t('workflowAnalytics.totalApprovals')" :value="summary.total || 0" />
              </NCard>
            </NGi>
            <NGi>
              <NCard>
                <NStatistic :label="t('workflowAnalytics.pendingCount')" :value="summary.pending || 0" />
              </NCard>
            </NGi>
            <NGi>
              <NCard>
                <NStatistic :label="t('workflowAnalytics.slaCompliance')" :value="slaCompliancePct" />
              </NCard>
            </NGi>
            <NGi>
              <NCard>
                <NStatistic :label="t('workflowAnalytics.autoApprovalRate')" :value="autoApprovalPct" />
              </NCard>
            </NGi>
          </NGrid>
        </NGi>
      </NGrid>

      <!-- Charts Row 1 -->
      <NGrid :cols="2" :x-gap="12" :y-gap="12" style="margin-bottom: 16px" responsive="screen">
        <NGi>
          <NCard :title="t('workflowAnalytics.volumeTrend')">
            <VChart :option="volumeChartOption" style="height: 300px" autoresize />
          </NCard>
        </NGi>
        <NGi>
          <NCard :title="t('workflowAnalytics.approvalBreakdown')">
            <VChart :option="typePieOption" style="height: 300px" autoresize />
          </NCard>
        </NGi>
      </NGrid>

      <!-- Charts Row 2 -->
      <NGrid :cols="2" :x-gap="12" :y-gap="12" responsive="screen">
        <NGi>
          <NCard :title="t('workflowAnalytics.pendingAge')">
            <VChart :option="pendingAgeOption" style="height: 250px" autoresize />
          </NCard>
        </NGi>
        <NGi>
          <NCard :title="t('workflowAnalytics.avgApprovalTime')">
            <VChart :option="avgTimeOption" style="height: 250px" autoresize />
          </NCard>
        </NGi>
      </NGrid>
    </template>
  </div>
</template>
