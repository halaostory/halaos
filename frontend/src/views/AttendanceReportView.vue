<script setup lang="ts">
import { ref, h, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NDataTable, NButton, NSpace, NDatePicker, NEmpty, NStatistic, NGrid, NGi,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { attendanceAPI } from '../api/client'

const { t } = useI18n()
const message = useMessage()

interface ReportRow {
  employee_id: number
  employee_no: string
  first_name: string
  last_name: string
  department_name: string
  days_worked: number
  total_work_hours: string
  total_overtime_hours: string
  total_late_minutes: number
  total_undertime_minutes: number
  late_count: number
  undertime_count: number
}

const report = ref<ReportRow[]>([])
const loading = ref(false)
const dateRange = ref<[number, number] | null>(null)

const totals = computed(() => {
  const rows = report.value
  return {
    employees: rows.length,
    days: rows.reduce((sum, r) => sum + r.days_worked, 0),
    hours: rows.reduce((sum, r) => sum + parseFloat(r.total_work_hours || '0'), 0).toFixed(1),
    overtime: rows.reduce((sum, r) => sum + parseFloat(r.total_overtime_hours || '0'), 0).toFixed(1),
    late: rows.reduce((sum, r) => sum + r.total_late_minutes, 0),
    undertime: rows.reduce((sum, r) => sum + r.total_undertime_minutes, 0),
  }
})

const columns = computed<DataTableColumns<ReportRow>>(() => [
  { title: t('attendanceReport.employeeNo'), key: 'employee_no', width: 100 },
  {
    title: t('attendanceReport.employeeName'), key: 'name', width: 160,
    render: (row) => h('span', {}, `${row.last_name}, ${row.first_name}`),
  },
  { title: t('attendanceReport.department'), key: 'department_name', width: 140 },
  { title: t('attendanceReport.daysWorked'), key: 'days_worked', width: 90, align: 'center' },
  {
    title: t('attendanceReport.totalHours'), key: 'total_work_hours', width: 100, align: 'right',
    render: (row) => h('span', {}, parseFloat(row.total_work_hours || '0').toFixed(1)),
  },
  {
    title: t('attendanceReport.overtimeHours'), key: 'total_overtime_hours', width: 90, align: 'right',
    render: (row) => h('span', {}, parseFloat(row.total_overtime_hours || '0').toFixed(1)),
  },
  { title: t('attendanceReport.lateMinutes'), key: 'total_late_minutes', width: 90, align: 'right' },
  { title: t('attendanceReport.undertimeMinutes'), key: 'total_undertime_minutes', width: 90, align: 'right' },
  { title: t('attendanceReport.lateCount'), key: 'late_count', width: 80, align: 'center' },
  { title: t('attendanceReport.undertimeCount'), key: 'undertime_count', width: 80, align: 'center' },
])

async function generateReport() {
  if (!dateRange.value) {
    message.warning('Please select a date range')
    return
  }
  loading.value = true
  try {
    const start = new Date(dateRange.value[0]).toISOString().split('T')[0]
    const end = new Date(dateRange.value[1]).toISOString().split('T')[0]
    const res = await attendanceAPI.getReport(start, end) as { data?: ReportRow[] }
    const data = (res as any)?.data ?? res
    report.value = Array.isArray(data) ? data : []
  } catch {
    message.error('Failed to generate report')
  } finally {
    loading.value = false
  }
}

function handlePrint() {
  window.print()
}
</script>

<template>
  <div>
    <div class="no-print" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
      <h2>{{ t('attendanceReport.title') }}</h2>
      <NSpace>
        <NDatePicker
          v-model:value="dateRange"
          type="daterange"
          :placeholder="t('attendanceReport.dateRange')"
          clearable
        />
        <NButton type="primary" :loading="loading" @click="generateReport">
          {{ t('attendanceReport.generate') }}
        </NButton>
        <NButton v-if="report.length > 0" @click="handlePrint">
          {{ t('attendanceReport.print') }}
        </NButton>
      </NSpace>
    </div>

    <NEmpty v-if="report.length === 0 && !loading" :description="t('attendanceReport.noData')" />

    <template v-if="report.length > 0">
      <!-- Summary Stats -->
      <NGrid :cols="6" :x-gap="12" :y-gap="12" responsive="screen" style="margin-bottom: 20px;" class="no-print">
        <NGi>
          <NCard size="small">
            <NStatistic label="Employees" :value="totals.employees" />
          </NCard>
        </NGi>
        <NGi>
          <NCard size="small">
            <NStatistic label="Total Days" :value="totals.days" />
          </NCard>
        </NGi>
        <NGi>
          <NCard size="small">
            <NStatistic label="Total Hours" :value="totals.hours" />
          </NCard>
        </NGi>
        <NGi>
          <NCard size="small">
            <NStatistic label="OT Hours" :value="totals.overtime" />
          </NCard>
        </NGi>
        <NGi>
          <NCard size="small">
            <NStatistic label="Late (min)" :value="totals.late" />
          </NCard>
        </NGi>
        <NGi>
          <NCard size="small">
            <NStatistic label="Undertime (min)" :value="totals.undertime" />
          </NCard>
        </NGi>
      </NGrid>

      <!-- Print header -->
      <div class="print-only" style="text-align: center; margin-bottom: 16px;">
        <h2>Attendance Summary Report</h2>
        <p v-if="dateRange">{{ new Date(dateRange[0]).toLocaleDateString() }} - {{ new Date(dateRange[1]).toLocaleDateString() }}</p>
      </div>

      <NCard>
        <NDataTable
          :columns="columns"
          :data="report"
          :bordered="true"
          :loading="loading"
          :row-key="(row: ReportRow) => row.employee_id"
          :pagination="false"
          size="small"
        />
      </NCard>
    </template>
  </div>
</template>

<style scoped>
.print-only {
  display: none;
}

@media print {
  .no-print {
    display: none !important;
  }
  .print-only {
    display: block !important;
  }
}
</style>
