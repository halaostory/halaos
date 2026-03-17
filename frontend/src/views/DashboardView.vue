<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NGrid, NGi, NCard, NStatistic, NButton, NSpace, NTag, NBadge, useMessage } from 'naive-ui'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, PieChart, LineChart } from 'echarts/charts'
import {
  TitleComponent, TooltipComponent, LegendComponent,
  GridComponent, DatasetComponent,
} from 'echarts/components'
import { attendanceAPI, dashboardAPI, analyticsAPI, suggestionsAPI, announcementAPI, employeeAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import DashboardBriefing from '../components/DashboardBriefing.vue'
import FlightRiskWidget from '../components/FlightRiskWidget.vue'
import TeamHealthWidget from '../components/TeamHealthWidget.vue'
import BurnoutRiskWidget from '../components/BurnoutRiskWidget.vue'
import ComplianceAlertsWidget from '../components/ComplianceAlertsWidget.vue'

use([
  CanvasRenderer, BarChart, PieChart, LineChart,
  TitleComponent, TooltipComponent, LegendComponent,
  GridComponent, DatasetComponent,
])

const router = useRouter()
const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()
const themeStore = useThemeStore()

const clockedIn = ref(false)
const clockLoading = ref(false)
const totalEmployees = ref(0)
const presentToday = ref(0)
const pendingLeaves = ref(0)
const pendingOT = ref(0)

interface Suggestion {
  type: string
  priority: string
  title: string
  description: string
  count: number
  items?: unknown[]
}

interface AnnouncementItem {
  id: number
  title: string
  content: string
  priority: string
  published_at: string | null
  author_first_name: string
  author_last_name: string
}

const announcements = ref<AnnouncementItem[]>([])
const suggestions = ref<Suggestion[]>([])

const deptData = ref<{ name: string; count: number }[]>([])
const payrollData = ref<{ name: string; gross: number; deductions: number; net: number }[]>([])
const leaveData = ref<{ name: string; count: number }[]>([])
const birthdays = ref<{ id: number; name: string; date: string }[]>([])
const anniversaries = ref<{ id: number; name: string; date: string; years: number }[]>([])
const headcountData = ref<{ month: string; active_count: number; separated_count: number }[]>([])
const turnoverData = ref<{ month: string; separations: number; active_count: number }[]>([])
const expiringDocs = ref<{ id: string; employee_no: string; first_name: string; last_name: string; doc_type: string; file_name: string; expiry_date: string }[]>([])

interface ActionItem {
  category: string
  label: string
  count: number
  route: string
}
const actionItems = ref<ActionItem[]>([])

const chartTextColor = computed(() => themeStore.isDark ? '#ccc' : '#333')

const deptChartOption = computed(() => ({
  tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
  legend: { bottom: 0, textStyle: { color: chartTextColor.value } },
  series: [{
    type: 'pie',
    radius: ['40%', '70%'],
    avoidLabelOverlap: false,
    itemStyle: { borderRadius: 6, borderColor: themeStore.isDark ? '#1e1e2e' : '#fff', borderWidth: 2 },
    label: { show: false },
    emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold' } },
    data: deptData.value.map((d) => ({ value: d.count, name: d.name })),
  }],
}))

const payrollChartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  legend: { bottom: 0, textStyle: { color: chartTextColor.value } },
  grid: { left: '3%', right: '4%', bottom: '15%', containLabel: true },
  xAxis: {
    type: 'category',
    data: payrollData.value.map((d) => d.name),
    axisLabel: { color: chartTextColor.value, rotate: 30, fontSize: 10 },
  },
  yAxis: {
    type: 'value',
    axisLabel: { color: chartTextColor.value, formatter: (v: number) => v >= 1000 ? `${(v / 1000).toFixed(0)}K` : v },
  },
  series: [
    { name: t('payroll.grossPay'), type: 'bar', stack: 'total', data: payrollData.value.map((d) => d.gross), itemStyle: { color: '#36d399' } },
    { name: t('payroll.deductions'), type: 'bar', stack: 'total', data: payrollData.value.map((d) => d.deductions), itemStyle: { color: '#f87272' } },
    { name: t('payroll.netPay'), type: 'line', data: payrollData.value.map((d) => d.net), itemStyle: { color: '#3abff8' } },
  ],
}))

