<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NSpace, NModal, NForm, NFormItem,
  NInput, NSelect, NTag, NDatePicker, NProgress,
  useMessage, type DataTableColumns,
} from 'naive-ui'
import { clearanceAPI, employeeAPI } from '../api/client'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()

interface ClearanceRow {
  id: number
  employee_id: number
  employee_name: string
  employee_no: string
  department_name: string
  resignation_date: string
  last_working_day: string
  reason: string | null
  status: string
  created_at: string
}

interface ClearanceItem {
  id: number
  clearance_id: number
  department: string
  item_name: string
  status: string
  cleared_by_email: string
  cleared_at: string | null
  remarks: string | null
}

const loading = ref(false)
const clearances = ref<ClearanceRow[]>([])

// New request form
const showNewModal = ref(false)
const newLoading = ref(false)
const newForm = ref({
  employee_id: null as number | null,
  resignation_date: null as number | null,
  last_working_day: null as number | null,
  reason: '',
})

// Detail modal
const showDetailModal = ref(false)
const detailLoading = ref(false)
const detailRequest = ref<Record<string, unknown> | null>(null)
const detailItems = ref<ClearanceItem[]>([])

// Employees for select
const employees = ref<{ label: string; value: number }[]>([])

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

function statusType(s: string): 'default' | 'warning' | 'success' | 'error' | 'info' {
  const map: Record<string, 'default' | 'warning' | 'success' | 'error' | 'info'> = {
    pending: 'warning', in_progress: 'info', completed: 'success', cancelled: 'error',
  }
  return map[s] || 'default'
}

function statusLabel(s: string): string {
  const map: Record<string, string> = {
    pending: t('clearance.pending'),
    in_progress: t('clearance.inProgress'),
    completed: t('clearance.completed'),
    cancelled: t('clearance.cancelled'),
  }
  return map[s] || s
}

function itemStatusType(s: string): 'default' | 'success' | 'info' {
  const map: Record<string, 'default' | 'success' | 'info'> = {
    pending: 'default', cleared: 'success', not_applicable: 'info',
  }
  return map[s] || 'default'
}

const columns = computed<DataTableColumns>(() => [
  { title: t('clearance.employee'), key: 'employee_name', width: 160,
    render: (r: any) => `${r.employee_name} (${r.employee_no})` },
  { title: t('clearance.department'), key: 'department_name', width: 130 },
  { title: t('clearance.resignationDate'), key: 'resignation_date', width: 120,
    render: (r: any) => fmtDate(r.resignation_date) },
  { title: t('clearance.lastWorkingDay'), key: 'last_working_day', width: 120,
    render: (r: any) => fmtDate(r.last_working_day) },
  {
    title: t('clearance.status'), key: 'status', width: 110,
    render: (r: any) => h(NTag, { type: statusType(r.status), size: 'small' },
      () => statusLabel(r.status)),
  },
  { title: t('common.date'), key: 'created_at', width: 110, render: (r: any) => fmtDate(r.created_at) },
  {
    title: t('common.actions'), key: 'actions', width: 120, fixed: 'right',
    render: (r: any) => h(NButton, {
      size: 'small', type: 'primary', quaternary: true,
      onClick: () => openDetail(r.id),
    }, () => t('clearance.viewDetails')),
  },
])

const itemColumns = computed<DataTableColumns>(() => [
  { title: t('clearance.department'), key: 'department', width: 130 },
  { title: t('clearance.item'), key: 'item_name', width: 200 },
  {
    title: t('clearance.itemStatus'), key: 'status', width: 100,
    render: (r: any) => h(NTag, { type: itemStatusType(r.status), size: 'small' },
      () => r.status === 'cleared' ? t('clearance.cleared')
           : r.status === 'not_applicable' ? t('clearance.notApplicable')
           : t('clearance.pending')),
  },
  { title: t('clearance.remarks'), key: 'remarks', width: 150, ellipsis: { tooltip: true } },
  {
    title: t('common.actions'), key: 'actions', width: 160,
    render: (r: any) => {
      if (r.status !== 'pending') return ''
      return h(NSpace, { size: 4 }, () => [
        h(NButton, { size: 'tiny', type: 'success',
          onClick: () => updateItem(r.id, 'cleared') },
          () => t('clearance.clearItem')),
        h(NButton, { size: 'tiny', type: 'default',
          onClick: () => updateItem(r.id, 'not_applicable') },
          () => t('clearance.markNA')),
      ])
    },
  },
])

const clearanceProgress = computed(() => {
  if (detailItems.value.length === 0) return 0
  const done = detailItems.value.filter(i => i.status !== 'pending').length
  return Math.round((done / detailItems.value.length) * 100)
})

async function fetchClearances() {
  loading.value = true
  try {
    const res = await clearanceAPI.list()
    const data = (res as any)?.data ?? res
    clearances.value = Array.isArray(data) ? data : []
  } catch {
    clearances.value = []
  } finally {
    loading.value = false
  }
}

async function fetchEmployees() {
  try {
    const res = await employeeAPI.list({ status: 'active', limit: '500', offset: '0' })
    const data = (res as any)?.data ?? res
    employees.value = (Array.isArray(data) ? data : []).map((e: any) => ({
      label: `${e.first_name} ${e.last_name} (${e.employee_no})`,
      value: e.id,
    }))
  } catch (e) { console.error('Failed to load employees', e); message.error(t('common.loadFailed')) }
}

