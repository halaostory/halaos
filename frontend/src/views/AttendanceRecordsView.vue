<script setup lang="ts">
import { ref, computed, h, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NTag, NPagination, NSpace, NDatePicker, NSelect,
  NButton, NCard, useMessage, type DataTableColumns,
} from 'naive-ui'
import { attendanceAPI, employeeAPI, exportAPI } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { format, subDays } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const auth = useAuthStore()
const data = ref<Record<string, unknown>[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// Filters
const dateRange = ref<[number, number]>([
  subDays(new Date(), 30).getTime(),
  new Date().getTime(),
])
const selectedEmployee = ref<number | null>(null)
const employeeOptions = ref<{ label: string; value: number }[]>([])
const employeeMap = ref(new Map<number, string>())

const isManager = auth.isAdmin || auth.isManager

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}
function fmtTime(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'HH:mm') } catch { return String(d) }
}

const statusMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  present: 'success', late: 'warning', undertime: 'warning', absent: 'error', open: 'info',
}

const columns = computed<DataTableColumns>(() => {
  const cols: DataTableColumns = []
  if (isManager) {
    cols.push({
      title: t('employee.name'), key: 'employee_name', width: 150,
      render: (row) => employeeMap.value.get(row.employee_id as number) || String(row.employee_id || '-'),
    })
  }
  cols.push(
    { title: t('attendance.date'), key: 'date', width: 120, render: (row) => fmtDate(row.clock_in_at) },
    { title: t('attendance.timeIn'), key: 'time_in', width: 100, render: (row) => fmtTime(row.clock_in_at) },
    { title: t('attendance.timeOut'), key: 'time_out', width: 100, render: (row) => fmtTime(row.clock_out_at) },
    { title: t('attendance.workHours'), key: 'work_hours', width: 100, render: (row) => row.work_hours ? Number(row.work_hours).toFixed(1) : '-' },
    { title: t('attendance.otHours'), key: 'overtime_hours', width: 100, render: (row) => row.overtime_hours ? Number(row.overtime_hours).toFixed(1) : '-' },
    { title: t('attendance.lateMin'), key: 'late_minutes', width: 100, render: (row) => String(row.late_minutes || '-') },
    {
      title: t('common.status'), key: 'status', width: 100,
      render: (row) => h(NTag, { type: statusMap[row.status as string] || 'default', size: 'small' }, () => String(row.status)),
    },
  )
  return cols
})

async function fetchRecords() {
  loading.value = true
  try {
    const params: Record<string, string> = {
      page: String(page.value),
      limit: String(pageSize.value),
    }
    if (dateRange.value) {
      params.from = format(new Date(dateRange.value[0]), 'yyyy-MM-dd')
      params.to = format(new Date(dateRange.value[1]).getTime() + 86400000, 'yyyy-MM-dd')
    }
    if (selectedEmployee.value) {
      params.employee_id = String(selectedEmployee.value)
    }
    const res = await attendanceAPI.listRecords(params) as { data: Record<string, unknown>[]; meta?: { total: number } }
    data.value = res.data || []
    total.value = res.meta?.total ?? data.value.length
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
}

async function loadEmployees() {
  if (!isManager) return
  try {
    const res = await employeeAPI.list({ page: '1', limit: '500' })
    const resData = (res as any)?.data ?? res
    const list = Array.isArray(resData) ? resData : (resData?.data ?? [])
    const newMap = new Map<number, string>()
    employeeOptions.value = list.map((e: any) => {
      const name = `${e.first_name} ${e.last_name}`
      newMap.set(e.id, name)
      return { label: `${name} (${e.employee_no})`, value: e.id }
    })
    employeeMap.value = newMap
  } catch { /* ignore */ }
}

function handleSearch() {
  page.value = 1
  fetchRecords()
}

function handleExportCSV() {
  const token = localStorage.getItem('token')
  const start = dateRange.value ? format(new Date(dateRange.value[0]), 'yyyy-MM-dd') : format(subDays(new Date(), 30), 'yyyy-MM-dd')
  const end = dateRange.value ? format(new Date(dateRange.value[1]), 'yyyy-MM-dd') : format(new Date(), 'yyyy-MM-dd')
  const url = exportAPI.attendanceCSV(start, end)
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then((res) => res.blob())
    .then((blob) => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = `attendance_${start}_${end}.csv`
      a.click()
      URL.revokeObjectURL(a.href)
    })
    .catch(() => message.error(t('common.failed')))
}

watch(page, fetchRecords)

onMounted(() => {
  fetchRecords()
  loadEmployees()
})
</script>

<template>
  <div>
    <h2 style="margin-bottom: 16px;">{{ t('attendance.records') }}</h2>

    <NCard size="small" style="margin-bottom: 16px;">
      <NSpace align="center" :wrap="true">
        <NDatePicker v-model:value="dateRange" type="daterange" clearable style="width: 300px;" />
        <NSelect
          v-if="isManager"
          v-model:value="selectedEmployee"
          :options="employeeOptions"
          :placeholder="t('common.allEmployees')"
          clearable
          filterable
          style="width: 250px;"
        />
        <NButton type="primary" @click="handleSearch">{{ t('common.search') }}</NButton>
        <NButton v-if="isManager" @click="handleExportCSV">CSV</NButton>
      </NSpace>
    </NCard>

    <NDataTable :columns="columns" :data="data" :loading="loading" :bordered="false" />
    <div style="display: flex; justify-content: flex-end; margin-top: 16px;" v-if="total > pageSize">
      <NPagination v-model:page="page" :page-size="pageSize" :item-count="total" />
    </div>
  </div>
</template>