const leaveChartOption = computed(() => ({
  tooltip: { trigger: 'item', formatter: '{b}: {c}' },
  legend: { bottom: 0, textStyle: { color: chartTextColor.value } },
  series: [{
    type: 'pie',
    radius: '65%',
    data: leaveData.value.map((d) => ({ value: d.count, name: d.name })),
    label: { color: chartTextColor.value },
  }],
}))

const headcountChartOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  legend: { bottom: 0, textStyle: { color: chartTextColor.value } },
  grid: { left: '3%', right: '4%', bottom: '15%', containLabel: true },
  xAxis: {
    type: 'category',
    data: headcountData.value.map(d => d.month),
    axisLabel: { color: chartTextColor.value, rotate: 30, fontSize: 10 },
  },
  yAxis: { type: 'value', axisLabel: { color: chartTextColor.value } },
  series: [
    { name: t('dashboard.activeEmployees'), type: 'line', data: headcountData.value.map(d => d.active_count), itemStyle: { color: '#36d399' }, areaStyle: { opacity: 0.15 } },
    { name: t('dashboard.separated'), type: 'bar', data: headcountData.value.map(d => d.separated_count), itemStyle: { color: '#f87272' } },
  ],
}))

const turnoverChartOption = computed(() => {
  const data = turnoverData.value.map(d => ({
    month: d.month,
    rate: d.active_count > 0 ? ((d.separations / d.active_count) * 100).toFixed(1) : '0.0',
    separations: d.separations,
  }))
  return {
    tooltip: { trigger: 'axis', formatter: (params: any) => {
      const p = params[0]
      return `${p.name}<br/>${t('dashboard.turnoverRate')}: ${p.value}%<br/>${t('dashboard.separations')}: ${data[p.dataIndex]?.separations ?? 0}`
    }},
    grid: { left: '3%', right: '4%', bottom: '10%', containLabel: true },
    xAxis: {
      type: 'category',
      data: data.map(d => d.month),
      axisLabel: { color: chartTextColor.value, rotate: 30, fontSize: 10 },
    },
    yAxis: { type: 'value', axisLabel: { color: chartTextColor.value, formatter: '{value}%' } },
    series: [{
      name: t('dashboard.turnoverRate'), type: 'line',
      data: data.map(d => parseFloat(d.rate)),
      itemStyle: { color: '#f59e0b' },
      areaStyle: { opacity: 0.1 },
    }],
  }
})