async function submitNewRequest() {
  if (!newForm.value.employee_id || !newForm.value.resignation_date || !newForm.value.last_working_day) return
  newLoading.value = true
  try {
    await clearanceAPI.create({
      employee_id: newForm.value.employee_id,
      resignation_date: format(new Date(newForm.value.resignation_date), 'yyyy-MM-dd'),
      last_working_day: format(new Date(newForm.value.last_working_day), 'yyyy-MM-dd'),
      reason: newForm.value.reason || undefined,
    })
    message.success(t('clearance.created'))
    showNewModal.value = false
    newForm.value = { employee_id: null, resignation_date: null, last_working_day: null, reason: '' }
    await fetchClearances()
  } catch {
    message.error(t('common.failed'))
  } finally {
    newLoading.value = false
  }
}

async function openDetail(id: number) {
  detailLoading.value = true
  showDetailModal.value = true
  try {
    const res = await clearanceAPI.get(id)
    const data = (res as any)?.data ?? res
    detailRequest.value = data.request
    detailItems.value = Array.isArray(data.items) ? data.items : []
  } catch {
    message.error(t('common.failed'))
  } finally {
    detailLoading.value = false
  }
}

async function updateItem(itemId: number, status: string) {
  try {
    await clearanceAPI.updateItem(itemId, { status })
    message.success(t('clearance.updated'))
    if (detailRequest.value) {
      await openDetail((detailRequest.value as any).id)
    }
  } catch {
    message.error(t('common.failed'))
  }
}

async function markComplete() {
  if (!detailRequest.value) return
  try {
    await clearanceAPI.updateStatus((detailRequest.value as any).id, 'completed')
    message.success(t('clearance.updated'))
    showDetailModal.value = false
    await fetchClearances()
  } catch {
    message.error(t('common.failed'))
  }
}

onMounted(() => {
  fetchClearances()
  fetchEmployees()
})
</script>

<template>
  <div>
    <NSpace justify="space-between" align="center" style="margin-bottom: 16px;">
      <h2 style="margin: 0;">{{ t('clearance.title') }}</h2>
      <NButton type="primary" @click="showNewModal = true">
        {{ t('clearance.newRequest') }}
      </NButton>
    </NSpace>

    <NDataTable
      :columns="columns"
      :data="clearances"
      :loading="loading"
      size="small"
      :scroll-x="900"
      :max-height="500"
    />

    <!-- New Clearance Request Modal -->
    <NModal
      v-model:show="showNewModal"
      :title="t('clearance.newRequest')"
      preset="card"
      style="width: 520px;"
    >
      <NForm label-placement="left" label-width="150">
        <NFormItem :label="t('clearance.employee')" required>
          <NSelect
            v-model:value="newForm.employee_id"
            :options="employees"
            filterable
            :placeholder="t('clearance.employee')"
          />
        </NFormItem>
        <NFormItem :label="t('clearance.resignationDate')" required>
          <NDatePicker v-model:value="newForm.resignation_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('clearance.lastWorkingDay')" required>
          <NDatePicker v-model:value="newForm.last_working_day" type="date" style="width: 100%;" />
        </NFormItem>
        <NFormItem :label="t('clearance.reason')">
          <NInput v-model:value="newForm.reason" type="textarea" :rows="2" />
        </NFormItem>
        <NSpace justify="end">
          <NButton @click="showNewModal = false">{{ t('common.cancel') }}</NButton>
          <NButton
            type="primary"
            :loading="newLoading"
            :disabled="!newForm.employee_id || !newForm.resignation_date || !newForm.last_working_day"
            @click="submitNewRequest"
          >
            {{ t('common.submit') }}
          </NButton>
        </NSpace>
      </NForm>
    </NModal>

    <!-- Detail Modal -->
    <NModal
      v-model:show="showDetailModal"
      :title="t('clearance.viewDetails')"
      preset="card"
      style="width: 800px;"
    >
      <template v-if="detailRequest">
        <NSpace vertical :size="12" style="margin-bottom: 16px;">
          <div style="display: flex; gap: 24px; flex-wrap: wrap;">
            <div><strong>{{ t('clearance.employee') }}:</strong> {{ (detailRequest as any).employee_name }}</div>
            <div><strong>{{ t('clearance.resignationDate') }}:</strong> {{ fmtDate((detailRequest as any).resignation_date) }}</div>
            <div><strong>{{ t('clearance.lastWorkingDay') }}:</strong> {{ fmtDate((detailRequest as any).last_working_day) }}</div>
          </div>
          <div v-if="(detailRequest as any).reason">
            <strong>{{ t('clearance.reason') }}:</strong> {{ (detailRequest as any).reason }}
          </div>
          <div style="display: flex; align-items: center; gap: 12px;">
            <strong>{{ t('clearance.progress') }}:</strong>
            <NProgress
              type="line"
              :percentage="clearanceProgress"
              :indicator-placement="'inside'"
              style="flex: 1; max-width: 300px;"
            />
          </div>
        </NSpace>

        <NDataTable
          :columns="itemColumns"
          :data="detailItems"
          :loading="detailLoading"
          size="small"
          :max-height="350"
        />

        <NSpace justify="end" style="margin-top: 16px;" v-if="(detailRequest as any).status !== 'completed'">
          <NButton
            type="success"
            :disabled="clearanceProgress < 100"
            @click="markComplete"
          >
            {{ t('clearance.complete') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>
