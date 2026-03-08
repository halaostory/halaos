<script setup lang="ts">
import { ref, h, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NSpace, NTag, NDatePicker, NSelect, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { reportsAPI, employeeAPI } from '../api/client'
import { format } from 'date-fns'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()

interface DTRRecord {
  id: number
  employee_id: number
  employee_no: string
  first_name: string
  last_name: string
  department_name: string
  position_name: string
  clock_in_at: string
  clock_out_at: string | null
  work_hours: number | string
  overtime_hours: number | string
  late_minutes: number
  undertime_minutes: number
  status: string
  clock_in_source: string
  clock_out_source: string
}

const records = ref<DTRRecord[]>([])
const loading = ref(false)
const dateRange = ref<[number, number] | null>(null)
const selectedEmployee = ref<number | null>(null)
const employees = ref<{ label: string; value: number }[]>([])

function fmtTime(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'HH:mm') } catch { return String(d) }
}

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

function fmtDay(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'EEE') } catch { return '' }
}

const statusMap: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
  present: 'success', late: 'warning', undertime: 'error', absent: 'error',
}

const columns = computed<DataTableColumns<DTRRecord>>(() => {
  const cols: DataTableColumns<DTRRecord> = []

  // Show employee info columns when viewing all employees
  if (!selectedEmployee.value) {
    cols.push(
      { title: t('employee.employeeNo'), key: 'employee_no', width: 100 },
      { title: t('employee.name'), key: 'name', width: 160, render: (r) => `${r.first_name} ${r.last_name}` },
      { title: t('dtr.department'), key: 'department_name', width: 120 },
    )
  }

  cols.push(
    { title: t('dtr.date'), key: 'date', width: 110, render: (r) => fmtDate(r.clock_in_at) },
    { title: t('dtr.day'), key: 'day', width: 60, render: (r) => fmtDay(r.clock_in_at) },
    { title: t('dtr.clockIn'), key: 'clock_in_at', width: 80, render: (r) => fmtTime(r.clock_in_at) },
    { title: t('dtr.clockOut'), key: 'clock_out_at', width: 80, render: (r) => fmtTime(r.clock_out_at) },
    { title: t('dtr.workHours'), key: 'work_hours', width: 90, render: (r) => Number(r.work_hours || 0).toFixed(2) },
    { title: t('dtr.otHours'), key: 'overtime_hours', width: 80, render: (r) => Number(r.overtime_hours || 0).toFixed(2) },
    {
      title: t('dtr.late'), key: 'late_minutes', width: 70,
      render: (r) => r.late_minutes > 0 ? h(NTag, { type: 'warning', size: 'small' }, () => `${r.late_minutes}m`) : '-'
    },
    {
      title: t('dtr.undertime'), key: 'undertime_minutes', width: 80,
      render: (r) => r.undertime_minutes > 0 ? h(NTag, { type: 'error', size: 'small' }, () => `${r.undertime_minutes}m`) : '-'
    },
    {
      title: t('common.status'), key: 'status', width: 90,
      render: (r) => h(NTag, { type: statusMap[r.status] || 'default', size: 'small' }, () => r.status)
    },
  )
  return cols
})

// Summary stats
const summary = computed(() => {
  const data = records.value
  return {
    totalDays: data.length,
    totalHours: data.reduce((s, r) => s + Number(r.work_hours || 0), 0).toFixed(2),
    totalOT: data.reduce((s, r) => s + Number(r.overtime_hours || 0), 0).toFixed(2),
    totalLate: data.reduce((s, r) => s + (r.late_minutes || 0), 0),
    totalUndertime: data.reduce((s, r) => s + (r.undertime_minutes || 0), 0),
    lateCount: data.filter(r => r.status === 'late').length,
  }
})

async function loadEmployees() {
  try {
    const res = await employeeAPI.list({ page: '1', limit: '500' }) as any
    const emps = res?.data?.data || res?.data || []
    employees.value = [
      { label: t('dtr.allEmployees'), value: 0 },
      ...emps.map((e: any) => ({ label: `${e.first_name} ${e.last_name} (${e.employee_no})`, value: e.id })),
    ]
  } catch (e) {
    console.error('Failed to load employees', e)
  }
}

async function loadDTR() {
  if (!dateRange.value) {
    message.warning(t('dtr.selectDateRange'))
    return
  }
  loading.value = true
  try {
    const start = format(new Date(dateRange.value[0]), 'yyyy-MM-dd')
    const end = format(new Date(dateRange.value[1]), 'yyyy-MM-dd')
    const params: Record<string, string> = { start, end }
    if (selectedEmployee.value && selectedEmployee.value > 0) {
      params.employee_id = String(selectedEmployee.value)
    }
    const res = await reportsAPI.dtr(params) as any
    records.value = res?.data || []
  } catch {
    message.error(t('common.failed'))
  } finally {
    loading.value = false
  }
}

function exportCSV() {
  if (!dateRange.value) return
  const start = format(new Date(dateRange.value[0]), 'yyyy-MM-dd')
  const end = format(new Date(dateRange.value[1]), 'yyyy-MM-dd')
  const token = auth.token
  const url = reportsAPI.dtrCsvUrl(start, end)
  const link = document.createElement('a')
  link.href = url
  link.setAttribute('download', `dtr_${start}_${end}.csv`)
  // For authenticated download, use fetch
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then(r => r.blob())
    .then(blob => {
      const blobUrl = URL.createObjectURL(blob)
      link.href = blobUrl
      link.click()
      URL.revokeObjectURL(blobUrl)
    })
}

// Set default date range to current month
const now = new Date()
const firstDay = new Date(now.getFullYear(), now.getMonth(), 1)
const lastDay = new Date(now.getFullYear(), now.getMonth() + 1, 0)
dateRange.value = [firstDay.getTime(), lastDay.getTime()]

loadEmployees()
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('dtr.title') }}</h2>
      <NSpace>
        <NButton type="info" size="small" :disabled="!dateRange" @click="exportCSV">
          {{ t('dtr.exportCSV') }}
        </NButton>
      </NSpace>
    </NSpace>

    <NSpace style="margin-bottom: 16px;" align="center">
      <NDatePicker v-model:value="dateRange" type="daterange" clearable style="width: 280px;" />
      <NSelect
        v-model:value="selectedEmployee"
        :options="employees"
        filterable
        clearable
        style="width: 280px;"
        :placeholder="t('dtr.selectEmployee')"
      />
      <NButton type="primary" :loading="loading" @click="loadDTR">{{ t('dtr.generate') }}</NButton>
    </NSpace>

    <!-- Summary -->
    <div v-if="records.length > 0" style="display: flex; gap: 16px; margin-bottom: 16px; flex-wrap: wrap;">
      <NTag type="info" size="large">{{ t('dtr.totalDays') }}: {{ summary.totalDays }}</NTag>
      <NTag type="success" size="large">{{ t('dtr.totalHours') }}: {{ summary.totalHours }}</NTag>
      <NTag type="info" size="large">{{ t('dtr.totalOT') }}: {{ summary.totalOT }}</NTag>
      <NTag v-if="summary.totalLate > 0" type="warning" size="large">{{ t('dtr.totalLate') }}: {{ summary.totalLate }}m ({{ summary.lateCount }}x)</NTag>
      <NTag v-if="summary.totalUndertime > 0" type="error" size="large">{{ t('dtr.totalUndertime') }}: {{ summary.totalUndertime }}m</NTag>
    </div>

    <NDataTable :columns="columns" :data="records" :loading="loading" :row-key="(r: DTRRecord) => r.id" :max-height="600" virtual-scroll />
  </div>
</template>