onMounted(async () => {
  try {
    const [stats, att] = await Promise.allSettled([
      dashboardAPI.getStats(),
      attendanceAPI.getSummary(),
    ])
    if (stats.status === 'fulfilled') {
      const res = stats.value as { success: boolean; data: { total_employees: number; present_today: number; pending_leaves: number; pending_overtime: number } }
      if (res.data) {
        totalEmployees.value = res.data.total_employees || 0
        presentToday.value = res.data.present_today || 0
        pendingLeaves.value = res.data.pending_leaves || 0
        pendingOT.value = res.data.pending_overtime || 0
      }
    }
    if (att.status === 'fulfilled') {
      const res = att.value as { data?: Record<string, unknown> }
      const data = res.data || res as unknown as Record<string, unknown>
      if (data.clock_in_at && !data.clock_out_at) {
        clockedIn.value = true
      }
    }
  } catch (e) {
    console.error('Failed to load dashboard data', e)
  }

  // Load announcements
  try {
    const aRes = await announcementAPI.list()
    const aData = (aRes as any)?.data ?? aRes
    announcements.value = Array.isArray(aData) ? aData.slice(0, 3) : []
  } catch (e) { console.error('Failed to load announcements', e) }

  // Load suggestions
  if (auth.isAdmin || auth.isManager) {
    try {
      const sRes = await suggestionsAPI.list()
      const sData = (sRes as any)?.data ?? sRes
      suggestions.value = Array.isArray(sData) ? sData : []
    } catch (e) { console.error('Failed to load suggestions', e) }
  }

  // Load action items & expiring documents (admin/manager)
  if (auth.isAdmin || auth.isManager) {
    dashboardAPI.getActionItems().then((res) => {
      const d = (res as any)?.data ?? res
      actionItems.value = Array.isArray(d) ? d : []
    }).catch((e) => { console.error('Failed to load action items', e) })

    employeeAPI.listExpiringDocuments().then((res) => {
      const d = (res as any)?.data ?? res
      expiringDocs.value = Array.isArray(d) ? d : []
    }).catch((e) => { console.error('Failed to load expiring documents', e) })
  }

  // Load celebrations
  dashboardAPI.getCelebrations().then((res) => {
    const d = (res as any)?.data ?? res
    birthdays.value = d?.birthdays || []
    anniversaries.value = d?.anniversaries || []
  }).catch((e) => { console.error('Failed to load celebrations', e) })

  // Load chart data in parallel
  const chartPromises = [
    dashboardAPI.getDepartmentDistribution().then((res) => {
      const d = (res as { data?: unknown[] }).data || res
      deptData.value = (d as { name: string; count: number }[]) || []
    }).catch((e) => { console.error('Failed to load department distribution', e) }),
    dashboardAPI.getLeaveSummary().then((res) => {
      const d = (res as { data?: unknown[] }).data || res
      leaveData.value = (d as { name: string; count: number }[]) || []
    }).catch((e) => { console.error('Failed to load leave summary', e) }),
  ]
  if (auth.isAdmin) {
    chartPromises.push(
      dashboardAPI.getPayrollTrend().then((res) => {
        const d = (res as { data?: unknown[] }).data || res
        payrollData.value = (d as { name: string; gross: number; deductions: number; net: number }[]) || []
      }).catch((e) => { console.error('Failed to load payroll trend', e) }),
      analyticsAPI.getHeadcountTrend().then((res) => {
        const d = (res as any)?.data ?? res
        headcountData.value = Array.isArray(d) ? d : []
      }).catch((e) => { console.error('Failed to load headcount trend', e) }),
      analyticsAPI.getTurnover().then((res) => {
        const d = (res as any)?.data ?? res
        turnoverData.value = Array.isArray(d) ? d : []
      }).catch((e) => { console.error('Failed to load turnover data', e) }),
    )
  }
  await Promise.allSettled(chartPromises)
})

async function refreshClockState() {
  try {
    const res = await attendanceAPI.getSummary() as { data?: Record<string, unknown> }
    const data = res.data || res as unknown as Record<string, unknown>
    clockedIn.value = !!(data.clock_in_at && !data.clock_out_at)
  } catch (e) {
    console.error('Failed to refresh clock state', e)
  }
}

function getDashboardLocation(): Promise<{ lat: string; lng: string } | null> {
  if (!navigator.geolocation) return Promise.resolve(null)
  return new Promise((resolve) => {
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lat: pos.coords.latitude.toFixed(7), lng: pos.coords.longitude.toFixed(7) }),
      () => resolve(null),
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 60000 }
    )
  })
}

async function handleClockIn() {
  clockLoading.value = true
  try {
    const loc = await getDashboardLocation()
    await attendanceAPI.clockIn({ source: 'web', lat: loc?.lat, lng: loc?.lng })
    message.success(t('dashboard.clockInSuccess'))
    await refreshClockState()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('dashboard.clockInFailed'))
  } finally {
    clockLoading.value = false
  }
}

async function handleClockOut() {
  clockLoading.value = true
  try {
    const loc = await getDashboardLocation()
    await attendanceAPI.clockOut({ source: 'web', lat: loc?.lat, lng: loc?.lng })
    message.success(t('dashboard.clockOutSuccess'))
    await refreshClockState()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('dashboard.clockOutFailed'))
  } finally {
    clockLoading.value = false
  }
}
</script>

