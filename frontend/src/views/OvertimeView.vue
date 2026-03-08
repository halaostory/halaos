<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput, NDatePicker,
  NInputNumber, NSelect, NSpace, NTag, NTimePicker, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { overtimeAPI, formPrefillAPI } from '../api/client'
import { format } from 'date-fns'

const { t } = useI18n()
const message = useMessage()
const data = ref<Record<string, unknown>[]>([])
const loading = ref(false)
const showModal = ref(false)
const submitting = ref(false)

const form = ref({
  ot_date: null as number | null,
  start_at: null as number | null,
  end_at: null as number | null,
  hours: 1,
  ot_type: 'regular',
  reason: '',
})

function resetForm() {
  form.value = { ot_date: null, start_at: null, end_at: null, hours: 1, ot_type: 'regular', reason: '' }
}

function timeStringToTimestamp(timeStr: string): number {
  const [h, m] = timeStr.split(':').map(Number)
  const d = new Date()
  d.setHours(h, m, 0, 0)
  return d.getTime()
}

async function openOTModal() {
  resetForm()
  showModal.value = true
  try {
    const res = await formPrefillAPI.get('overtime')
    const d = (res as any)?.data ?? res
    if (d) {
      if (d.ot_type) form.value.ot_type = d.ot_type
      if (d.suggested_hours) form.value.hours = d.suggested_hours
      if (d.common_start_time) form.value.start_at = timeStringToTimestamp(d.common_start_time)
      if (d.common_end_time) form.value.end_at = timeStringToTimestamp(d.common_end_time)
    }
  } catch { /* prefill is best-effort */ }
}

function fmtDate(d: unknown): string {
  if (!d) return '-'
  try { return format(new Date(d as string), 'yyyy-MM-dd') } catch { return String(d) }
}

function fmtTime(ts: number | null): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

const otTypes = [
  { label: t('overtime.regular'), value: 'regular' },
  { label: t('overtime.restDay'), value: 'rest_day' },
  { label: t('overtime.holiday'), value: 'holiday' },
]

const statusMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  pending: 'warning', approved: 'success', rejected: 'error',
}

const columns: DataTableColumns = [
  { title: t('overtime.date'), key: 'ot_date', width: 120, render: (row) => fmtDate(row.ot_date) },
  { title: t('overtime.hours'), key: 'hours', width: 80, render: (row) => row.hours ? Number(row.hours).toFixed(1) : '-' },
  {
    title: t('overtime.type'), key: 'ot_type', width: 120,
    render: (row) => {
      const typeLabels: Record<string, string> = { regular: t('overtime.regular'), rest_day: t('overtime.restDay'), holiday: t('overtime.holiday') }
      return typeLabels[row.ot_type as string] || String(row.ot_type)
    }
  },
  {
    title: t('common.status'), key: 'status', width: 110,
    render: (row) => h(NTag, { type: statusMap[row.status as string] || 'default', size: 'small' }, () => t(`common.${row.status}`) || String(row.status))
  },
  { title: t('leave.reason'), key: 'reason' },
]

async function loadData() {
  loading.value = true
  try {
    const res = await overtimeAPI.listRequests({ page: '1', limit: '50' }) as { data: Record<string, unknown>[] }
    data.value = res.data || (Array.isArray(res) ? res : []) as Record<string, unknown>[]
  } catch {
    data.value = []
  } finally {
    loading.value = false
  }
}

onMounted(loadData)

async function submitRequest() {
  if (!form.value.ot_date) {
    message.warning(t('overtime.selectDate'))
    return
  }
  submitting.value = true
  try {
    const otDate = new Date(form.value.ot_date).toISOString().split('T')[0]
    const startTime = fmtTime(form.value.start_at) || '18:00'
    const endTime = fmtTime(form.value.end_at) || '20:00'
    await overtimeAPI.createRequest({
      ot_date: otDate,
      start_at: `${otDate}T${startTime}:00Z`,
      end_at: `${otDate}T${endTime}:00Z`,
      hours: String(form.value.hours),
      ot_type: form.value.ot_type,
      reason: form.value.reason || undefined,
    })
    message.success(t('overtime.submitted'))
    showModal.value = false
    resetForm()
    loadData()
  } catch (e: unknown) {
    const err = e as { data?: { error?: { message?: string } } }
    message.error(err.data?.error?.message || t('common.saveFailed'))
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div>
    <NSpace justify="space-between" style="margin-bottom: 16px;">
      <h2>{{ t('overtime.title') }}</h2>
      <NButton type="primary" @click="openOTModal">{{ t('overtime.apply') }}</NButton>
    </NSpace>
    <NDataTable :columns="columns" :data="data" :loading="loading" />

    <NModal v-model:show="showModal" :title="t('overtime.apply')" preset="card" style="max-width: 480px; width: 95vw;" @after-leave="resetForm">
      <NForm @submit.prevent="submitRequest">
        <NFormItem :label="t('overtime.date')" required>
          <NDatePicker v-model:value="form.ot_date" type="date" style="width: 100%;" />
        </NFormItem>
        <NSpace :size="12" style="width: 100%;">
          <NFormItem :label="t('overtime.startTime')" style="flex: 1;">
            <NTimePicker v-model:value="form.start_at" format="HH:mm" style="width: 100%;" />
          </NFormItem>
          <NFormItem :label="t('overtime.endTime')" style="flex: 1;">
            <NTimePicker v-model:value="form.end_at" format="HH:mm" style="width: 100%;" />
          </NFormItem>
        </NSpace>
        <NFormItem :label="t('overtime.hours')">
          <NInputNumber v-model:value="form.hours" :min="0.5" :step="0.5" />
        </NFormItem>
        <NFormItem :label="t('overtime.type')">
          <NSelect v-model:value="form.ot_type" :options="otTypes" />
        </NFormItem>
        <NFormItem :label="t('leave.reason')">
          <NInput v-model:value="form.reason" type="textarea" />
        </NFormItem>
        <NSpace>
          <NButton type="primary" :loading="submitting" attr-type="submit">{{ t('common.submit') }}</NButton>
          <NButton @click="showModal = false">{{ t('common.cancel') }}</NButton>
        </NSpace>
      </NForm>
    </NModal>
  </div>
</template>