<template>
  <div>
    <h2 style="margin-bottom: 24px;">{{ t('dashboard.title') }}</h2>

    <div id="dashboard-briefing">
      <DashboardBriefing />
    </div>

    <NGrid id="dashboard-stats" :cols="4" :x-gap="16" :y-gap="16" responsive="screen" style="margin-bottom: 24px;">
      <NGi>
        <NCard>
          <NStatistic :label="t('dashboard.totalEmployees')" :value="totalEmployees" />
        </NCard>
      </NGi>
      <NGi>
        <NCard>
          <NStatistic :label="t('dashboard.presentToday')" :value="presentToday" />
        </NCard>
      </NGi>
      <NGi>
        <NCard>
          <NStatistic :label="t('dashboard.onLeave')" :value="pendingLeaves" />
        </NCard>
      </NGi>
      <NGi>
        <NCard>
          <NStatistic :label="t('dashboard.pendingApprovals')" :value="pendingOT" />
        </NCard>
      </NGi>
    </NGrid>

    <NCard id="dashboard-clock" :title="t('dashboard.quickActions')" style="margin-bottom: 24px;">
      <NSpace>
        <NButton v-if="!clockedIn" type="primary" :loading="clockLoading" @click="handleClockIn">
          {{ t('dashboard.clockIn') }}
        </NButton>
        <template v-else>
          <NTag type="success">{{ t('dashboard.clockedIn') }}</NTag>
          <NButton type="warning" :loading="clockLoading" @click="handleClockOut">
            {{ t('dashboard.clockOut') }}
          </NButton>
        </template>
      </NSpace>
    </NCard>

    <!-- Pending Actions -->
    <NCard v-if="actionItems.length > 0" :title="t('dashboard.pendingActions')" style="margin-bottom: 24px;">
      <div style="display: flex; flex-wrap: wrap; gap: 12px;">
        <div v-for="item in actionItems" :key="item.label"
          style="display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5); cursor: pointer; min-width: 200px;"
          @click="router.push(item.route)">
          <NBadge :value="item.count" :max="99" :type="item.category === 'approvals' ? 'warning' : item.category === 'payroll' ? 'info' : 'error'" />
          <span style="font-size: 13px;">{{ item.label }}</span>
        </div>
      </div>
    </NCard>

    <!-- Announcements -->
    <NCard v-if="announcements.length > 0" :title="t('announcement.title')" style="margin-bottom: 24px;">
      <div style="display: flex; flex-direction: column; gap: 10px;">
        <div v-for="ann in announcements" :key="ann.id" style="padding: 10px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5);">
          <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px;">
            <NTag :type="ann.priority === 'urgent' ? 'error' : ann.priority === 'important' ? 'warning' : 'default'" size="small">
              {{ ann.priority }}
            </NTag>
            <span style="font-weight: 600;">{{ ann.title }}</span>
          </div>
          <div style="font-size: 13px; color: #666; white-space: pre-wrap; max-height: 60px; overflow: hidden;">{{ ann.content }}</div>
        </div>
      </div>
    </NCard>

    <!-- Celebrations -->
    <NGrid v-if="birthdays.length > 0 || anniversaries.length > 0" :cols="2" :x-gap="16" :y-gap="16" responsive="screen" style="margin-bottom: 24px;">
      <NGi v-if="birthdays.length > 0">
        <NCard :title="t('dashboard.upcomingBirthdays')">
          <div style="display: flex; flex-direction: column; gap: 8px;">
            <div v-for="b in birthdays" :key="b.id" style="display: flex; align-items: center; gap: 10px; padding: 6px 0;">
              <span style="font-size: 20px;">&#127874;</span>
              <div style="flex: 1;">
                <div style="font-weight: 600;">{{ b.name }}</div>
                <div style="font-size: 12px; color: #999;">{{ b.date }}</div>
              </div>
            </div>
          </div>
        </NCard>
      </NGi>
      <NGi v-if="anniversaries.length > 0">
        <NCard :title="t('dashboard.workAnniversaries')">
          <div style="display: flex; flex-direction: column; gap: 8px;">
            <div v-for="a in anniversaries" :key="a.id" style="display: flex; align-items: center; gap: 10px; padding: 6px 0;">
              <span style="font-size: 20px;">&#127942;</span>
              <div style="flex: 1;">
                <div style="font-weight: 600;">{{ a.name }}</div>
                <div style="font-size: 12px; color: #999;">{{ a.years }} {{ a.years === 1 ? 'year' : 'years' }} - {{ a.date }}</div>
              </div>
            </div>
          </div>
        </NCard>
      </NGi>
    </NGrid>

    <!-- Expiring Documents -->
    <NCard v-if="expiringDocs.length > 0" :title="t('dashboard.expiringDocuments')" style="margin-bottom: 24px;">
      <div style="display: flex; flex-direction: column; gap: 8px;">
        <div v-for="doc in expiringDocs" :key="doc.id" style="display: flex; align-items: center; gap: 10px; padding: 6px 0;">
          <NTag :type="new Date(doc.expiry_date) < new Date() ? 'error' : 'warning'" size="small">
            {{ new Date(doc.expiry_date).toLocaleDateString() }}
          </NTag>
          <div style="flex: 1;">
            <div style="font-weight: 600;">{{ doc.first_name }} {{ doc.last_name }} ({{ doc.employee_no }})</div>
            <div style="font-size: 12px; color: #999;">{{ doc.doc_type }} - {{ doc.file_name }}</div>
          </div>
        </div>
      </div>
    </NCard>

    <!-- AI Smart Suggestions -->
    <NCard v-if="suggestions.length > 0" :title="t('dashboard.smartSuggestions')" style="margin-bottom: 24px;">
      <div style="display: flex; flex-direction: column; gap: 10px;">
        <div v-for="s in suggestions" :key="s.type" style="display: flex; align-items: flex-start; gap: 10px; padding: 10px; border-radius: 8px; background: var(--n-color-hover, #f5f5f5);">
          <NTag :type="s.priority === 'high' ? 'error' : s.priority === 'medium' ? 'warning' : 'info'" size="small" style="flex-shrink: 0;">
            {{ s.priority }}
          </NTag>
          <div style="flex: 1;">
            <div style="font-weight: 600; font-size: 14px;">{{ s.title }}</div>
            <div style="font-size: 12px; color: #999; margin-top: 2px;">{{ s.description }}</div>
          </div>
        </div>
      </div>
    </NCard>

    <!-- Flight Risk Dashboard -->
    <FlightRiskWidget v-if="auth.isAdmin || auth.isManager" />

    <!-- Team Health Dashboard -->
    <TeamHealthWidget v-if="auth.isAdmin || auth.isManager" />

    <!-- Burnout Risk Dashboard -->
    <BurnoutRiskWidget v-if="auth.isAdmin || auth.isManager" />

    <!-- Compliance Alerts Dashboard -->
    <ComplianceAlertsWidget v-if="auth.isAdmin || auth.isManager" />

    <NGrid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" style="margin-bottom: 24px;">
      <NGi v-if="deptData.length > 0">
        <NCard :title="t('dashboard.deptDistribution')">
          <VChart :option="deptChartOption" style="height: 300px;" autoresize />
        </NCard>
      </NGi>
      <NGi v-if="leaveData.length > 0">
        <NCard :title="t('dashboard.leaveOverview')">
          <VChart :option="leaveChartOption" style="height: 300px;" autoresize />
        </NCard>
      </NGi>
    </NGrid>

    <NCard v-if="auth.isAdmin && payrollData.length > 0" :title="t('dashboard.payrollTrend')" style="margin-bottom: 24px;">
      <VChart :option="payrollChartOption" style="height: 350px;" autoresize />
    </NCard>

    <NGrid v-if="auth.isAdmin" :cols="2" :x-gap="16" :y-gap="16" responsive="screen" style="margin-bottom: 24px;">
      <NGi v-if="headcountData.length > 0">
        <NCard :title="t('dashboard.headcountTrend')">
          <VChart :option="headcountChartOption" style="height: 300px;" autoresize />
        </NCard>
      </NGi>
      <NGi v-if="turnoverData.length > 0">
        <NCard :title="t('dashboard.turnoverRate')">
          <VChart :option="turnoverChartOption" style="height: 300px;" autoresize />
        </NCard>
      </NGi>
    </NGrid>
  </div>
</template>
